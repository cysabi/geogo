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

export default function HomeScreen() {
  const { location, status, error } = useBackgroundLocation();

  const mapRef: any = useRef(null);

  const [coords, setCoords] = useState<[number?, number?]>([]);

  const [points, setPoints] = useState<[number, number][]>([]);

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

  useEffect(() => {
    if (location) {
      setCoords([location.coords.longitude, location.coords.latitude]);
      addPoint(location.coords.longitude, location.coords.latitude);
    }
    // TODO: send to server - longitude and latitude
  }, [location]);

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
          <FillLayer
            id="routeShape"
            style={{
              fillColor: "#DA3E15",
              fillOpacity: 0.0,
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
