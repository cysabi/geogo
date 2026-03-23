package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/engelsjk/polygol"
	"github.com/golang/geo/s1"
	"github.com/golang/geo/s2"
)

// LineWidth is the physical width of a drawn line in meters.
// Two lines "intersect" if their centerlines are within this distance,
// meaning their 5m-wide buffers overlap.
const LineWidth = 20.0

// NearbyRadius is the search radius (meters) for reconnecting to an existing point.
const NearbyRadius = 500.0

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func metersToAngle(m float64) s1.Angle {
	return s1.Angle(m / 6_371_000.0)
}

func metersToChordAngle(m float64) s1.ChordAngle {
	return s1.ChordAngleFromAngle(metersToAngle(m))
}

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// TimestampedPoint pairs an S2 point with the moment it was recorded.
type TimestampedPoint struct {
	Point     s2.Point
	Timestamp time.Time
}

// PlayerLine is a polyline the player is actively drawing, with per-vertex timestamps.
type PlayerLine struct {
	Points   []TimestampedPoint
	Polyline *s2.Polyline
}

// rebuildPolyline reconstructs the s2.Polyline from the timestamped points.
func (pl *PlayerLine) rebuildPolyline() {
	pts := make([]s2.Point, len(pl.Points))
	for i, tp := range pl.Points {
		pts[i] = tp.Point
	}
	poly := s2.Polyline(pts)
	pl.Polyline = &poly
}

type Player struct {
	ID      string
	Team    string
	City    string
	Lines   []*PlayerLine
	Claimed []*s2.Polygon
}

type GameState struct {
	mu      sync.RWMutex
	Colors  []string
	Players map[string]*Player
}

func NewGameState(colors []string) *GameState {
	return &GameState{
		Colors:  colors,
		Players: make(map[string]*Player),
	}
}

// ---------------------------------------------------------------------------
// JSON response types (S2 types don't marshal to friendly JSON)
// ---------------------------------------------------------------------------

type LatLngJSON struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type PolylineJSON struct {
	Points []LatLngJSON `json:"points"`
}

type PolygonJSON struct {
	Loops [][]LatLngJSON `json:"loops"`
}

type PlayerJSON struct {
	ID      string         `json:"id"`
	Team    string         `json:"team"`
	City    string         `json:"city"`
	Lines   []PolylineJSON `json:"lines"`
	Claimed []PolygonJSON  `json:"claimed"`
}

type StateJSON struct {
	Colors  []string     `json:"colors"`
	Players []PlayerJSON `json:"players"`
}

func pointToLatLngJSON(p s2.Point) LatLngJSON {
	ll := s2.LatLngFromPoint(p)
	return LatLngJSON{Lat: ll.Lat.Degrees(), Lng: ll.Lng.Degrees()}
}

