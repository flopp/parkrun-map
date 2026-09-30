package osm

import (
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/qedus/osmpbf"

	"github.com/flopp/parkrun-map/internal/utils"
)

// defaultPBFPath is used when no explicit PBF file path is given.
const defaultPBFPath = "germany.osm.pbf"

// pbfBufferMeters is the bounding box buffer used for PBF-based POI lookups.
const pbfBufferMeters = 100.0

// pbfPOI is a single tag-matching point of interest extracted from a PBF file.
type pbfPOI struct {
	id   int64
	lat  float64
	lon  float64
	tags map[string]string
}

// PBFIndex holds all POIs matching one tag (e.g. amenity=toilets) that were
// extracted from a local .osm.pbf file.
type PBFIndex struct {
	pois []pbfPOI
}

// LoadPBFIndex streams through the given .osm.pbf file once and keeps every
// node whose tags contain tagKey=tagValue. Ways and relations are skipped:
// resolving their geometry would require buffering all node coordinates in
// the file, which is impractical for a country-sized extract, and the vast
// majority of point amenities (e.g. public toilets) are mapped as nodes.
// If path is empty, defaultPBFPath is used.
func LoadPBFIndex(path, tagKey, tagValue string) (*PBFIndex, error) {
	if path == "" {
		path = defaultPBFPath
	}
	start := time.Now()
	log.Printf("PBF: loading %s=%s from %s", tagKey, tagValue, path)

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening PBF file %s: %w", path, err)
	}
	defer f.Close()

	decoder := osmpbf.NewDecoder(f)
	if err := decoder.Start(runtime.GOMAXPROCS(-1)); err != nil {
		return nil, fmt.Errorf("starting PBF decoder for %s: %w", path, err)
	}

	var pois []pbfPOI
	scanned := 0
	for {
		obj, err := decoder.Decode()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decoding PBF file %s: %w", path, err)
		}
		node, ok := obj.(*osmpbf.Node)
		if !ok {
			continue
		}
		scanned++
		if scanned%5_000_000 == 0 {
			log.Printf("PBF: scanned %d nodes, %d matches so far (%s)", scanned, len(pois), path)
		}
		if node.Tags[tagKey] != tagValue {
			continue
		}
		pois = append(pois, pbfPOI{
			id:   node.ID,
			lat:  node.Lat,
			lon:  node.Lon,
			tags: node.Tags,
		})
	}

	log.Printf("PBF: loaded %d %s=%s POIs from %d nodes in %s (%s)", len(pois), tagKey, tagValue, scanned, time.Since(start).Round(time.Millisecond), path)
	return &PBFIndex{pois: pois}, nil
}

// NearbyPlaces returns the POIs from the index that fall within bounds and
// are within maxTrackDistanceMeters of any of the given tracks (the bounds
// only cheaply narrow down candidates; the track distance is the real
// filter), sorted by distance from origin and capped at maxPlacesPerType.
func (idx *PBFIndex) NearbyPlaces(origin utils.Coordinates, bounds boundingBox, tracks [][]utils.Coordinates, maxTrackDistanceMeters float64) []Place {
	places := make([]Place, 0)
	for _, poi := range idx.pois {
		point := coordinates{Lat: poi.lat, Lon: poi.lon}
		if !bounds.contains(point) {
			continue
		}
		if restrictedAccess(poi.tags["access"]) {
			continue
		}
		if distanceToTracksMeters(point, tracks) > maxTrackDistanceMeters {
			continue
		}
		distance := utils.DistanceMeters(origin, utils.Coordinates{Lat: poi.lat, Lon: poi.lon})
		places = append(places, Place{
			Lat:      poi.lat,
			Lon:      poi.lon,
			Name:     placeName(poi.tags),
			Distance: int(math.Round(distance)),
			URL:      fmt.Sprintf("https://www.openstreetmap.org/node/%d", poi.id),
		})
	}
	return nearest(places)
}

// distanceToTracksMeters returns the shortest distance from point to any
// segment of any track, using a local planar approximation (accurate enough
// for the short, sub-kilometer distances involved here).
func distanceToTracksMeters(point coordinates, tracks [][]utils.Coordinates) float64 {
	lonScale := math.Cos(point.Lat * math.Pi / 180)
	toXY := func(c utils.Coordinates) (float64, float64) {
		return (c.Lon - point.Lon) * metersPerDegree * lonScale, (c.Lat - point.Lat) * metersPerDegree
	}

	minDistance := math.Inf(1)
	for _, track := range tracks {
		if len(track) == 1 {
			x, y := toXY(track[0])
			minDistance = math.Min(minDistance, math.Hypot(x, y))
			continue
		}
		for i := 0; i+1 < len(track); i++ {
			ax, ay := toXY(track[i])
			bx, by := toXY(track[i+1])
			minDistance = math.Min(minDistance, distanceToSegment(ax, ay, bx, by))
		}
	}
	return minDistance
}

// distanceToSegment returns the distance from the origin (0,0) to the
// segment a-b in a local planar coordinate system.
func distanceToSegment(ax, ay, bx, by float64) float64 {
	dx := bx - ax
	dy := by - ay
	lengthSq := dx*dx + dy*dy
	if lengthSq == 0 {
		return math.Hypot(ax, ay)
	}
	t := (-ax*dx - ay*dy) / lengthSq
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return math.Hypot(ax+t*dx, ay+t*dy)
}

// EventTrack identifies an event's coordinate and track(s) for a PBF-based lookup.
type EventTrack struct {
	EventID string
	Coord   utils.Coordinates
	Tracks  [][]utils.Coordinates
}

// LoadNearbyToiletsFromPBF loads the "amenity=toilets" index from the given
// .osm.pbf file once, then returns the nearby toilets for every event, using
// a 500m buffer around each event's track instead of querying Overpass.
func LoadNearbyToiletsFromPBF(pbfPath string, events []EventTrack) (map[string]Nearby, error) {
	idx, err := LoadPBFIndex(pbfPath, "amenity", "toilets")
	if err != nil {
		return nil, err
	}

	log.Printf("PBF: looking up nearby toilets for %d event(s)", len(events))
	totalToilets := 0
	results := make(map[string]Nearby, len(events))
	for _, event := range events {
		bounds, err := boundingBoxForTracksWithBuffer(event.Tracks, pbfBufferMeters)
		if err != nil {
			return nil, fmt.Errorf("determining route bounding box for %s: %w", event.EventID, err)
		}
		nearby := Nearby{Toilets: idx.NearbyPlaces(event.Coord, bounds, event.Tracks, pbfBufferMeters)}
		totalToilets += len(nearby.Toilets)
		results[event.EventID] = nearby
	}
	log.Printf("PBF: found %d toilet(s) across %d event(s)", totalToilets, len(events))
	return results, nil
}
