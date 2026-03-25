package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"

	"github.com/twpayne/go-geos"
)

const TRAIL_WIDTH = 8

type PingRequest struct {
	Lobby  string       `json:"lobby"`
	Player string       `json:"player"`
	Points [][2]float64 `json:"points"` // [lng, lat] pairs, ascending order (oldest first)
}

// PingEndpoint receives location pings from a client and updates game state.
// POST /ping { lobby, player, points }
func PingEndpoint(w http.ResponseWriter, r *http.Request) {
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
		if p.ID == req.Player {
			player = p
			break
		}
	}
	if player == nil {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}

	if err := updatePlayerState(game, player, req.Points); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(nil)
}

func updatePlayerState(game *Game, p *Player, points [][2]float64) error {
	first := points[0]
	continuing := pointsWithinMeters(p.LatestPoint, &first, 1000, p.City)

	// build the segment to snap
	var raw [][2]float64
	if continuing {
		raw = append([][2]float64{*p.LatestPoint}, points...)
	} else if len(points) >= 2 {
		raw = points
	} else {
		// single point, not continuing — just update LatestPoint
		last := points[len(points)-1]
		p.LatestPoint = &last
		return nil
	}

	segment, err := snapToRoads(raw)
	if err != nil {
		return err
	}
	if len(segment) < 2 {
		last := points[len(points)-1]
		p.LatestPoint = &last
		return nil
	}

	// buffer the segment and union into trail
	line := geos.NewLineString(toGeosCoords(segment))
	bufDeg := metersToDeg(TRAIL_WIDTH, true, p.City)
	strip := line.Buffer(bufDeg, 8)

	if p.Trail == nil {
		p.Trail = strip
	} else {
		p.Trail = p.Trail.Union(strip)
	}

	// update LatestPoint from snapped segment
	last := segment[len(segment)-1]
	p.LatestPoint = &last

	// detect holes in trail → claim enclosed areas
	claimHoles(game, p)

	return nil
}

// claimHoles finds interior rings (holes) in the player's Trail,
// buffers them, claims them as territory, and subtracts from Trail.
func claimHoles(game *Game, player *Player) {
	if player.Trail == nil {
		return
	}

	var holes []*geos.Geom
	for i := range player.Trail.NumGeometries() {
		poly := player.Trail.Geometry(i)
		for j := range poly.NumInteriorRings() {
			ring := poly.InteriorRing(j)
			hole := geos.NewPolygon([][][]float64{ring.CoordSeq().ToCoords()})
			bufDeg := metersToDeg(10, true, player.City)
			holes = append(holes, hole.Buffer(bufDeg, 8))
		}
	}

	if len(holes) == 0 {
		return
	}

	// union all holes into one claimed area
	var newClaimed *geos.Geom
	for _, h := range holes {
		if newClaimed == nil {
			newClaimed = h
		} else {
			newClaimed = newClaimed.Union(h)
		}
	}

	// add to player's claimed territory
	if player.Claimed == nil {
		player.Claimed = newClaimed
	} else {
		player.Claimed = player.Claimed.Union(newClaimed)
	}

	// subtract claimed area from player's trail
	player.Trail = player.Trail.Difference(newClaimed)
	if player.Trail.IsEmpty() {
		player.Trail = nil
	}

	// subtract from opponents
	for _, opponent := range game.Players {
		if opponent.ID == player.ID {
			continue
		}
		if opponent.Trail != nil {
			opponent.Trail = opponent.Trail.Difference(newClaimed)
			if opponent.Trail.IsEmpty() {
				opponent.Trail = nil
			}
		}
		if opponent.Claimed != nil {
			opponent.Claimed = opponent.Claimed.Difference(newClaimed)
			if opponent.Claimed.IsEmpty() {
				opponent.Claimed = nil
			}
		}
	}
}

func snapToRoads(points [][2]float64) ([][2]float64, error) {
	coords := ""
	for i, p := range points {
		if i > 0 {
			coords += ";"
		}
		coords += fmt.Sprintf("%f,%f", p[0], p[1])
	}
	resp, err := http.Get("https://router.project-osrm.org/match/v1/foot/" + coords + "?geometries=geojson&overview=full")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Matchings []struct {
			Geometry struct {
				Coordinates [][2]float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"matchings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Matchings) == 0 {
		return points, nil
	}
	return result.Matchings[0].Geometry.Coordinates, nil
}

func pointsWithinMeters(a, b *[2]float64, m float64, city string) bool {
	if a == nil || b == nil {
		return false
	}
	ptA, ptB := geos.NewPointFromXY(a[0], a[1]), geos.NewPointFromXY(b[0], b[1])
	if !ptA.DistanceWithin(ptB, metersToDeg(m, true, city)) {
		return false
	}
	return math.Abs(a[1]-b[1]) < metersToDeg(m, false, city)
}

func metersToDeg(m float64, lng bool, city string) float64 {
	if !lng {
		return m / 111_000
	}
	switch city {
	case "london":
		return m / 69_400
	case "shanghai":
		return m / 95_000
	default: // nyc
		return m / 84_400
	}
}
