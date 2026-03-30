import { Link } from 'expo-router';
import { View, Text, StyleSheet } from 'react-native';

// lobby creation popup
// if lobby code | player tag empty
// - ask type lobby code & player tag
// if state is still null, ask to generate new lobby
// if state isnt null but player tag doesn't exist, ask to make new player

export default function ModalScreen() {
  return (
    <View style={styles.container}>
      <Text style={styles.title}>Join Lobby</Text>
      {/* TODO: lobby join UI */}
      <Link href="/" dismissTo style={styles.link}>
        <Text style={styles.linkText}>Back</Text>
      </Link>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    padding: 20,
  },
  title: {
    fontSize: 24,
    fontWeight: 'bold',
    marginBottom: 20,
  },
  link: {
    marginTop: 15,
    paddingVertical: 15,
  },
  linkText: {
    color: '#007AFF',
    fontSize: 16,
  },
});
