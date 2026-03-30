import * as Location from "expo-location";
import * as TaskManager from "expo-task-manager";
import { useCallback, useEffect, useState } from "react";

const SERVER_URL = "http://10.100.19.188:9090";
const MIN_LOCATION_UPDATE = 80;
const REFETCH_INTERVAL = 10;

type Player = {
  tag: string;
  team: string;
  city: string;
  lastPoint: [number, number] | null;
  trail: [number, number][][][] | null;
  claimed: [number, number][][][] | null;
};

export type GameState = {
  colors: string[];
  players: Player[];
};

let _onLocations: ((locations: Location.LocationObject[]) => void) | null = null;

TaskManager.defineTask('task-location', async ({ data, error }) => {
  if (error) return;
  const { locations } = data as { locations: Location.LocationObject[] };
  if (locations?.length) _onLocations?.(locations);
});

export function useGame(lobbyId: string, playerTag: string) {
  const [state, setState] = useState<GameState | null>(null);
  const [error, setError] = useState<string | null>(null);
  const active = state !== null;

  const refetch = useCallback(async () => {
    try {
      const res = await fetch(SERVER_URL + "/state?lobby=" + lobbyId);
      if (!res.ok) {
        setError(`State fetch did not ok: ${res.status}`);
        return;
      }
      setState(await res.json());
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? `Failed to fetch state: ${e.message}` : "Failed to fetch state");
    }
  }, [lobbyId]);

  const onLocations = useCallback(async (locations: Location.LocationObject[]) => {
    try {
      // const res = await fetch(SERVER_URL + "/ping", {
      //   method: "POST",
      //   headers: { "Content-Type": "application/json" },
      //   body: JSON.stringify({
      //     lobby: lobbyId,
      //     player: playerTag,
      //     points: locations.map((l) => [l.coords.longitude, l.coords.latitude]),
      //   }),
      // });
      // if (!res.ok) setError(`Send ping did not ok: ${res.status}`);
      // else refetch();
    } catch (e) {
      setError(e instanceof Error ? `Failed to send ping: ${e.message}` : "Failed to send ping");
    }
  }, [lobbyId, playerTag, refetch]);

  // Poll game state
  useEffect(() => {
    refetch();
    const id = setInterval(refetch, REFETCH_INTERVAL * 1000);
    return () => clearInterval(id);
  }, [refetch]);


  // Start location tracking once state is fetched
  useEffect(() => {
    if (!active) return;

    _onLocations = onLocations;
    startLocationTask().catch((e) => setError(`Location task failed: ${e.message}`));

    return () => {
      _onLocations = null;
      Location.stopLocationUpdatesAsync('task-location').catch(() => { });
    };
  }, [active, onLocations]);

  return { state, error };
}

async function startLocationTask() {
  const fg = await Location.requestForegroundPermissionsAsync();
  if (fg.status !== "granted") throw new Error("Foreground permission denied");

  const bg = await Location.requestBackgroundPermissionsAsync();
  if (bg.status !== "granted") throw new Error("Background permission denied");

  const alreadyRunning = await Location.hasStartedLocationUpdatesAsync('task-location');
  if (!alreadyRunning) {
    await Location.startLocationUpdatesAsync('task-location', {
      accuracy: Location.Accuracy.Highest,
      distanceInterval: MIN_LOCATION_UPDATE,
      foregroundService: {
        notificationTitle: "go claim land!",
        notificationBody: "geogo is running in the background",
      },
    });
  }
}
