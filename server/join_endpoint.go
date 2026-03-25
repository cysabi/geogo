package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

type JoinRequest struct {
	Lobby  string `json:"lobby"`
	Player string `json:"player"`
	Name   string `json:"name"`
	Team   string `json:"team"`
	City   string `json:"city"`
}

func newLobbyCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func JoinEndpoint(w http.ResponseWriter, r *http.Request) {
	var req JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Player == "" {
		http.Error(w, "player is required", http.StatusBadRequest)
		return
	}

	lobbiesMu.Lock()
	defer lobbiesMu.Unlock()

	// create lobby if none specified
	if req.Lobby == "" {
		req.Lobby = newLobbyCode()
		lobbies[req.Lobby] = &Game{}
	}
	game, ok := lobbies[req.Lobby]
	if !ok {
		http.Error(w, "lobby not found", http.StatusNotFound)
		return
	}

	// find or create player
	var player *Player
	for _, p := range game.Players {
		if p.ID == req.Player {
			player = p
			break
		}
	}
	if player == nil {
		player = &Player{ID: req.Player}
		game.Players = append(game.Players, player)
	}
	player.Name = req.Name
	player.Team = req.Team
	player.City = req.City

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"lobby": req.Lobby, "game": game})
}