// SendState serialises the full game state to JSON and writes it out.
func (gs *GameState) SendState(w http.ResponseWriter) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	out := StateJSON{Colors: gs.Colors}
	for _, p := range gs.Players {
		pj := PlayerJSON{ID: p.ID, Team: p.Team, City: p.City}
		for _, line := range p.Lines {
			var pts []LatLngJSON
			for _, tp := range line.Points {
				pts = append(pts, pointToLatLngJSON(tp.Point))
			}
			pj.Lines = append(pj.Lines, PolylineJSON{Points: pts})
		}
		for _, poly := range p.Claimed {
			var pjson PolygonJSON
			for li := 0; li < poly.NumLoops(); li++ {
				loop := poly.Loop(li)
				var verts []LatLngJSON
				for vi := 0; vi < loop.NumVertices(); vi++ {
					verts = append(verts, pointToLatLngJSON(loop.Vertex(vi)))
				}
				pjson.Loops = append(pjson.Loops, verts)
			}
			pj.Claimed = append(pj.Claimed, pjson)
		}
		out.Players = append(out.Players, pj)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// ---------------------------------------------------------------------------
// Helper: point inside any claimed polygon?
// ---------------------------------------------------------------------------

func (gs *GameState) pointInAnyClaimed(pt s2.Point) bool {
	for _, p := range gs.Players {
		for _, poly := range p.Claimed {
			if poly.ContainsPoint(pt) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Helper: find the most-recent point within `radius` meters across a
// player's lines. Returns the line index, vertex index, and true if found.
// ---------------------------------------------------------------------------

// NearbyResult holds the outcome of a proximity search against a player's lines.
type NearbyResult struct {
	LineIdx   int       // which PlayerLine was closest
	EdgeIdx   int       // which edge on that line (-1 if snapped to a lone vertex)
	SnapPoint s2.Point  // the actual closest point on the edge
	Timestamp time.Time // the later timestamp of the edge's two endpoints
}

// findNearbyPoint searches every edge of every player line for the closest
// point within radiusMeters. It returns the projected point on the edge
// (not just the nearest vertex) and picks the most-recent match by timestamp.
func findNearbyPoint(player *Player, pt s2.Point, radiusMeters float64) (result NearbyResult, found bool) {
	limit := metersToChordAngle(radiusMeters)
	target := s2.NewMinDistanceToPointTarget(pt)
	opts := s2.NewClosestEdgeQueryOptions().MaxResults(1).DistanceLimit(limit)

	var bestTime time.Time

	for li, line := range player.Lines {
		if line.Polyline == nil {
			continue
		}

		// --- Handle single-vertex polylines (no edges to query). ---
		if len(*line.Polyline) == 1 {
			d := s2.ChordAngleBetweenPoints(pt, (*line.Polyline)[0])
			if d <= limit && (!found || line.Points[0].Timestamp.After(bestTime)) {
				result = NearbyResult{
					LineIdx:   li,
					EdgeIdx:   -1, // sentinel: lone vertex, not an edge
					SnapPoint: (*line.Polyline)[0],
					Timestamp: line.Points[0].Timestamp,
				}
				found = true
				bestTime = line.Points[0].Timestamp
			}
			continue
		}

		if line.Polyline.NumEdges() == 0 {
			continue
		}

		idx := s2.NewShapeIndex()
		idx.Add(line.Polyline)
		idx.Build()

		query := s2.NewClosestEdgeQuery(idx, opts)
		results := query.FindEdges(target)
		if len(results) == 0 {
			continue
		}

		r := results[0]
		edgeIdx := int(r.EdgeID())
		edge := line.Polyline.Edge(edgeIdx)

		// Project pt onto the winning edge to get the exact snap point.
		snap := s2.Project(pt, edge.V0, edge.V1)

		// The edge spans Points[edgeIdx] → Points[edgeIdx+1].
		// Use the later timestamp (when the segment was completed).
		ts := line.Points[edgeIdx].Timestamp
		if line.Points[edgeIdx+1].Timestamp.After(ts) {
			ts = line.Points[edgeIdx+1].Timestamp
		}

		if !found || ts.After(bestTime) {
			result = NearbyResult{
				LineIdx:   li,
				EdgeIdx:   edgeIdx,
				SnapPoint: snap,
				Timestamp: ts,
			}
			found = true
			bestTime = ts
		}
	}
	return
}

// ---------------------------------------------------------------------------
// Helper: fetch a walking route between two lat/lngs from OSRM.
// Returns intermediate S2 points (excluding the start, including the end).
// ---------------------------------------------------------------------------

type osrmGeometry struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"` // [lng, lat]
}

type osrmRoute struct {
	Geometry osrmGeometry `json:"geometry"`
}

type osrmResponse struct {
	Code   string      `json:"code"`
	Routes []osrmRoute `json:"routes"`
}

func fetchRoute(from, to s2.LatLng) ([]s2.Point, error) {
	url := fmt.Sprintf(
		"https://router.project-osrm.org/route/v1/foot/%f,%f;%f,%f?overview=full&geometries=geojson",
		from.Lng.Degrees(), from.Lat.Degrees(),
		to.Lng.Degrees(), to.Lat.Degrees(),
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
	if osrm.Code != "Ok" || len(osrm.Routes) == 0 {
		return nil, fmt.Errorf("osrm returned code %s", osrm.Code)
	}

	coords := osrm.Routes[0].Geometry.Coordinates
	// Skip the first coordinate (it's the `from` point we already have).
	pts := make([]s2.Point, 0, len(coords)-1)
	for i := 1; i < len(coords); i++ {
		ll := s2.LatLngFromDegrees(coords[i][1], coords[i][0])
		pts = append(pts, s2.PointFromLatLng(ll))
	}
	return pts, nil
}

// joinViaRoute extends a PlayerLine to a destination by fetching an OSRM
// walking route and appending the intermediate points.
func joinViaRoute(line *PlayerLine, dest s2.Point, ts time.Time) error {
	if len(line.Points) == 0 {
		return fmt.Errorf("line has no points")
	}

	lastPt := line.Points[len(line.Points)-1]
	from := s2.LatLngFromPoint(lastPt.Point)
	to := s2.LatLngFromPoint(dest)

	routePts, err := fetchRoute(from, to)
	if err != nil {
		return err
	}

	// fetchRoute skips the `from` point and includes `dest`,
	// so append all returned points.
	for _, rp := range routePts {
		line.Points = append(line.Points, TimestampedPoint{Point: rp, Timestamp: ts})
	}
	line.rebuildPolyline()
	return nil
}

// ---------------------------------------------------------------------------
// Helper: detect self-intersection after extending a polyline.
// `oldEdgeCount` is how many edges existed before the extension.
// We check every new edge against every non-adjacent old edge.
// ---------------------------------------------------------------------------

func findSelfIntersection(pl *s2.Polyline, oldEdgeCount int) (
	found bool, crossPt s2.Point, oldEdgeIdx int, newEdgeIdx int,
) {
	limit := metersToChordAngle(LineWidth)
	bestDist := s1.InfChordAngle()

	for ni := oldEdgeCount; ni < pl.NumEdges(); ni++ {
		ne := pl.Edge(ni)
		for oi := 0; oi < oldEdgeCount; oi++ {
			// Skip edges that share a vertex (are adjacent) to avoid
			// false positives.
			if oi == ni-1 || oi == ni+1 || oi == ni {
				continue
			}
			oe := pl.Edge(oi)
			ca, cb := s2.EdgePairClosestPoints(oe.V0, oe.V1, ne.V0, ne.V1)
			dist := s2.ChordAngleBetweenPoints(ca, cb)
			if dist <= limit && dist < bestDist {
				bestDist = dist
				crossPt = s2.Interpolate(0.5, ca, cb)
				oldEdgeIdx = oi
				newEdgeIdx = ni
				found = true
			}
		}
	}
	return
}

// ---------------------------------------------------------------------------
// Helper: extract a polygon from the loop portion of a self-intersecting
// polyline. The loop spans from oldEdgeIdx+1 through newEdgeIdx, closed
// by the crossPt.
// ---------------------------------------------------------------------------

func createPolygonFromSelfIntersection(pl *s2.Polyline, crossPt s2.Point, oldEdgeIdx, newEdgeIdx int) *s2.Polygon {
	pts := *pl
	// Loop vertices: crossPt, V_{oldEdgeIdx+1}, ..., V_{newEdgeIdx}, crossPt
	var loopPts []s2.Point
	loopPts = append(loopPts, crossPt)
	for i := oldEdgeIdx + 1; i <= newEdgeIdx; i++ {
		loopPts = append(loopPts, pts[i])
	}
	// Close back to crossPt (the Loop constructor handles the implicit closing edge).
	loop := s2.LoopFromPoints(loopPts)
	loop.Normalize() // ensure CCW orientation
	return s2.PolygonFromLoops([]*s2.Loop{loop})
}

// ---------------------------------------------------------------------------
// polygol conversion helpers
// ---------------------------------------------------------------------------

// s2PolyToGeom converts an S2 Polygon into a polygol Geom (MultiPolygon coords).
// Each S2 Loop becomes its own single-ring polygon in the MultiPolygon,
// avoiding ambiguity between outer shells and holes.
func s2PolyToGeom(p *s2.Polygon) polygol.Geom {
	var geom polygol.Geom
	for i := 0; i < p.NumLoops(); i++ {
		loop := p.Loop(i)
		ring := make([][]float64, loop.NumVertices()+1) // +1 to close the ring
		for j := 0; j < loop.NumVertices(); j++ {
			ll := s2.LatLngFromPoint(loop.Vertex(j))
			ring[j] = []float64{ll.Lng.Degrees(), ll.Lat.Degrees()}
		}
		// Close the ring by repeating the first vertex.
		ll := s2.LatLngFromPoint(loop.Vertex(0))
		ring[loop.NumVertices()] = []float64{ll.Lng.Degrees(), ll.Lat.Degrees()}
		geom = append(geom, [][][]float64{ring})
	}
	return geom
}

// geomToS2Poly converts a polygol Geom (MultiPolygon) back to an *s2.Polygon.
// Every ring across all polygons is collected into a flat list of S2 loops.
func geomToS2Poly(g polygol.Geom) *s2.Polygon {
	var loops []*s2.Loop
	for _, poly := range g {
		for _, ring := range poly {
			n := len(ring)
			if n < 4 { // need at least 3 unique vertices + closing vertex
				continue
			}
			// Drop the closing duplicate vertex.
			pts := make([]s2.Point, n-1)
			for i := 0; i < n-1; i++ {
				ll := s2.LatLngFromDegrees(ring[i][1], ring[i][0]) // lat, lng
				pts[i] = s2.PointFromLatLng(ll)
			}
			loop := s2.LoopFromPoints(pts)
			loop.Normalize()
			loops = append(loops, loop)
		}
	}
	if len(loops) == 0 {
		return nil
	}
	return s2.PolygonFromLoops(loops)
}

// ---------------------------------------------------------------------------
// Helper: subtract already-claimed polygons from a newly created polygon
// using polygol's exact boolean difference operation.
// ---------------------------------------------------------------------------

func subtractClaimed(newPoly *s2.Polygon, claimed []*s2.Polygon) *s2.Polygon {
	if len(claimed) == 0 {
		return newPoly
	}

	subject := s2PolyToGeom(newPoly)

	clips := make([]polygol.Geom, len(claimed))
	for i, cp := range claimed {
		clips[i] = s2PolyToGeom(cp)
	}

	result, err := polygol.Difference(subject, clips...)
	if err != nil || len(result) == 0 {
		return nil
	}

	return geomToS2Poly(result)
}

// ---------------------------------------------------------------------------
// Helper: after a polygon is carved out of a polyline, return the leftover
// "extruding" tails as new PlayerLines.
//
//   Original: V0 ... V_{oldEdge} [loop] V_{newEdge+1} ... V_end
//   Tail 1 (before loop): V0 ... V_{oldEdge}, crossPt
//   Tail 2 (after loop):  crossPt, V_{newEdge+1} ... V_end
// ---------------------------------------------------------------------------

func splitLineAfterPolygon(line *PlayerLine, crossPt s2.Point, oldEdgeIdx, newEdgeIdx int, ts time.Time) []*PlayerLine {
	var tails []*PlayerLine

	// Tail before the loop (at least 2 vertices to be a valid polyline).
	if oldEdgeIdx+1 >= 1 {
		var pts []TimestampedPoint
		for i := 0; i <= oldEdgeIdx; i++ {
			pts = append(pts, line.Points[i])
		}
		pts = append(pts, TimestampedPoint{Point: crossPt, Timestamp: ts})
		tail := &PlayerLine{Points: pts}
		tail.rebuildPolyline()
		if len(*tail.Polyline) >= 2 {
			tails = append(tails, tail)
		}
	}

	// Tail after the loop.
	if newEdgeIdx+1 < len(line.Points) {
		pts := []TimestampedPoint{{Point: crossPt, Timestamp: ts}}
		for i := newEdgeIdx + 1; i < len(line.Points); i++ {
			pts = append(pts, line.Points[i])
		}
		tail := &PlayerLine{Points: pts}
		tail.rebuildPolyline()
		if len(*tail.Polyline) >= 2 {
			tails = append(tails, tail)
		}
	}

	return tails
}

// ---------------------------------------------------------------------------
// Helper: check if a polygon (buffered by LineWidth) intersects any of an
// opponent's polylines. Returns the indices of intersecting lines.
// ---------------------------------------------------------------------------

func findIntersectingOpponentLines(poly *s2.Polygon, opponent *Player) []int {
	// Build ShapeIndex for the polygon.
	polyIdx := s2.NewShapeIndex()
	polyIdx.Add(poly)
	polyIdx.Build()

	limit := metersToChordAngle(LineWidth)
	var hits []int

	for li, line := range opponent.Lines {
		if line.Polyline == nil || len(*line.Polyline) < 2 {
			continue
		}
		lineIdx := s2.NewShapeIndex()
		lineIdx.Add(line.Polyline)
		lineIdx.Build()

		target := s2.NewMinDistanceToShapeIndexTarget(lineIdx)
		query := s2.NewClosestEdgeQuery(polyIdx, s2.NewClosestEdgeQueryOptions().MaxResults(1))
		if query.IsDistanceLess(target, limit) {
			hits = append(hits, li)
		}
	}
	return hits
}

// ---------------------------------------------------------------------------
// Helper: destroy a line and any lines "connected" to it (sharing an
// endpoint within LineWidth). Works iteratively to cascade.
// ---------------------------------------------------------------------------

func destroyConnectedLines(player *Player, seedIndices []int) {
	destroyed := make(map[int]bool)
	queue := append([]int{}, seedIndices...)

	for len(queue) > 0 {
		idx := queue[0]
		queue = queue[1:]
		if destroyed[idx] || idx >= len(player.Lines) {
			continue
		}
		destroyed[idx] = true

		line := player.Lines[idx]
		if len(line.Points) == 0 {
			continue
		}

		endpoints := []s2.Point{
			line.Points[0].Point,
			line.Points[len(line.Points)-1].Point,
		}
		limit := metersToChordAngle(LineWidth)

		// Find other lines sharing an endpoint.
		for oi, other := range player.Lines {
			if destroyed[oi] || len(other.Points) == 0 {
				continue
			}
			otherEndpoints := []s2.Point{
				other.Points[0].Point,
				other.Points[len(other.Points)-1].Point,
			}
			for _, ep := range endpoints {
				for _, oep := range otherEndpoints {
					if s2.ChordAngleBetweenPoints(ep, oep) <= limit {
						queue = append(queue, oi)
					}
				}
			}
		}
	}

	// Remove destroyed lines in reverse order to preserve indices.
	var kept []*PlayerLine
	for i, line := range player.Lines {
		if !destroyed[i] {
			kept = append(kept, line)
		}
	}
	player.Lines = kept
}

// ---------------------------------------------------------------------------
// Core game logic: ReceivePing
// ---------------------------------------------------------------------------

func (gs *GameState) ReceivePing(playerID string, lat, lng float64, ts time.Time) error {
	pt := s2.PointFromLatLng(s2.LatLngFromDegrees(lat, lng))

	gs.mu.Lock()
	defer gs.mu.Unlock()

	player, ok := gs.Players[playerID]
	if !ok {
		return fmt.Errorf("unknown player %s", playerID)
	}

	// ---- 1. If position is already inside a claimed shape, ignore. --------
	if gs.pointInAnyClaimed(pt) {
		return nil
	}

	// ---- 2. Find the most-recent nearby point within 20 m. ----------------
	nearby, found := findNearbyPoint(player, pt, NearbyRadius)

	if !found {
		// ---- 3a. No nearby point → start a brand-new polyline. -------------
		newLine := &PlayerLine{
			Points: []TimestampedPoint{{Point: pt, Timestamp: ts}},
		}
		poly := s2.Polyline([]s2.Point{pt})
		newLine.Polyline = &poly
		player.Lines = append(player.Lines, newLine)
		return nil
	}

	// ---- 3b. Nearby point found → join to it via OSRM route. --------------
	lineIdx := nearby.LineIdx
	line := player.Lines[lineIdx]

	oldEdgeCount := 0
	if line.Polyline != nil {
		oldEdgeCount = line.Polyline.NumEdges()
	}

	if err := joinViaRoute(line, pt, ts); err != nil {
		// Fallback: just append the point directly (straight-line join).
		line.Points = append(line.Points, TimestampedPoint{Point: pt, Timestamp: ts})
		line.rebuildPolyline()
	}

	// ---- 4. Check for self-intersection (creates a polygon). ---------------
	if line.Polyline == nil || oldEdgeCount < 1 {
		return nil
	}

	selfHit, crossPt, oldEI, newEI := findSelfIntersection(line.Polyline, oldEdgeCount)
	if !selfHit {
		return nil
	}

	// ---- 5. Create polygon from the loop portion. --------------------------
	rawPoly := createPolygonFromSelfIntersection(line.Polyline, crossPt, oldEI, newEI)
	if rawPoly == nil {
		return nil
	}

	// Subtract already-claimed areas.
	allClaimed := collectAllClaimed(gs)
	finalPoly := subtractClaimed(rawPoly, allClaimed)
	if finalPoly == nil {
		return nil
	}

	// ---- 6. Replace the line with its leftover tails. ----------------------
	tails := splitLineAfterPolygon(line, crossPt, oldEI, newEI, ts)

	// Swap the original line out for the tails.
	var newLines []*PlayerLine
	for i, l := range player.Lines {
		if i == lineIdx {
			newLines = append(newLines, tails...)
		} else {
			newLines = append(newLines, l)
		}
	}
	player.Lines = newLines

	// Record the claimed polygon.
	player.Claimed = append(player.Claimed, finalPoly)

	// ---- 7. Check if new shape intersects opponent lines. ------------------
	for _, opp := range gs.Players {
		if opp.ID == playerID {
			continue
		}
		if opp.Team == player.Team {
			continue // teammates are safe
		}
		hits := findIntersectingOpponentLines(finalPoly, opp)
		if len(hits) > 0 {
			destroyConnectedLines(opp, hits)
		}
	}

	return nil
}

// collectAllClaimed gathers every claimed polygon from every player.
func collectAllClaimed(gs *GameState) []*s2.Polygon {
	var all []*s2.Polygon
	for _, p := range gs.Players {
		all = append(all, p.Claimed...)
	}
	return all
}

// ---------------------------------------------------------------------------
// HTTP handlers
// ---------------------------------------------------------------------------

func (gs *GameState) PingEndpoint(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	playerID := q.Get("player_id")
	if playerID == "" {
		http.Error(w, "missing player_id", http.StatusBadRequest)
		return
	}

	latStr := q.Get("lat")
	lngStr := q.Get("lng")
	var lat, lng float64
	if _, err := fmt.Sscanf(latStr, "%f", &lat); err != nil {
		http.Error(w, "invalid lat", http.StatusBadRequest)
		return
	}
	if _, err := fmt.Sscanf(lngStr, "%f", &lng); err != nil {
		http.Error(w, "invalid lng", http.StatusBadRequest)
		return
	}

	// Use the Date header if present, otherwise fall back to server time.
	ts := time.Now()
	if dateStr := r.Header.Get("Date"); dateStr != "" {
		if parsed, err := time.Parse(time.RFC1123, dateStr); err == nil {
			ts = parsed
		}
	}

	if err := gs.ReceivePing(playerID, lat, lng, ts); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

func (gs *GameState) StateEndpoint(w http.ResponseWriter, r *http.Request) {
	gs.SendState(w)
}

// ---------------------------------------------------------------------------
// Bootstrap
// ---------------------------------------------------------------------------

func main() {
	gs := NewGameState([]string{"red", "blue"})

	// Seed some players for testing.
	gs.Players["alice"] = &Player{ID: "alice", Team: "red", City: "New York"}
	gs.Players["bob"] = &Player{ID: "bob", Team: "blue", City: "New York"}

	http.HandleFunc("/ping", gs.PingEndpoint)
	http.HandleFunc("/state", gs.StateEndpoint)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
