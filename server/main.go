package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/golang/geo/s2"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	LineWidthMeters    = 10.0
	NearbyRadiusMeters = 20.0
	EarthRadiusMeters  = 6_371_008.8
	OSRMBaseURL        = "http://router.project-osrm.org/route/v1/foot"
)

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

type TimedPoint struct {
	Point     s2.Point  `json:"-"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Timestamp time.Time `json:"timestamp"`
}

type PlayerLine struct {
	Points []TimedPoint `json:"points"`
}

type ClaimedArea struct {
	Loop *s2.Loop `json:"-"`
	// For JSON serialization we keep the raw lat/lngs.
	Vertices []LatLng `json:"vertices"`
}

type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Player struct {
	ID      string        `json:"id"`
	Team    string        `json:"team"`
	City    string        `json:"city"`
	Lines   []PlayerLine  `json:"lines"`
	Claimed []ClaimedArea `json:"claimed"`
}

type GameState struct {
	Colors  []string  `json:"colors"`
	Players []*Player `json:"players"`
	mu      sync.RWMutex
}

// ---------------------------------------------------------------------------
// Global state
// ---------------------------------------------------------------------------

var state = &GameState{
	Colors:  []string{"red", "blue", "green", "yellow"},
	Players: []*Player{},
}

// ---------------------------------------------------------------------------
// Coordinate helpers
// ---------------------------------------------------------------------------

func llToPoint(lat, lng float64) s2.Point {
	return s2.PointFromLatLng(s2.LatLngFromDegrees(lat, lng))
}

func pointToLL(p s2.Point) (float64, float64) {
	ll := s2.LatLngFromPoint(p)
	return ll.Lat.Degrees(), ll.Lng.Degrees()
}

// distanceMeters returns the great-circle distance between two s2.Points.
func distanceMeters(a, b s2.Point) float64 {
	angle := a.Distance(b) // radians on the unit sphere
	return float64(angle) * EarthRadiusMeters
}

// metersToAngle converts a metric distance to a unit-sphere angle (radians).
func metersToAngle(m float64) float64 {
	return m / EarthRadiusMeters
}

// polylineFromTimedPoints builds an s2.Polyline for spatial queries.
func polylineFromTimedPoints(pts []TimedPoint) *s2.Polyline {
	out := make(s2.Polyline, len(pts))
	for i, p := range pts {
		out[i] = p.Point
	}
	return &out
}

// ---------------------------------------------------------------------------
// OSRM routing helper
// ---------------------------------------------------------------------------

type osrmResponse struct {
	Routes []struct {
		Geometry struct {
			Coordinates [][]float64 `json:"coordinates"`
		} `json:"geometry"`
	} `json:"routes"`
}

// GetOSRMRoute calls the public OSRM demo server to get a walkable route
// between two lat/lng pairs. Returns the intermediate points (excluding the
// endpoints themselves so they can be stitched without duplication).
func GetOSRMRoute(fromLat, fromLng, toLat, toLng float64) ([]TimedPoint, error) {
	url := fmt.Sprintf(
		"%s/%f,%f;%f,%f?overview=full&geometries=geojson",
		OSRMBaseURL, fromLng, fromLat, toLng, toLat,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("osrm request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("osrm read body: %w", err)
	}

	var osrm osrmResponse
	if err := json.Unmarshal(body, &osrm); err != nil {
		return nil, fmt.Errorf("osrm decode: %w", err)
	}
	if len(osrm.Routes) == 0 {
		return nil, fmt.Errorf("osrm returned no routes")
	}

	coords := osrm.Routes[0].Geometry.Coordinates
	now := time.Now()
	// Skip the first and last coordinate (they duplicate the from/to points).
	var pts []TimedPoint
	for i := 1; i < len(coords)-1; i++ {
		lng, lat := coords[i][0], coords[i][1]
		pts = append(pts, TimedPoint{
			Point:     llToPoint(lat, lng),
			Lat:       lat,
			Lng:       lng,
			Timestamp: now,
		})
	}
	return pts, nil
}

// ---------------------------------------------------------------------------
// Helper: find nearest recent point within radius
// ---------------------------------------------------------------------------

// NearestRecentPoint scans every point across all of a player's polylines and
// returns the line index + point index of the most recent point that falls
// within `radiusMeters` of `target`. Returns (-1, -1) if nothing is close.
func NearestRecentPoint(player *Player, target s2.Point, radiusMeters float64) (lineIdx, ptIdx int) {
	lineIdx, ptIdx = -1, -1
	var best time.Time

	for li, line := range player.Lines {
		for pi, tp := range line.Points {
			if distanceMeters(target, tp.Point) <= radiusMeters {
				if tp.Timestamp.After(best) {
					best = tp.Timestamp
					lineIdx = li
					ptIdx = pi
				}
			}
		}
	}
	return lineIdx, ptIdx
}

// ---------------------------------------------------------------------------
// Helper: join polyline to point via OSRM route
// ---------------------------------------------------------------------------

// JoinPolylineToPoint appends a routed path from the last point of
// player.Lines[lineIdx] to `dest`, mutating the line in place.
func JoinPolylineToPoint(player *Player, lineIdx int, dest TimedPoint) error {
	line := &player.Lines[lineIdx]
	last := line.Points[len(line.Points)-1]

	route, err := GetOSRMRoute(last.Lat, last.Lng, dest.Lat, dest.Lng)
	if err != nil {
		return err
	}

	line.Points = append(line.Points, route...)
	line.Points = append(line.Points, dest)
	return nil
}

// ---------------------------------------------------------------------------
// Helper: polyline ↔ polyline intersection (width-aware)
// ---------------------------------------------------------------------------

// segmentMinDist returns the minimum distance in meters between two edges
// (a0→a1) and (b0→b1). It samples the closest-point-on-edge approach.
func segmentMinDist(a0, a1, b0, b1 s2.Point) float64 {
	edge1 := s2.Edge{V0: a0, V1: a1}
	edge2 := s2.Edge{V0: b0, V1: b1}

	// Use the s2 EdgePairMinDistance which returns a ChordAngle.
	ca := s2.EdgePairMinDistance(edge1.V0, edge1.V1, edge2.V0, edge2.V1, s2.InfChordAngle())
	return ca.Angle().Radians() * EarthRadiusMeters
}

// PolylinesIntersect returns true if any edge of A is within LineWidthMeters
// of any edge of B (i.e. the "buffered" lines overlap).
// When true it also returns the indices (on A and B) of the first pair of
// edges that triggered the intersection.
func PolylinesIntersect(a, b []TimedPoint) (bool, int, int) {
	for i := 0; i < len(a)-1; i++ {
		for j := 0; j < len(b)-1; j++ {
			d := segmentMinDist(a[i].Point, a[i+1].Point, b[j].Point, b[j+1].Point)
			if d <= LineWidthMeters {
				return true, i, j
			}
		}
	}
	return false, -1, -1
}

// ---------------------------------------------------------------------------
// Helper: point inside any claimed polygon
// ---------------------------------------------------------------------------

func PointInAnyClaimed(players []*Player, pt s2.Point) bool {
	for _, p := range players {
		for _, c := range p.Claimed {
			if c.Loop.ContainsPoint(pt) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Helper: create polygon (shape) from a self-intersecting line
// ---------------------------------------------------------------------------

// CreateShapeFromLoop extracts the loop portion of a polyline between indices
// `from` and `to` (inclusive) and builds a ClaimedArea polygon.
func CreateShapeFromLoop(pts []TimedPoint, from, to int) ClaimedArea {
	loopPts := make([]s2.Point, 0, to-from+1)
	verts := make([]LatLng, 0, to-from+1)

	for i := from; i <= to; i++ {
		loopPts = append(loopPts, pts[i].Point)
		verts = append(verts, LatLng{Lat: pts[i].Lat, Lng: pts[i].Lng})
	}

	loop := s2.LoopFromPoints(loopPts)
	// Normalize so the loop encloses the smaller area.
	loop.Normalize()

	return ClaimedArea{
		Loop:     loop,
		Vertices: verts,
	}
}

// ---------------------------------------------------------------------------
// Helper: subtract already-claimed areas from a new shape
// ---------------------------------------------------------------------------

// SubtractClaimed removes overlap between `area` and every existing claimed
// polygon owned by the player. Full boolean polygon subtraction on the sphere
// is non-trivial; here we approximate by dropping any vertex of `area` that
// falls inside an existing claim, then rebuilding the loop.
//
// For a production system, replace this with a proper polygon-clipping
// library (e.g. S2 BooleanOperation in C++ ported to Go, or a planar
// clipper after projecting to a local tangent plane).
func SubtractClaimed(area ClaimedArea, player *Player) ClaimedArea {
	var kept []s2.Point
	var keptLL []LatLng

	for i, v := range area.Vertices {
		inside := false
		for _, c := range player.Claimed {
			if c.Loop.ContainsPoint(area.Loop.Vertex(i)) {
				inside = true
				break
			}
		}
		if !inside {
			kept = append(kept, area.Loop.Vertex(i))
			keptLL = append(keptLL, v)
		}
	}

	if len(kept) < 3 {
		// Degenerate after subtraction — return empty.
		return ClaimedArea{Loop: s2.EmptyLoop(), Vertices: nil}
	}

	loop := s2.LoopFromPoints(kept)
	loop.Normalize()
	return ClaimedArea{Loop: loop, Vertices: keptLL}
}

// ---------------------------------------------------------------------------
// Helper: replace lines with shape, keep extruding remainder
// ---------------------------------------------------------------------------

// ReplaceLineWithShape removes the loop segment [from..to] from the line and
// adds the resulting polygon to claimed. If there are leftover points before
// `from` or after `to`, they become new independent polylines.
func ReplaceLineWithShape(player *Player, lineIdx, from, to int) {
	line := player.Lines[lineIdx]

	// Build & subtract the shape.
	shape := CreateShapeFromLoop(line.Points, from, to)
	shape = SubtractClaimed(shape, player)
	if shape.Loop != nil && !shape.Loop.IsEmpty() {
		player.Claimed = append(player.Claimed, shape)
	}

	// Build residual polylines from the parts of the line outside the loop.
	var newLines []PlayerLine

	// Keep points before the loop start (the "tail" leading in).
	if from > 0 {
		newLines = append(newLines, PlayerLine{Points: line.Points[:from+1]})
	}
	// Keep points after the loop end (the "extruding" part).
	if to < len(line.Points)-1 {
		newLines = append(newLines, PlayerLine{Points: line.Points[to:]})
	}

	// Remove original line and splice in the residuals.
	player.Lines = append(player.Lines[:lineIdx], player.Lines[lineIdx+1:]...)
	player.Lines = append(player.Lines, newLines...)
}

// ---------------------------------------------------------------------------
// Helper: destroy an opponent line and any connected lines
// ---------------------------------------------------------------------------

// DestroyLineAndConnected removes `lines[targetIdx]` and any other lines
// whose endpoints are within LineWidthMeters of the removed line's endpoints
// (recursively).
func DestroyLineAndConnected(lines []PlayerLine, targetIdx int) []PlayerLine {
	destroyed := map[int]bool{targetIdx: true}
	queue := []int{targetIdx}

	endpointsOf := func(l PlayerLine) (s2.Point, s2.Point) {
		return l.Points[0].Point, l.Points[len(l.Points)-1].Point
	}

	for len(queue) > 0 {
		ci := queue[0]
		queue = queue[1:]
		startC, endC := endpointsOf(lines[ci])

		for i, other := range lines {
			if destroyed[i] {
				continue
			}
			startO, endO := endpointsOf(other)
			if distanceMeters(startC, startO) <= LineWidthMeters ||
				distanceMeters(startC, endO) <= LineWidthMeters ||
				distanceMeters(endC, startO) <= LineWidthMeters ||
				distanceMeters(endC, endO) <= LineWidthMeters {
				destroyed[i] = true
				queue = append(queue, i)
			}
		}
	}

	var kept []PlayerLine
	for i, l := range lines {
		if !destroyed[i] {
			kept = append(kept, l)
		}
	}
	return kept
}

// ---------------------------------------------------------------------------
// Core game logic
// ---------------------------------------------------------------------------

func findPlayer(id string) *Player {
	for _, p := range state.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func opponents(team string) []*Player {
	var out []*Player
	for _, p := range state.Players {
		if p.Team != team {
			out = append(out, p)
		}
	}
	return out
}

// ReceiveLat is the core logic executed when a player pings a new position.
func ReceiveLat(playerID string, lat, lng float64, ts time.Time) error {
	state.mu.Lock()
	defer state.mu.Unlock()

	player := findPlayer(playerID)
	if player == nil {
		return fmt.Errorf("unknown player %s", playerID)
	}

	incomingPt := llToPoint(lat, lng)

	// 1. If position is already inside a claimed shape, ignore.
	if PointInAnyClaimed(state.Players, incomingPt) {
		return nil
	}

	tp := TimedPoint{
		Point:     incomingPt,
		Lat:       lat,
		Lng:       lng,
		Timestamp: ts,
	}

	// 2. Find nearest recent point within 20 m.
	lineIdx, ptIdx := NearestRecentPoint(player, incomingPt, NearbyRadiusMeters)

	if lineIdx == -1 {
		// ------------------------------------------------------------------
		// No nearby point → start a brand-new polyline with a single vertex.
		// ------------------------------------------------------------------
		player.Lines = append(player.Lines, PlayerLine{Points: []TimedPoint{tp}})
		return nil
	}

	// ------------------------------------------------------------------
	// Nearby point found → join to end of that line via OSRM route.
	// ------------------------------------------------------------------
	_ = ptIdx // We always append to the line's tail.
	if err := JoinPolylineToPoint(player, lineIdx, tp); err != nil {
		// If OSRM fails, fall back to a straight-line append.
		player.Lines[lineIdx].Points = append(player.Lines[lineIdx].Points, tp)
	}

	// 3. Check self-intersection (same player's lines).
	for i := 0; i < len(player.Lines); i++ {
		for j := i; j < len(player.Lines); j++ {
			var hit bool
			var ai, _ int
			if i == j {
				// Self-intersection within the same polyline.
				pts := player.Lines[i].Points
				if len(pts) < 4 {
					continue
				}
				// Compare non-adjacent edges.
				hit, ai, _ = selfIntersects(pts)
			} else {
				hit, ai, _ = PolylinesIntersect(
					player.Lines[i].Points,
					player.Lines[j].Points,
				)
			}

			if hit {
				// Create shape, subtract claimed, replace line.
				if i == j {
					ReplaceLineWithShape(player, i, ai, len(player.Lines[i].Points)-1)
				} else {
					// Merge the two lines conceptually; use line i's range.
					ReplaceLineWithShape(player, i, ai, len(player.Lines[i].Points)-1)
				}

				// 4. Check if new shape intersects opponent lines.
				for _, opp := range opponents(player.Team) {
					toDestroy := []int{}
					for oi, ol := range opp.Lines {
						for _, claimed := range player.Claimed {
							for k := 0; k < len(ol.Points)-1; k++ {
								edge := s2.Edge{V0: ol.Points[k].Point, V1: ol.Points[k+1].Point}
								_ = edge
								if claimed.Loop.ContainsPoint(ol.Points[k].Point) {
									toDestroy = append(toDestroy, oi)
									break
								}
							}
						}
					}
					// Destroy from highest index down to avoid shifting.
					for d := len(toDestroy) - 1; d >= 0; d-- {
						opp.Lines = DestroyLineAndConnected(opp.Lines, toDestroy[d])
					}
				}

				// After mutation the indices are stale; break out.
				return nil
			}
		}
	}

	return nil
}

// selfIntersects checks whether a single polyline crosses itself. It compares
// every edge against every non-adjacent edge.
func selfIntersects(pts []TimedPoint) (bool, int, int) {
	for i := 0; i < len(pts)-1; i++ {
		// Start j at i+2 to skip the immediately adjacent edge.
		for j := i + 2; j < len(pts)-1; j++ {
			d := segmentMinDist(pts[i].Point, pts[i+1].Point, pts[j].Point, pts[j+1].Point)
			if d <= LineWidthMeters {
				return true, i, j
			}
		}
	}
	return false, -1, -1
}

// ---------------------------------------------------------------------------
// HTTP handlers
// ---------------------------------------------------------------------------

// SendState serializes the full game state as JSON.
func SendState(w http.ResponseWriter, r *http.Request) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

// PingEndpoint receives a player's lat/lng and updates the game state.
//
//	GET /ping?id=<playerID>&lat=<lat>&lng=<lng>&ts=<unix_ms>
func PingEndpoint(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	playerID := q.Get("id")
	if playerID == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	var lat, lng float64
	if _, err := fmt.Sscanf(q.Get("lat"), "%f", &lat); err != nil {
		http.Error(w, "bad lat", http.StatusBadRequest)
		return
	}
	if _, err := fmt.Sscanf(q.Get("lng"), "%f", &lng); err != nil {
		http.Error(w, "bad lng", http.StatusBadRequest)
		return
	}

	if err := ReceiveLat(playerID, lat, lng, time.Now()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated state to the pinging client.
	SendState(w, r)
}

// ---------------------------------------------------------------------------
// Bootstrap: seed a couple of test players (remove in production)
// ---------------------------------------------------------------------------

func seedTestPlayers() {
	state.Players = append(state.Players,
		&Player{ID: "p1", Team: "red", City: "New York"},
		&Player{ID: "p2", Team: "blue", City: "New York"},
	)
}

// ---------------------------------------------------------------------------
// Unused import guard (math is used via EarthRadiusMeters constant usage)
// ---------------------------------------------------------------------------
var _ = math.Pi

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	seedTestPlayers()

	http.HandleFunc("/ping", PingEndpoint)
	http.HandleFunc("/state", SendState)

	addr := ":8080"
	log.Printf("game server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
