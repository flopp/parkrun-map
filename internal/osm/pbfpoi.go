package osm

import (
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/qedus/osmpbf"

	"github.com/flopp/parkrun-map/internal/utils"
)

// defaultPBFPath is used when no explicit PBF file path is given.
const defaultPBFPath = "germany.osm.pbf"

// pbfBufferMeters is the bounding box buffer used for PBF-based POI lookups.
const pbfBufferMeters = 200.0

// pbfPOI is a single tag-matching point of interest extracted from a PBF file.
type pbfPOI struct {
	id    int64
	lat   float64
	lon   float64
	tags  map[string]string
	isWay bool
}

// PBFIndex holds all POIs matching one tag (e.g. amenity=toilets) that were
// extracted from a local .osm.pbf file.
type PBFIndex struct {
	pois []pbfPOI
}

// TagFilter selects nodes whose tags contain Key=Value.
type TagFilter struct {
	Key   string
	Value string
}

// LoadPBFIndex streams through the given .osm.pbf file once and keeps every
// node matching the filter. If path is empty, defaultPBFPath is used.
func LoadPBFIndex(path string, filter TagFilter) (*PBFIndex, error) {
	indexes, err := LoadPBFIndexes(path, map[string]TagFilter{"default": filter})
	if err != nil {
		return nil, err
	}
	return indexes["default"], nil
}

// LoadPBFIndexes streams through the given .osm.pbf file exactly once and
// builds one PBFIndex per named filter, keyed by the same name. Ways and
// relations are skipped: resolving their geometry would require buffering
// all node coordinates in the file, which is impractical for a
// country-sized extract, and the vast majority of point amenities (e.g.
// public toilets, parking) are mapped as nodes. If path is empty,
// defaultPBFPath is used.
func LoadPBFIndexes(path string, filters map[string]TagFilter) (map[string]*PBFIndex, error) {
	if path == "" {
		path = defaultPBFPath
	}
	start := time.Now()
	log.Printf("PBF: loading %d POI categories from %s", len(filters), path)

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening PBF file %s: %w", path, err)
	}
	defer f.Close()

	decoder := osmpbf.NewDecoder(f)
	decoder.SetBufferSize(osmpbf.MaxBlobSize)
	if err := decoder.Start(runtime.GOMAXPROCS(-1)); err != nil {
		return nil, fmt.Errorf("starting PBF decoder for %s: %w", path, err)
	}

	type namedFilter struct {
		name   string
		filter TagFilter
	}
	namedFilters := make([]namedFilter, 0, len(filters))
	for name, filter := range filters {
		namedFilters = append(namedFilters, namedFilter{name, filter})
	}

	poisByName := make(map[string][]pbfPOI, len(filters))
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
			log.Printf("PBF: scanned %d nodes so far (%s)", scanned, path)
		}
		if len(node.Tags) == 0 {
			// the vast majority of nodes are untagged track geometry; skip them
			// without touching the (rarely populated) tags map at all
			continue
		}
		for _, nf := range namedFilters {
			if node.Tags[nf.filter.Key] != nf.filter.Value {
				continue
			}
			poisByName[nf.name] = append(poisByName[nf.name], pbfPOI{
				id:   node.ID,
				lat:  node.Lat,
				lon:  node.Lon,
				tags: node.Tags,
			})
		}
	}

	indexes := make(map[string]*PBFIndex, len(filters))
	for name, filter := range filters {
		pois := poisByName[name]
		indexes[name] = &PBFIndex{pois: pois}
		log.Printf("PBF: loaded %d %s=%s POIs (%s)", len(pois), filter.Key, filter.Value, name)
	}
	log.Printf("PBF: scanned %d nodes from %s in %s", scanned, path, time.Since(start).Round(time.Millisecond))
	return indexes, nil
}

