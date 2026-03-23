import { useBackgroundLocation } from "@/hooks/use-location";
import {
  Camera,
  LineLayer,
  MapView,
  ShapeSource,
  UserLocation,
} from "@maplibre/maplibre-react-native";
import type GeoJSON from "geojson";
import { useEffect, useRef, useState } from "react";
import { ActivityIndicator, Text } from "react-native";

export default function HomeScreen() {
  const SERVER_URL = "http://10.100.1.50:8080";
  const userId = "alice"; // TODO: Determine based on device

  const { location, status, error } = useBackgroundLocation();

  const mapRef: any = useRef(null);

  const [coords, setCoords] = useState<[number?, number?]>([]);

  const [points, setPoints] = useState<[number, number][]>([]);

  const [mapState, setMapState] = useState({});

  // Call this every ~10s with the new GPS coordinate
  const addPoint = (longitude: number, latitude: number) => {
    setPoints((prev) => [...prev, [longitude, latitude]]);
  };

  // Build a GeoJSON LineString from the collected points
  const routeGeoJSON: GeoJSON.Feature<GeoJSON.LineString> = {
    type: "Feature", // now inferred as literal 'Feature', not string
    properties: {},
    geometry: {
      type: "LineString", // same here
      coordinates: points,
    },
  };

  async function fetchState() {
    try {
      const response = await fetch(SERVER_URL + "/state");
      const json = await response.json();
      console.log(json);
      setMapState(json);
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
      const json = await response.text();
    } catch (error) {
      console.error("Error fetching data:", error);
    }
  }

  useEffect(() => {
    console.log("-- update location " + new Date().toLocaleTimeString() + "--");
    if (location) {
      setCoords([location.coords.longitude, location.coords.latitude]);
      addPoint(location.coords.longitude, location.coords.latitude);

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
        <ShapeSource id="route" shape={routeGeoJSON}>
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
      <Text
        style={{
          position: "absolute",
          top: 50,
          left: 50,
        }}
      >
        {coords[0]}, {coords[1]}
      </Text>
    </MapView>
  );
}

/*
// TODO: Only fill in claimed shapes

<FillLayer
  id="routeShape"
  style={{
    fillColor: "#DA3E15",
    fillOpacity: 0.5,
  }}
/>

*/
