package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

type JoinRequest struct {
	Lobby  string    `json:"lobby"`
	Colors [2]string `json:"colors"`
	Player string    `json:"player"`
	Team   string    `json:"team"`
	City   string    `json:"city"`
}

func JoinEndpoint(w http.ResponseWriter, r *http.Request) {
	var req JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Player == "" {
		http.Error(w, "player tag is required", http.StatusBadRequest)
		return
	}

	lobbiesMu.Lock()
	defer lobbiesMu.Unlock()

	// create lobby if none specified
	if req.Lobby == "" {
		req.Lobby = newLobbyCode()
		lobbies[req.Lobby] = &Game{Colors: req.Colors}
	}
	game, _ := lobbies[req.Lobby]

	// find or create player
	var player *Player
	for _, p := range game.Players {
		if p.Tag == req.Player {
			player = p
			break
		}
	}
	if player == nil {
		player = (&Player{Tag: req.Player}).New()
		game.Players = append(game.Players, player)
	}
	player.Team = req.Team
	player.City = req.City

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"lobby": req.Lobby, "game": game})
}

func newLobbyCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
