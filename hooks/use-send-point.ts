import { SERVER } from "@/components/state";
import { useCallback } from "react";

const URL = `http://${SERVER}`;

export function useSendPoint(lobbyId: string, playerTag: string) {
  return useCallback(async (point: [number, number]) => {
    return await fetch(URL + "/ping", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        lobby: lobbyId,
        player: playerTag,
        points: [{
          lng: point[0],
          lat: point[1],
          rad: 0,
          ts: Date.now()
        }],
      }),
    }).then(success => {
      console.log({ success })
      return success
    }, error => {
      console.log({ error })
      return error
    });
  }, [lobbyId, playerTag]);
}
