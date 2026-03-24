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
  team: string;
  lines: { points: Point[] }[] | null;
  claimed: { loops: Point[][] }[] | null;
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
    // Extract lines
    if (player.lines) {
      for (const line of player.lines) {
        lines.push(line.points.map(toCoord));
      }
    }

    // Extract loops
    if (player.claimed) {
      for (const claim of player.claimed) {
        for (const loop of claim.loops) {
          loops.push(loop.map(toCoord));
        }
      }
    }
  }

  return { lines: lines, loops: loops };
};

export default function HomeScreen() {
  //const SERVER_URL = "http://10.100.1.50:8080"; // RC
  const SERVER_URL = "http://192.168.1.28:8080"; // RC

  const userId = "alice"; // TODO: Determine based on device

  const { location, status, error } = useBackgroundLocation();

  const mapRef: any = useRef(null);

  const [coords, setCoords] = useState<[number?, number?]>([]);

  // points for lines
  const [points, setPoints] = useState<[number, number][]>([]);

  // claimed shapes
  const [claimedShapes, setClaimedShapes] = useState<[number, number][]>([]);

  const [mapState, setMapState] = useState({});

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

  // Build a GeoJSON LineString from the collected claimed shapes
  const shapesGeoJSON: GeoJSON.Feature<GeoJSON.LineString> = {
    type: "Feature", // now inferred as literal 'Feature', not string
    properties: {},
    geometry: {
      type: "LineString", // same here
      coordinates: claimedShapes,
    },
  };

  async function fetchState() {
    try {
      const response = await fetch(SERVER_URL + "/state");
      const json = await response.json();
      console.log(JSON.stringify(json));
      setMapState(json);

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

      const allShapes: [number, number][] = [];
      for (const loop of linesAndLoops.loops) {
        allShapes.push(...loop);
      }
      setClaimedShapes(allShapes);
    } catch (error) {
      console.error("Error fetching data:", error);
    }
  }

  async function pingLocation(lat: number, lng: number) {
    try {
      const url =
        SERVER_URL +
        "/ping?lat=" +
        lat +
        "&lng=" +
        lng +
        "&player_id=" +
        userId;

      const response = await fetch(url, {
        method: "POST", // Specify the method
        headers: {
          "Content-Type": "application/json", // Inform the server about the body format
        },
      });
      const json = await response.json();
      console.log(json);
    } catch (error) {
      console.error("Error fetching data:", error);
    }
  }

  useEffect(() => {
    console.log("-- update location " + new Date().toLocaleTimeString() + "--");
    if (location) {
      setCoords([location.coords.longitude, location.coords.latitude]);
      // addPoint(location.coords.longitude, location.coords.latitude);

      // Send to server - longitude and latitude
      // TODO: Waiting for JSON response from pingLocation
      pingLocation(location.coords.latitude, location.coords.longitude);
      fetchState();
    }
  }, [location]);

  console.log(mapState);

  if (status === "starting") return <ActivityIndicator style={{ flex: 1 }} />;
  if (status === "error") return <Text>{error}</Text>;

  console.log(coords[0], coords[1]);

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
      {claimedShapes.length >= 1 && (
        <ShapeSource id="route" shape={shapesGeoJSON}>
          <FillLayer
            id="routeShape"
            style={{
              fillColor: "#DA3E15",
              fillOpacity: 0.4,
            }}
          />
        </ShapeSource>
      )}
    </MapView>
  );
}
