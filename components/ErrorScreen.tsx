import { StyleSheet, Text, View } from "react-native";

export default function ErrorScreen({ message }: { message: string }) {
  return (
    <View style={styles.error}>
      <Text style={styles.errorTitle}>Something went wrong</Text>
      <Text style={styles.errorMessage}>{message}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  error: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    padding: 24,
    backgroundColor: "#1a1a1a",
  },
  errorTitle: {
    fontSize: 20,
    fontWeight: "bold",
    color: "#ff6b6b",
    marginBottom: 8,
  },
  errorMessage: {
    fontSize: 16,
    color: "#ccc",
    textAlign: "center",
  },
});
