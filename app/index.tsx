import ErrorScreen from "@/components/ErrorScreen";
import { GameState, useGame } from "@/hooks/use-game";
import { useSendPoint } from "@/hooks/use-send-point";
import {
  Camera,
  CircleLayer,
  FillLayer,
  LineLayer,
  MapView,
  ShapeSource,
} from "@maplibre/maplibre-react-native";
import React, { useState } from "react";

export default function Main() {
  const [lobbyId, setLobbyId] = useState("test12");
  const [playerTag, setPlayerTag] = useState("claire");

  const { state, error } = useGame(lobbyId, playerTag);
  const sendPoint = useSendPoint(lobbyId, playerTag);

  if (error) return <ErrorScreen message={error} />;

  return <Map state={state} playerTag={playerTag} sendPoint={sendPoint} />;
}


function Map({ state, playerTag, sendPoint }: { state: GameState | null; playerTag: string; sendPoint: (point: [number, number]) => Promise<void> }) {
  const lastPoint = state?.players.find((p) => p.tag === playerTag)?.lastPoint ?? null;

  console.log(JSON.stringify(state?.players[0].trail))

  return (
    <MapView
      style={{ flex: 1 }}
      mapStyle={"https://tiles.openfreemap.org/styles/positron"}
      onLongPress={e => {
        sendPoint(e.geometry.coordinates);
      }}
    >
      <Camera centerCoordinate={lastPoint ?? undefined} />
      {lastPoint && <MePoint lastPoint={lastPoint} />}
      {state?.players.map((player) => {
        const color = state.colors[state.colors.indexOf(player.team)] ?? "#DA3E15";
        return (
          <React.Fragment key={player.tag}>
            {player.trail && (
              <ShapeSource id={`trail-${player.tag}`} shape={{
                type: "Feature", properties: {},
                geometry: { type: "MultiPolygon", coordinates: player.trail },
              }}>
                <FillLayer id={`trailFill-${player.tag}`} style={{ fillColor: color, fillOpacity: 0.1 }} />
                <LineLayer id={`trailLine-${player.tag}`} style={{ lineColor: color, lineWidth: 2, lineOpacity: 0.6 }} />
              </ShapeSource>
            )}
            {player.claimed && (
              <ShapeSource id={`claimed-${player.tag}`} shape={{
                type: "Feature", properties: {},
                geometry: { type: "MultiPolygon", coordinates: player.claimed },
              }}>
                <FillLayer id={`claimedFill-${player.tag}`} style={{ fillColor: color, fillOpacity: 0.5 }} />
                {/*<LineLayer id={`claimedLine-${player.tag}`} style={{ lineColor: color, lineWidth: 2, lineOpacity: 0.8 }} />*/}
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
