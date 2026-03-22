import { useBackgroundLocation } from "@/hooks/use-location";
import { ActivityIndicator, Text, View } from "react-native";
import MapView, { Marker } from "react-native-maps";

export default function HomeScreen() {
  const { location, status, error } = useBackgroundLocation();

  if (status === "starting") return <ActivityIndicator style={{ flex: 1 }} />;
  if (status === "error") return <Text>{error}</Text>;

  const { latitude, longitude } = location
    ? location.coords
    : { latitude: 40.7128, longitude: 74.006 };

  return (
    <View>
      <Text>Hellooo</Text>
      <MapView
        style={{
          width: "100%",
          height: "100%",
        }}
        initialRegion={{
          latitude: 40.7128,
          longitude: 74.006,
          latitudeDelta: 0.01,
          longitudeDelta: 0.01,
        }}
      >
        <Marker
          coordinate={{ latitude, longitude }}
          title={`last updated ${location ? location.timestamp : 0}`}
        />
      </MapView>
    </View>
  );
}
