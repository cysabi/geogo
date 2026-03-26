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
	lobby.Players = append(lobby.Players, &Player{
		ID:   "ebf22b7d-1b2f-433f-8402-118f8d8dbf56",
		Name: "Claire",
		Team: "red",
		City: "nyc",
	})

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Post("/ping", PingEndpoint)
	r.Post("/join", JoinEndpoint)
	r.Get("/state", StateEndpoint)

	log.Println("listening on :9090")
	log.Fatal(http.ListenAndServe(":9090", r))
}
