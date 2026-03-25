import { useBackgroundLocation } from "@/hooks/use-location";
import {
  Camera,
  FillLayer,
  LineLayer,
  MapView,
  ShapeSource,
  UserLocation,
} from "@maplibre/maplibre-react-native";
import type GeoJSON from "geojson";
import { useEffect, useRef, useState } from "react";
import { ActivityIndicator, Text } from "react-native";

type Point = { lat: number; lng: number };

type Player = {
  id: string;
  name: string;
  team: string;
  city: string;
  lastPoint: [number, number] | null;
  trail: [number, number][][][] | null; // array of groups, each group has lines
  claimed: [number, number][][][] | null; // same shape
};

type GameState = {
  colors: string[];
  players: Player[];
};

type ExtractedData = {
  lines: [number, number][][]; // array of lines, each line is array of [lng, lat]
  loops: [number, number][][]; // array of loops, each loop is array of [lng, lat]
};

const toCoord = (p: Point): [number, number] => [p.lng, p.lat];

const extractLinesAndLoops = (data: GameState): ExtractedData => {
  const lines: [number, number][][] = [];
  const loops: [number, number][][] = [];

  for (const player of data.players) {
    if (player.trail) {
      for (const group of player.trail) {
        // group = [line, line, ...]
        for (const line of group) {
          // line = [[lng, lat], ...]
          lines.push(line);
        }
      }
    }

    if (player.claimed) {
      for (const group of player.claimed) {
        // group = [loop, loop, ...]
        for (const loop of group) {
          // loop = [[lng, lat], ...]
          loops.push(loop);
        }
      }
    }
  }

  return { lines, loops };
};

export default function HomeScreen() {
  //const SERVER_URL = "http://10.100.1.50:8080"; // RC
  //const SERVER_URL = "http://192.168.1.28:8080"; // Home
  const SERVER_URL = "http://10.100.19.188:9090"; // Cyrene
  //const SERVER_URL = "https://geogo.rcdis.co/";

  const LOBBY_ID = "test12";
  const PLAYER_ID = "ebf22b7d-1b2f-433f-8402-118f8d8dbf56";

  const userId = "alice"; // TODO: Determine based on device

  const { location, status, error } = useBackgroundLocation();

  const mapRef: any = useRef(null);

  const [coords, setCoords] = useState<[number?, number?]>([]);

  // points for lines
  const [points, setPoints] = useState<[number, number][]>([]);

  // claimed shapes
  const [claimedShapes, setClaimedShapes] = useState<[number, number][][]>([]);

  //const [mapState, setMapState] = useState({});

  // Call this every ~10s with the new GPS coordinate
  // For use live updating on client
  const addPoint = (longitude: number, latitude: number) => {
    setPoints((prev) => [...prev, [longitude, latitude]]);
  };

  // Build a GeoJSON LineString from the collected lines
  const linesGeoJSON: GeoJSON.Feature<GeoJSON.LineString> = {
    type: "Feature", // now inferred as literal 'Feature', not string
    properties: {},
    geometry: {
      type: "LineString", // same here
      coordinates: points,
    },
  };

  // TODO: Array of shapesGeoJSON
  // Build a GeoJSON LineString from the collected claimed shapes
  const allShapesGeoJSON: GeoJSON.Feature<GeoJSON.LineString>[] = [];
  for (const shape of claimedShapes) {
    allShapesGeoJSON.push({
      type: "Feature", // now inferred as literal 'Feature', not string
      properties: {},
      geometry: {
        type: "LineString", // same here
        coordinates: shape,
      },
    });
  }

  async function fetchState() {
    try {
      const response = await fetch(SERVER_URL + "/state?lobby=" + LOBBY_ID);
      const json = await response.json();
      console.log("game state");
      console.log(JSON.stringify(json));

      // Render lines and shapes - could be a separate function
      // for each lines, extract from .points [lng, lat]
      const linesAndLoops = extractLinesAndLoops(json);
      console.log(JSON.stringify("--- lines and loops ---"));
      console.log(JSON.stringify(linesAndLoops));

      const allLines: [number, number][] = [];
      for (const line of linesAndLoops.lines) {
        allLines.push(...line);
      }
      setPoints(allLines);

      const allShapes: [number, number][][] = [];
      for (const loop of linesAndLoops.loops) {
        allShapes.push(loop);
      }
      setClaimedShapes(allShapes);

      console.log("lines:");
      console.log(JSON.stringify(allLines));
      console.log("shapes:");
      console.log(JSON.stringify(allShapes));
    } catch (error) {
      console.error("Error fetching data - fetchState:", error);
    }
  }

  async function pingLocation(lat: number, lng: number) {
    try {
      const data = { lobby: LOBBY_ID, player: PLAYER_ID, points: [[lng, lat]] };

      const response = await fetch(SERVER_URL + "/ping", {
        method: "POST", // Specify the method
        headers: {
          "Content-Type": "application/json", // Inform the server about the body format
        },
        body: JSON.stringify(data),
      });
      const json = await response.text();

      console.log("--- ping location response ---");
      console.log(json);
    } catch (error) {
      console.error("Error fetching data - pingLocation:", error);
    }
  }

  useEffect(() => {
    console.log("-- update location " + new Date().toLocaleTimeString() + "--");
    if (location) {
      setCoords([location.coords.longitude, location.coords.latitude]);

      // Uncomment this to render a point locally/on the client, without the server
      // addPoint(location.coords.longitude, location.coords.latitude);

      // Send to server - longitude and latitude
      // TODO: Waiting for JSON response from pingLocation
      pingLocation(location.coords.latitude, location.coords.longitude);
      fetchState();
    }
  }, [location]);

  if (status === "starting") return <ActivityIndicator style={{ flex: 1 }} />;
  if (status === "error") return <Text>{error}</Text>;

  return (
    <MapView
      ref={mapRef}
      style={{ flex: 1 }}
      mapStyle="https://api.maptiler.com/maps/streets-v2/style.json?key=ZkJOL4BGmS6lWcFXLlfG"
    >
      <Camera
        centerCoordinate={[coords[0] ?? 0, coords[1] ?? 0]}
        zoomLevel={17}
        followUserLocation
      />
      <UserLocation />
      {points.length >= 2 && (
        <ShapeSource id="route" shape={linesGeoJSON}>
          <LineLayer
            id="routeLine"
            style={{
              lineColor: "#DA3E15", // TODO: Change based on user team. #20B8EA is blue
              lineWidth: 4,
              lineOpacity: 0.8,
              lineCap: "round",
              lineJoin: "round",
            }}
          />
        </ShapeSource>
      )}
      {allShapesGeoJSON.length >= 1 &&
        allShapesGeoJSON.map((shape, index) => (
          <ShapeSource id="route" key={index} shape={shape}>
            <FillLayer
              id="routeShape"
              style={{
                fillColor: "#DA3E15",
                fillOpacity: 0.4,
              }}
            />
          </ShapeSource>
        ))}
    </MapView>
  );
}