// NearbyPlaces returns the POIs from the index that fall within bounds and
// are within maxTrackDistanceMeters of any of the given tracks (the bounds
// only cheaply narrow down candidates; the track distance is the real
// filter), sorted by distance from origin and capped at maxPlacesPerType.
func (idx *PBFIndex) NearbyPlaces(origin utils.Coordinates, bounds boundingBox, tracks [][]utils.Coordinates, maxTrackDistanceMeters float64) []Place {
	candidates := make([]placeCandidate, 0)
	for _, poi := range idx.pois {
		point := coordinates{Lat: poi.lat, Lon: poi.lon}
		if !bounds.contains(point) {
			continue
		}

		if restrictedAccess(poi.tags["access"]) {
			continue
		}
		distance := distanceToTracksMeters(point, tracks)

		if distance > maxTrackDistanceMeters {
			continue
		}

		urlType := "node"
		if poi.isWay {
			urlType = "way"
		}

		candidates = append(candidates, placeCandidate{
			place: Place{
				Lat:  poi.lat,
				Lon:  poi.lon,
				Name: placeName(poi.tags),
				URL:  fmt.Sprintf("https://www.openstreetmap.org/%s/%d", urlType, poi.id),
			},
			distance: utils.DistanceMeters(origin, utils.Coordinates{Lat: poi.lat, Lon: poi.lon}),
		})
	}
	return nearest(candidates)
}

// placeCandidate pairs a Place with its distance from the origin, used only
// for sorting/capping; the distance itself is not exposed on Place.
type placeCandidate struct {
	place    Place
	distance float64
}

// nearest sorts candidates by distance (ties broken by name) and returns the
// closest maxPlacesPerType as plain Places.
func nearest(candidates []placeCandidate) []Place {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance == candidates[j].distance {
			return candidates[i].place.Name < candidates[j].place.Name
		}
		return candidates[i].distance < candidates[j].distance
	})
	if len(candidates) > maxPlacesPerType {
		candidates = candidates[:maxPlacesPerType]
	}
	places := make([]Place, len(candidates))
	for i, c := range candidates {
		places[i] = c.place
	}
	return places
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

func distancePointToSegmentMeters(point, a, b coordinates) float64 {
	lonScale := math.Cos(point.Lat * math.Pi / 180)
	ax := (a.Lon - point.Lon) * metersPerDegree * lonScale
	ay := (a.Lat - point.Lat) * metersPerDegree
	bx := (b.Lon - point.Lon) * metersPerDegree * lonScale
	by := (b.Lat - point.Lat) * metersPerDegree
	return distanceToSegment(ax, ay, bx, by)
}

func distanceBetweenSegmentsMeters(a, b, c, d coordinates) float64 {
	originLat := (a.Lat + b.Lat + c.Lat + d.Lat) / 4
	lonScale := math.Cos(originLat * math.Pi / 180)
	toXY := func(point coordinates) (float64, float64) {
		return point.Lon * metersPerDegree * lonScale, point.Lat * metersPerDegree
	}
	ax, ay := toXY(a)
	bx, by := toXY(b)
	cx, cy := toXY(c)
	dx, dy := toXY(d)

	cross := func(x1, y1, x2, y2 float64) float64 {
		return x1*y2 - y1*x2
	}
	abx, aby := bx-ax, by-ay
	cdx, cdy := dx-cx, dy-cy
	det := cross(abx, aby, cdx, cdy)
	if det != 0 {
		t := cross(cx-ax, cy-ay, cdx, cdy) / det
		u := cross(cx-ax, cy-ay, abx, aby) / det
		if t >= 0 && t <= 1 && u >= 0 && u <= 1 {
			return 0
		}
	}
	return math.Min(
		math.Min(distancePointToSegmentMeters(a, c, d), distancePointToSegmentMeters(b, c, d)),
		math.Min(distancePointToSegmentMeters(c, a, b), distancePointToSegmentMeters(d, a, b)),
	)
}

// EventTrack identifies an event's coordinate and track(s) for a PBF-based lookup.
type EventTrack struct {
	EventID string
	Coord   utils.Coordinates
	Tracks  [][]utils.Coordinates
}

