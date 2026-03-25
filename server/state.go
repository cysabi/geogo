package main

import (
	"encoding/json"

	"github.com/twpayne/go-geos"
)

type Game struct {
	Colors  []string  `json:"colors"`
	Players []*Player `json:"players"`
}

type Player struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Team        string      `json:"team"`
	City        string      `json:"city"`
	LatestPoint *[2]float64 `json:"lastPoint"`
	Trail       *geos.Geom  `json:"-"` // MultiPolygon — buffered road segments
	Claimed     *geos.Geom  `json:"-"` // MultiPolygon — enclosed territory
}

func (p *Player) MarshalJSON() ([]byte, error) {
	type Alias Player
	return json.Marshal(&struct {
		*Alias
		Trail   [][][][2]float64 `json:"trail"`
		Claimed [][][][2]float64 `json:"claimed"`
	}{
		Alias:   (*Alias)(p),
		Trail:   multiPolygonCoords(p.Trail),
		Claimed: multiPolygonCoords(p.Claimed),
	})
}

func multiPolygonCoords(geom *geos.Geom) [][][][2]float64 {
	if geom == nil {
		return nil
	}
	var result [][][][2]float64
	for i := range geom.NumGeometries() {
		poly := geom.Geometry(i)
		rings := [][][2]float64{fromGeosCoords(poly.ExteriorRing().CoordSeq().ToCoords())}
		for j := range poly.NumInteriorRings() {
			rings = append(rings, fromGeosCoords(poly.InteriorRing(j).CoordSeq().ToCoords()))
		}
		result = append(result, rings)
	}
	return result
}

func toGeosCoords(pts [][2]float64) [][]float64 {
	out := make([][]float64, len(pts))
	for i, p := range pts {
		out[i] = []float64{p[0], p[1]}
	}
	return out
}

func fromGeosCoords(coords [][]float64) [][2]float64 {
	out := make([][2]float64, len(coords))
	for i, c := range coords {
		out[i] = [2]float64{c[0], c[1]}
	}
	return out
}
