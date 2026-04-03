package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/twpayne/go-geos"
)

type PingRequest struct {
	Lobby  string      `json:"lobby"`
	Player string      `json:"player"`
	Points []PingPoint `json:"points"`
}

type PingPoint struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
	Radius    float64 `json:"rad"`
	Timestamp float64 `json:"ts"`
}

// PingEndpoint receives location pings from a client and updates game state.
// POST /ping { lobby, player, points }
func PingEndpoint(w http.ResponseWriter, r *http.Request) {
	// validate request
	w.Header().Set("Content-Type", "application/json")
	var req PingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Lobby == "" || req.Player == "" {
		http.Error(w, "lobby and player are required", http.StatusBadRequest)
		return
	}
	if len(req.Points) == 0 {
		http.Error(w, "points is required", http.StatusBadRequest)
		return
	}

	lobbiesMu.Lock()
	defer lobbiesMu.Unlock()

	// get game & player
	game := lobbies[req.Lobby]
	if game == nil {
		http.Error(w, "lobby not found", http.StatusNotFound)
		return
	}
	var player *Player
	for _, p := range game.Players {
		if p.Tag == req.Player {
			player = p
			break
		}
	}
	if player == nil {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}

	// update player state
	affected, err := updatePlayerState(game, player, req.Points)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// emit to ws
	messages := PreparePlayerUpdates(req.Lobby, affected)

	go BroadcastPrepared(req.Lobby, messages)

	json.NewEncoder(w).Encode(nil)
}

func updatePlayerState(game *Game, p *Player, points []PingPoint) ([]string, error) {
	// first call - setup latest point
	if p.LatestPoint == nil {
		if len(points) == 1 {
			p.LatestPoint = &[2]float64{points[0].Longitude, points[0].Latitude}
			p.LatestTs = &points[0].Timestamp
			return []string{p.Tag}, nil
		}
	}

	// prepend latest point
	points = append([]PingPoint{PingPoint{
		Longitude: p.LatestPoint[0],
		Latitude:  p.LatestPoint[1],
		Timestamp: *p.LatestTs,
	}}, points...)

	// snap to roads
	segments, err := snapToRoads(points)
	if err != nil {
		return nil, err
	}

	// build multilinestring and union into the trail
	lines := make([]*geos.Geom, len(segments))
	for i, seg := range segments {
		lines[i] = geos.NewLineString(seg)
	}
	lastSeg := segments[len(segments)-1]
	lastPoint := lastSeg[len(lastSeg)-1]
	p.LatestPoint = &[2]float64{lastPoint[0], lastPoint[1]}
	p.LatestTs = &points[len(points)-1].Timestamp
	p.Trail = p.Trail.Union(geos.NewCollection(geos.TypeIDMultiLineString, lines)).UnaryUnion()

	// detect holes in trail → claim enclosed areas
	affected := claim(game, p)

	affected = append([]string{p.Tag}, affected...)

	return affected, nil
}

func claim(game *Game, player *Player) []string {
	// get claimed areas
	claimed, cuts, dangles, _ := player.Trail.Union(player.Claimed.Boundary()).PolygonizeFull()
	// add to player's claimed territory
	player.Claimed = player.Claimed.Union(claimed)
	if player.Claimed.TypeID() == geos.TypeIDPolygon {
		player.Claimed = geos.NewCollection(geos.TypeIDMultiPolygon, []*geos.Geom{player.Claimed})
	}
	// trail becomes only the leftover lines (cuts + dangles)
	player.Trail = cuts.Union(dangles).Difference(player.Claimed).UnaryUnion()
	if player.Trail.TypeID() == geos.TypeIDLineString {
		player.Trail = geos.NewCollection(geos.TypeIDMultiLineString, []*geos.Geom{player.Trail})
	}

	// subtract from opponents
	var affected []string
	for _, opponent := range game.Players {
		if opponent.Tag == player.Tag {
			continue
		}

		affect := false
		opponent.Trail, affect = clearOpponentLines(opponent.Trail, claimed)
		if opponent.Trail.TypeID() == geos.TypeIDLineString {
			opponent.Trail = geos.NewCollection(geos.TypeIDMultiLineString, []*geos.Geom{opponent.Trail})
		}
		opponent.Claimed = opponent.Claimed.Difference(claimed)
		if opponent.Claimed.TypeID() == geos.TypeIDPolygon {
			opponent.Claimed = geos.NewCollection(geos.TypeIDMultiPolygon, []*geos.Geom{opponent.Claimed})
		}

		if affect {
			affected = append(affected, opponent.Tag)
		}
	}

	return affected
}

func clearOpponentLines(trail, claimed *geos.Geom) (*geos.Geom, bool) {
	newTrail := trail.Difference(claimed)
	n := newTrail.NumGeometries()

	lines := make([]*geos.Geom, n)
	for i := range n {
		lines[i] = newTrail.Geometry(i)
	}

	visited := map[int]bool{}
	var toVisit []int
	for i, line := range lines {
		if !line.Intersects(claimed) {
			continue
		}
		visited[i] = true
		toVisit = append(toVisit, i)
	}

	for i := 0; i < len(toVisit); i++ {
		for j, line := range lines {
			if !visited[j] && line.Intersects(lines[toVisit[i]]) {
				visited[j] = true
				toVisit = append(toVisit, j)
			}
		}
	}

	infected := make([]*geos.Geom, len(toVisit))
	for i, idx := range toVisit {
		infected[i] = lines[idx]
	}
	newTrail = newTrail.Difference(geos.NewCollection(geos.TypeIDMultiLineString, infected)).UnaryUnion()
	return newTrail, !newTrail.Equals(trail)
}

func snapToRoads(points []PingPoint) ([][][]float64, error) {
	segs := make([][]float64, len(points))
	for i, p := range points {
		segs[i] = []float64{p.Longitude, p.Latitude}
	}
	s := make([][][]float64, 1)
	s[0] = segs
	return s, nil

	// TODO :: ^ TEMP

	coords := make([]string, len(points))
	radiuses := make([]string, len(points))
	timestamps := make([]string, len(points))

	for i, p := range points {
		coords[i] = fmt.Sprintf("%f,%f", p.Longitude, p.Latitude)
		radiuses[i] = fmt.Sprintf("%f", p.Radius)
		timestamps[i] = fmt.Sprintf("%f", p.Timestamp)
	}

	uri := "https://router.project-osrm.org/match/v1/foot/" + strings.Join(coords, ";") + "?tidy=true&geometries=geojson"
	uri = uri + "&radiuses=" + strings.Join(radiuses, ";")
	uri = uri + "&timestamps=" + strings.Join(timestamps, ";")

	fmt.Printf("uri: %#v\n", uri)

	resp, err := http.Get(uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Matchings []struct {
			Geometry struct {
				Coordinates [][]float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"matchings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	segments := make([][][]float64, len(result.Matchings))
	for i, m := range result.Matchings {
		segments[i] = m.Geometry.Coordinates
	}
	return segments, nil
}
