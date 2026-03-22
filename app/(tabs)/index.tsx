import { useBackgroundLocation } from "@/hooks/use-location";
import { MapView } from "@maplibre/maplibre-react-native";
import { ActivityIndicator, Text } from "react-native";

export default function HomeScreen() {
  const { location, status, error } = useBackgroundLocation();

  if (status === "starting") return <ActivityIndicator style={{ flex: 1 }} />;
  if (status === "error") return <Text>{error}</Text>;

  const { latitude, longitude } = location
    ? location.coords
    : { latitude: 40.7128, longitude: 74.006 };

  return (
    <MapView
      style={{
        flex: 1,
      }}
    />
  );
}
