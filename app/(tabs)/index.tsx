import { useBackgroundLocation } from "@/hooks/use-location";
import { Camera, MapView } from "@maplibre/maplibre-react-native";
import { useEffect, useRef, useState } from "react";
import { ActivityIndicator, Text } from "react-native";

export default function HomeScreen() {
  const { location, status, error } = useBackgroundLocation();

  const mapRef: any = useRef(null);

  const [coords, setCoords] = useState<[number, number]>([-74.006, 40.7128]);

  useEffect(() => {
    if (location) {
      setCoords([location.coords.longitude, location.coords.latitude]);
    }
  }, [location]);

  if (status === "starting") return <ActivityIndicator style={{ flex: 1 }} />;
  if (status === "error") return <Text>{error}</Text>;

  return (
    <MapView ref={mapRef} style={{ flex: 1 }}>
      <Camera centerCoordinate={coords} zoomLevel={13} />
    </MapView>
  );
}
