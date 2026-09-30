// Package osm provides nearby-POI lookups (e.g. public toilets) for parkrun
// events, based on a local OSM PBF extract (see pbfpoi.go).
package osm

import (
	"errors"
	"math"
	"strings"

	"github.com/flopp/parkrun-map/internal/utils"
)

const (
	metersPerDegree  = 111320.0
	maxPlacesPerType = 5
)

type Place struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Name string  `json:"name"`
	URL  string  `json:"url"`
}

type Nearby struct {
	Toilets []Place
}

type coordinates struct {
	Lat float64
	Lon float64
}

type boundingBox struct {
	South float64
	West  float64
	North float64
	East  float64
}

func boundingBoxForTracksWithBuffer(tracks [][]utils.Coordinates, bufferMeters float64) (boundingBox, error) {
	box := boundingBox{South: 90, West: 180, North: -90, East: -180}
	maxAbsLatitude := 0.0
	found := false
	for _, track := range tracks {
		for _, point := range track {
			if math.IsNaN(point.Lat) || math.IsInf(point.Lat, 0) || math.IsNaN(point.Lon) || math.IsInf(point.Lon, 0) || point.Lat < -90 || point.Lat > 90 || point.Lon < -180 || point.Lon > 180 {
				continue
			}
			found = true
			box.South = math.Min(box.South, point.Lat)
			box.West = math.Min(box.West, point.Lon)
			box.North = math.Max(box.North, point.Lat)
			box.East = math.Max(box.East, point.Lon)
			maxAbsLatitude = math.Max(maxAbsLatitude, math.Abs(point.Lat))
		}
	}
	if !found {
		return boundingBox{}, errors.New("route has no valid track coordinates")
	}

	latitudeBuffer := bufferMeters / metersPerDegree
	box.South = math.Max(-90, box.South-latitudeBuffer)
	box.North = math.Min(90, box.North+latitudeBuffer)
	longitudeScale := math.Cos(maxAbsLatitude * math.Pi / 180)
	if longitudeScale < 1e-6 {
		box.West = -180
		box.East = 180
	} else {
		longitudeBuffer := bufferMeters / (metersPerDegree * longitudeScale)
		box.West = math.Max(-180, box.West-longitudeBuffer)
		box.East = math.Min(180, box.East+longitudeBuffer)
	}
	return box, nil
}

func (box boundingBox) contains(point coordinates) bool {
	return point.Lat >= box.South && point.Lat <= box.North && point.Lon >= box.West && point.Lon <= box.East
}

func restrictedAccess(access string) bool {
	return access == "private" || access == "no" || access == "customers"
}

func placeName(tags map[string]string) string {
	if name := strings.TrimSpace(tags["name"]); name != "" {
		return name
	}
	return "Toilette"
}
