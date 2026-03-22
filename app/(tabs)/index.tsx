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
  const { location, status, error } = useBackgroundLocation();

  const mapRef: any = useRef(null);

  const [coords, setCoords] = useState<[number, number]>([
    -73.985056, 40.691327,
  ]);

  const [points, setPoints] = useState<[number, number][]>([]);

  // Call this every ~10s with the new GPS coordinate
  const addPoint = (longitude: number, latitude: number) => {
    setPoints((prev) => [...prev, [longitude, latitude]]);
  };

  // Manually add points for now ...
  // TODO: Remove to replace with user location
  useEffect(() => {
    addPoint(-73.985056, 40.691327);
    addPoint(-73.985353, 40.690668);
    addPoint(-73.98636, 40.691084);
  }, []);

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
        centerCoordinate={coords}
        zoomLevel={17} /* followUserLocation */
      />
      <UserLocation />
      {points.length >= 2 && (
        <ShapeSource id="route" shape={routeGeoJSON}>
          <LineLayer
            id="routeLine"
            style={{
              lineColor: "#4A90E2",
              lineWidth: 4,
              lineOpacity: 0.8,
              lineCap: "round",
              lineJoin: "round",
            }}
          />
        </ShapeSource>
      )}
    </MapView>
  );
}
