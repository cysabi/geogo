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
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Post("/ping", PingEndpoint)
	r.Post("/join", JoinEndpoint)
	r.Get("/state", StateEndpoint)

	log.Println("listening on :9090")
	log.Fatal(http.ListenAndServe(":9090", r))
}
