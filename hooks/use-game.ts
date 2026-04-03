import { SERVER, type GameState, type Player } from "@/components/state";
import { useEffect, useRef, useState } from "react";

const URL = `ws://${SERVER}/ws`

export function useGame(lobbyId: string, playerTag: string) {
  const [state, setState] = useState<GameState | null>(null);
  const [error, setError] = useState<string | null>(null);

  const wsRef = useRef<WebSocket | null>(null);
  useEffect(() => {
    const ws = new WebSocket(URL);
    wsRef.current = ws;
    ws.onopen = () => ws.send(JSON.stringify({ type: "me", player: playerTag, lobby: lobbyId }));
    ws.onmessage = (e) => {
      console.log(e.data)
      const msg = JSON.parse(e.data);
      switch (msg.type) {
        case "state":
          setState(msg.game);
          break;
        case "update":
          setState((prev) => {
            if (!prev) return prev;
            const updated = msg.player as Player;
            return {
              ...prev,
              players: prev.players.some((p) => p.tag === updated.tag)
                ? prev.players.map((p) => (p.tag === updated.tag ? updated : p))
                : [...prev.players, updated],
            };
          });
          break;
      }
    };
    ws.onerror = () => setError("WebSocket error");
    return () => ws.close();
  }, [lobbyId, playerTag]);

  return { state, error };
}
