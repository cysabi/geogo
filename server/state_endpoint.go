package main

import (
	"encoding/json"
	"net/http"
)

// StateEndpoint returns the full lobby state for a given lobby code and player id.
// GET /state?lobby=CODE&player=ID
func StateEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	lobbyCode := r.URL.Query().Get("lobby")

	lobbiesMu.RLock()
	defer lobbiesMu.RUnlock()
	game := lobbies[lobbyCode]

	json.NewEncoder(w).Encode(game)
}
