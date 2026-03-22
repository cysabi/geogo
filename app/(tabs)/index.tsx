import { Image } from "expo-image";
import { View, StyleSheet } from "react-native";
import MapView from "react-native-maps";
import Location from "expo-location";
import { useState } from "react";

export default function HomeScreen() {
  const [location, setLocation] = useState<Location.LocationObject | null>(
    null,
  );

  return (
    <View style={styles.titleContainer}>
      <MapView style={styles.map} />
    </View>
  );
}

const styles = StyleSheet.create({
  map: {
    width: "100%",
    height: "100%",
  },
  titleContainer: {
    flex: 1,
  },
  stepContainer: {
    gap: 8,
    marginBottom: 8,
  },
  reactLogo: {
    height: 178,
    width: 290,
    bottom: 0,
    left: 0,
    position: "absolute",
  },
});
