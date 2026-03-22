import { useEffect, useRef, useState } from "react";
import * as Location from "expo-location";
import * as TaskManager from "expo-task-manager";

const TASK_NAME = "bg-location";

type Status = "idle" | "starting" | "tracking" | "error";

let _onUpdate: ((loc: Location.LocationObject) => void) | null = null;

TaskManager.defineTask(TASK_NAME, async ({ data, error }) => {
  if (error || !data) return;
  const { locations } = data as { locations: Location.LocationObject[] };
  if (locations?.[0]) _onUpdate?.(locations[0]);
});

export function useBackgroundLocation(
  opts: Location.LocationTaskOptions = {
    accuracy: Location.Accuracy.High,
    distanceInterval: 10,
    foregroundService: {
      notificationTitle: "Tracking location",
      notificationBody: "Running in background",
    },
  },
) {
  const [location, setLocation] = useState<Location.LocationObject | null>(
    null,
  );
  const [status, setStatus] = useState<Status>("idle");
  const [error, setError] = useState<string | null>(null);
  const optsRef = useRef(opts);

  useEffect(() => {
    let cancelled = false;

    _onUpdate = (loc) => {
      if (!cancelled) setLocation(loc);
    };

    (async () => {
      setStatus("starting");

      const fg = await Location.requestForegroundPermissionsAsync();
      if (fg.status !== "granted") {
        setError("Foreground permission denied");
        setStatus("error");
        return;
      }

      const bg = await Location.requestBackgroundPermissionsAsync();
      if (bg.status !== "granted") {
        setError("Background permission denied");
        setStatus("error");
        return;
      }

      if (cancelled) return;

      const alreadyRunning =
        await Location.hasStartedLocationUpdatesAsync(TASK_NAME);
      if (!alreadyRunning) {
        await Location.startLocationUpdatesAsync(TASK_NAME, optsRef.current);
      }

      if (!cancelled) setStatus("tracking");
    })();

    return () => {
      cancelled = true;
      _onUpdate = null;
      Location.stopLocationUpdatesAsync(TASK_NAME).catch(() => {});
    };
  }, []);

  return { location, status, error };
}
