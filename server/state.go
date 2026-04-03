package main

import (
	"encoding/json"

	"github.com/twpayne/go-geos"
)

type Game struct {
	Colors  [2]string `json:"colors"`
	Players []*Player `json:"players"`
}

type Player struct {
	Tag         string      `json:"tag"`
	Team        string      `json:"team"`
	City        string      `json:"city"`
	LatestPoint *[2]float64 `json:"lastPoint"`
	LatestTs    *float64    `json:"lastTs"`
	Trail       *geos.Geom  `json:"-"` // MultiLineString — raw road segments
	Claimed     *geos.Geom  `json:"-"` // MultiPolygon — enclosed territory
}

func (p *Player) New() *Player {
	p.Trail = geos.NewCollection(geos.TypeIDMultiLineString, []*geos.Geom{})
	p.Claimed = geos.NewCollection(geos.TypeIDMultiPolygon, []*geos.Geom{})
	return p
}

func (p *Player) MarshalJSON() ([]byte, error) {
	type P Player
	return json.Marshal(&struct {
		*P
		Trail   json.RawMessage `json:"trail"`
		Claimed json.RawMessage `json:"claimed"`
	}{
		P:       (*P)(p),
		Trail:   json.RawMessage(p.Trail.ToGeoJSON(-1)),
		Claimed: json.RawMessage(p.Claimed.Normalize().Reverse().ToGeoJSON(-1))})
}