// LoadNearbyToiletsFromPBF loads node- and way-mapped "amenity=toilets" from
// the given .osm.pbf file, then returns nearby toilets for every event, using
// a 100m buffer around each event's track instead of querying Overpass.
func LoadNearbyToiletsFromPBF(pbfPath string, events []EventTrack) (map[string]Nearby, error) {
	idx, err := loadToiletPBFIndex(pbfPath)
	if err != nil {
		return nil, err
	}

	log.Printf("PBF: looking up nearby toilets for %d event(s)", len(events))
	totalToilets := 0
	results := make(map[string]Nearby, len(events))
	for _, event := range events {
		fmt.Printf("Processing event: %s\n", event.EventID)
		bounds, err := boundingBoxForTracksWithBuffer(event.Tracks, pbfBufferMeters)
		if err != nil {
			return nil, fmt.Errorf("determining route bounding box for %s: %w", event.EventID, err)
		}
		fmt.Printf("Bounding box for event %s: %+v\n", event.EventID, bounds)
		nearby := Nearby{Toilets: idx.NearbyPlaces(event.Coord, bounds, event.Tracks, pbfBufferMeters)}
		fmt.Printf("Found %d nearby toilets for event %s\n", len(nearby.Toilets), event.EventID)
		totalToilets += len(nearby.Toilets)
		results[event.EventID] = nearby
	}
	log.Printf("PBF: found %d toilet(s) across %d event(s)", totalToilets, len(events))
	return results, nil
}

type taggedWay struct {
	id      int64
	tags    map[string]string
	nodeIDs []int64
}

func loadToiletPBFIndex(path string) (*PBFIndex, error) {
	if path == "" {
		path = defaultPBFPath
	}
	const toiletTagKey, toiletTagValue = "amenity", "toilets"
	index := &PBFIndex{}
	ways := make([]taggedWay, 0)
	nodeIDs := make(map[int64]struct{})
	nodeCount := 0

	if err := scanPBF(path, func(obj interface{}) error {
		switch element := obj.(type) {
		case *osmpbf.Node:
			if element.Tags[toiletTagKey] == toiletTagValue {
				nodeCount++
				index.pois = append(index.pois, pbfPOI{
					id: element.ID, lat: element.Lat, lon: element.Lon, tags: element.Tags,
				})
			}
		case *osmpbf.Way:
			if element.Tags[toiletTagKey] != toiletTagValue {
				return nil
			}
			way := taggedWay{id: element.ID, tags: element.Tags, nodeIDs: element.NodeIDs}
			ways = append(ways, way)
			for _, id := range element.NodeIDs {
				nodeIDs[id] = struct{}{}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	wayCount := 0
	if len(ways) > 0 {
		nodeLocations := make(map[int64]coordinates, len(nodeIDs))
		if err := scanPBF(path, func(obj interface{}) error {
			node, ok := obj.(*osmpbf.Node)
			if !ok {
				return nil
			}
			if _, needed := nodeIDs[node.ID]; needed {
				nodeLocations[node.ID] = coordinates{Lat: node.Lat, Lon: node.Lon}
			}
			return nil
		}); err != nil {
			return nil, err
		}

		for _, way := range ways {
			locations := make([]coordinates, 0, len(way.nodeIDs))
			for _, id := range way.nodeIDs {
				location, ok := nodeLocations[id]
				if !ok {
					locations = nil
					break
				}
				locations = append(locations, location)
			}
			if len(locations) == 0 {
				continue
			}
			lat, lon := 0.0, 0.0
			for _, location := range locations {
				lat += location.Lat
				lon += location.Lon
			}
			index.pois = append(index.pois, pbfPOI{
				id: way.id, lat: lat / float64(len(locations)), lon: lon / float64(len(locations)),
				tags: way.tags, isWay: true,
			})
			wayCount++
		}
	}
	log.Printf("PBF: loaded %d amenity=toilets node(s) and %d way(s) from %s", nodeCount, wayCount, path)
	return index, nil
}

func scanPBF(path string, visit func(interface{}) error) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening PBF file %s: %w", path, err)
	}
	defer f.Close()

	decoder := osmpbf.NewDecoder(f)
	decoder.SetBufferSize(osmpbf.MaxBlobSize)
	if err := decoder.Start(runtime.GOMAXPROCS(-1)); err != nil {
		return fmt.Errorf("starting PBF decoder for %s: %w", path, err)
	}
	for {
		obj, err := decoder.Decode()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("decoding PBF file %s: %w", path, err)
		}
		if err := visit(obj); err != nil {
			return err
		}
	}
}
