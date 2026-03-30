import { useCallback } from "react";

const SERVER_URL = "http://10.100.19.188:9090";

export function useSendPoint(lobbyId: string, playerTag: string) {
  return useCallback(async (point: [number, number]) => {
    await fetch(SERVER_URL + "/ping", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        lobby: lobbyId,
        player: playerTag,
        points: [point],
      }),
    });
  }, [lobbyId, playerTag]);
}
