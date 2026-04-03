import { SERVER } from "@/components/state";
import * as Location from "expo-location";
import * as TaskManager from "expo-task-manager";
import { useEffect, useState } from "react";

const URL = `http://${SERVER}`
const MIN_LOCATION_UPDATE = 80;

let _onLocations: ((locations: Location.LocationObject[]) => void) | null = null;

TaskManager.defineTask('task-location', async ({ data, error }) => {
  if (error) return;
  const { locations } = data as { locations: Location.LocationObject[] };
  if (locations?.length) _onLocations?.(locations);
});

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

export function useLocation(lobbyId: string, playerTag: string, active: boolean) {
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!active) return;

    _onLocations = async (locations: Location.LocationObject[]) => {
      try {
        // const res = await fetch(URL + "/ping", {
        //   method: "POST",
        //   headers: { "Content-Type": "application/json" },
        //   body: JSON.stringify({
        //     lobby: lobbyId,
        //     player: playerTag,
        //     points: locations.map((l) => ({
        //       lat: l.coords.latitude,
        //       lng: l.coords.longitude,
        //       rad: l.coords.accuracy,
        //       ts: l.timestamp
        //     })),
        //   }),
        // });
        // if (!res.ok) setError(`Send ping did not ok: ${res.status}`);
      } catch (e) {
        setError(e instanceof Error ? `Failed to send ping: ${e.message}` : "Failed to send ping");
      }
    };
    startLocationTask().catch((e) => setError(`Location task failed: ${e.message}`));

    return () => {
      _onLocations = null;
      Location.stopLocationUpdatesAsync('task-location').catch(() => { });
    };
  }, [active, lobbyId, playerTag]);

  return { error };
}
