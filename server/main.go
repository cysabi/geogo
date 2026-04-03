package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var (
	lobbies   = map[string]*Game{}
	lobbiesMu sync.RWMutex
)

func main() {

	lobbies["test12"] = &Game{}
	lobby := lobbies["test12"]
	lobby.Players = append(lobby.Players, (&Player{
		Tag:  "claire",
		Team: "red",
		City: "nyc",
	}).New())
	lobby.Colors = [2]string{"#ff0000", "#0000ff"}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Post("/ping", PingEndpoint)
	r.Post("/join", JoinEndpoint)
	r.Get("/state", StateEndpoint)
	r.Get("/ws", WSEndpoint)

	log.Println("listening on :9090")
	log.Fatal(http.ListenAndServe(":9090", r))
}
