import ErrorScreen from "@/components/ErrorScreen";
import { useGame } from "@/hooks/use-game";
import { useLocation } from "@/hooks/use-location";
import { useSendPoint } from "@/hooks/use-send-point";
import {
  Camera,
  CircleLayer,
  FillLayer,
  LineLayer,
  MapView,
  ShapeSource,
  UserLocation,
} from "@maplibre/maplibre-react-native";
import React, { useState } from "react";
import ModalScreen from "./modal";
import type { GameState } from "../components/state";

export default function Main() {
  const [lobbyId, setLobbyId] = useState("test12");
  const [playerTag, setPlayerTag] = useState("claire");

  const { state, error: wsError } = useGame(lobbyId, playerTag);
  const { error: locError } = useLocation(lobbyId, playerTag, state !== null);
  const sendPoint = useSendPoint(lobbyId, playerTag);


  const error = wsError || locError;
  if (error) return <ErrorScreen message={error} />;

  // if (state === null) return <ModalScreen />

  return <Map state={state} playerTag={playerTag} sendPoint={sendPoint} />;
}

function Map({ state, playerTag, sendPoint }: { state: GameState | null; playerTag: string; sendPoint: (point: [number, number]) => Promise<void> }) {
  const lastPoint = state?.players.find((p) => p.tag === playerTag)?.lastPoint ?? null;

  return (
    <MapView
      style={{ flex: 1 }}
      mapStyle={"https://tiles.openfreemap.org/styles/positron"}
      onLongPress={e => {
        console.log("long press")
        sendPoint(e.geometry.coordinates);
      }}
    >
      <UserLocation />
      {lastPoint && <MePoint lastPoint={lastPoint} />}
      {state?.players.map((player) => {
        const color = state.colors[state.colors.indexOf(player.team)] ?? "#DA3E15";
        return (
          <React.Fragment key={player.tag}>
            {player.trail?.coordinates?.[0]?.[0]?.[0] && (
              <ShapeSource id={`trail-${player.tag}`} shape={{
                type: "Feature", properties: {},
                geometry: player.trail,
              }}>
                <LineLayer id={`trailLine-${player.tag}`} style={{ lineColor: color, lineWidth: 3, lineOpacity: 0.7 }} />
              </ShapeSource>
            )}
            {player.claimed?.coordinates?.[0]?.[0]?.[0] && (
              <ShapeSource id={`claimed-${player.tag}`} shape={{
                type: "Feature", properties: {},
                geometry: player.claimed,
              }}>
                <FillLayer id={`claimedFill-${player.tag}`} style={{ fillColor: color, fillOpacity: 0.5 }} />
              </ShapeSource>
            )}
          </React.Fragment>
        );
      })}
    </MapView>
  );
}

function MePoint({ lastPoint }: { lastPoint: [number, number] }) {
  return <ShapeSource id="playerDot" shape={{
    type: "Feature", properties: {},
    geometry: { type: "Point", coordinates: lastPoint },
  }}>
    <CircleLayer
      id="playerDotCircle"
      style={{
        circleRadius: 8,
        circleColor: "#4A90D9",
        circleStrokeColor: "#fff",
        circleStrokeWidth: 3,
      }}
    />
  </ShapeSource>
}
