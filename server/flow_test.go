package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
)

type GameResponse struct {
	Players []PlayerResponse `json:"players"`
}

type PlayerResponse struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Team        string           `json:"team"`
	City        string           `json:"city"`
	LatestPoint *[2]float64      `json:"lastPoint"`
	Trail       [][][][2]float64 `json:"trail"`
	Claimed     [][][][2]float64 `json:"claimed"`
}

func feature(geomType string, coords any, props map[string]any) map[string]any {
	return map[string]any{
		"type":       "Feature",
		"properties": props,
		"geometry":   map[string]any{"type": geomType, "coordinates": coords},
	}
}

func TestFlow(t *testing.T) {
	// reset global state
	lobbiesMu.Lock()
	lobbies = map[string]*Game{}
	lobbiesMu.Unlock()

	r := chi.NewRouter()
	r.Post("/ping", PingEndpoint)
	r.Post("/join", JoinEndpoint)
	r.Get("/state", StateEndpoint)
	srv := httptest.NewServer(r)
	defer srv.Close()

	logFile, err := os.Create("test.log")
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()

	getState := func(lobby string) GameResponse {
		resp, err := http.Get(srv.URL + "/state?lobby=" + lobby)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var game GameResponse
		json.NewDecoder(resp.Body).Decode(&game)
		return game
	}

	doPing := func(lobby, player string, points [][2]float64) {
		t.Helper()
		body, _ := json.Marshal(PingRequest{Lobby: lobby, Player: player, Points: points})
		resp, err := http.Post(srv.URL+"/ping", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("ping failed (%d): %s", resp.StatusCode, b)
		}
	}

	logStep := func(label string, lobby string, inputPoints [][2]float64, route [][2]float64) {
		game := getState(lobby)
		features := []map[string]any{}

		// state: claimed polygons (purple)
		for _, p := range game.Players {
			for _, poly := range p.Claimed {
				features = append(features, feature("Polygon", poly, map[string]any{
					"type": "claimed", "player": p.ID,
					"stroke": "#9b59b6", "stroke-width": 2, "fill": "#9b59b6", "fill-opacity": 0.25,
				}))
			}
		}

		// state: trail polygons (green)
		for _, p := range game.Players {
			for _, poly := range p.Trail {
				features = append(features, feature("Polygon", poly, map[string]any{
					"type": "trail", "player": p.ID,
					"stroke": "#2ecc71", "stroke-width": 2, "fill": "#2ecc71", "fill-opacity": 0.15,
				}))
			}
		}

		// OSRM route for this step (blue)
		if len(route) >= 2 {
			features = append(features, feature("LineString", route, map[string]any{
				"type":   "route",
				"stroke": "#3498db", "stroke-width": 4, "stroke-opacity": 0.7,
			}))
		}

		// input ping points (red markers)
		for i, pt := range inputPoints {
			features = append(features, feature("Point", pt, map[string]any{
				"type": "input", "index": i,
				"marker-color": "#e74c3c", "marker-size": "small",
			}))
		}

		// state: latestPoint (orange)
		for _, p := range game.Players {
			if p.LatestPoint != nil {
				features = append(features, feature("Point", p.LatestPoint, map[string]any{
					"type": "latestPoint", "player": p.ID,
					"marker-color": "#f39c12", "marker-size": "medium",
				}))
			}
		}

		geojson := map[string]any{"type": "FeatureCollection", "features": features}
		b, _ := json.Marshal(geojson)
		link := "https://geojson.io/#data=data:application/json," + url.PathEscape(string(b))
		fmt.Fprintf(logFile, "=== %s ===\n%s\n\n", label, link)
		t.Logf("%s: logged to test.log", label)
	}

	// pingAndLog: gets state before to determine OSRM input, pings, calls OSRM
	// from the test to get the route, then logs everything.
	pingAndLog := func(label string, lobby, player string, points [][2]float64) {
		t.Helper()
		before := getState(lobby)

		// figure out what OSRM input the server will use
		var latestPoint *[2]float64
		for _, p := range before.Players {
			if p.ID == player {
				latestPoint = p.LatestPoint
				break
			}
		}
		continuing := pointsWithinMeters(latestPoint, &points[0], 1000, "nyc")
		osrmInput := points
		if continuing && latestPoint != nil {
			osrmInput = append([][2]float64{*latestPoint}, points...)
		}

		// call OSRM ourselves to get the route for visualization
		var route [][2]float64
		if len(osrmInput) >= 2 {
			route, _ = snapToRoads(osrmInput)
		}

		doPing(lobby, player, points)
		logStep(label, lobby, points, route)
	}

	// --- join ---
	joinBody, _ := json.Marshal(JoinRequest{Player: "test", Name: "tester", City: "nyc"})
	joinResp, err := http.Post(srv.URL+"/join", "application/json", bytes.NewReader(joinBody))
	if err != nil {
		t.Fatal(err)
	}
	defer joinResp.Body.Close()
	var joinData struct {
		Lobby string `json:"lobby"`
	}
	json.NewDecoder(joinResp.Body).Decode(&joinData)
	lobby := joinData.Lobby
	t.Logf("lobby: %s", lobby)

	pingAndLog("setup: 1st point (single)", lobby, "test", [][2]float64{
		{-73.99109722135462, 40.74523229521077},
		{-73.99074181962914, 40.74576239716069},
		{-73.98998103780977, 40.74578764000515},
		{-73.9889703641525, 40.74533326733669},
		{-73.98882042904981, 40.744870477168064},
		{-73.98947570098112, 40.74476950433947},
		{-73.98983110270711, 40.74492096352441},
		{-73.98967561445218, 40.74486626996966},
		{-73.99023648280047, 40.745101872654516},
		{-73.99151370775202, 40.74563618279393},
	})
}
