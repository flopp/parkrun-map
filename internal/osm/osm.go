package osm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/flopp/parkrun-map/internal/utils"
)

const (
	routeBufferMeters = 100.0
	metersPerDegree   = 111320.0
	cacheMaxAge       = 100 * 24 * time.Hour
	maxPlacesPerType  = 5
	overpassURL       = "https://overpass-api.de/api/interpreter"
)

var overpassDelay = 10 * time.Second

var overpassFallbackURLs = []string{
	"https://overpass.private.coffee/api/interpreter",
}

type Place struct {
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Name     string  `json:"name"`
	Distance int     `json:"distance"`
	URL      string  `json:"url"`
}

type Nearby struct {
	Toilets []Place
}

type coordinates struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type element struct {
	Type   string            `json:"type"`
	ID     int64             `json:"id"`
	Lat    float64           `json:"lat"`
	Lon    float64           `json:"lon"`
	Center *coordinates      `json:"center"`
	Tags   map[string]string `json:"tags"`
}

type response struct {
	Elements []element `json:"elements"`
}

type boundingBox struct {
	South float64 `json:"south"`
	West  float64 `json:"west"`
	North float64 `json:"north"`
	East  float64 `json:"east"`
}

type cacheEntry struct {
	Version  int             `json:"version"`
	Bounds   boundingBox     `json:"bounds"`
	Response json.RawMessage `json:"response"`
}

var overpassThrottle struct {
	sync.Mutex
	lastRequestEnd  map[string]time.Time
	disabledServers map[string]string
}

var errOverpassServerDisabled = errors.New("Overpass server disabled")

func LoadNearby(ctx context.Context, eventID, cacheDir string, coord utils.Coordinates, tracks [][]utils.Coordinates, now time.Time) (Nearby, error) {
	log.Printf("Loading nearby places for eventID: %s", eventID)
	if eventID == "" || filepath.Base(eventID) != eventID {
		return Nearby{}, fmt.Errorf("invalid parkrun ID for OSM cache: %q", eventID)
	}
	bounds, err := boundingBoxForTracks(tracks)
	if err != nil {
		return Nearby{}, fmt.Errorf("determining route bounding box for %s: %w", eventID, err)
	}

	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%s.json", eventID))
	return loadNearby(ctx, eventID, cachePath, coord, bounds, now, fetch)
}

type responseFetcher func(context.Context, string, boundingBox) ([]byte, error)

func loadNearby(ctx context.Context, eventID, cachePath string, coord utils.Coordinates, bounds boundingBox, now time.Time, fetchResponse responseFetcher) (Nearby, error) {
	cachedData, readErr := os.ReadFile(cachePath)
	cachedResponse := cachedData
	cachedBoundsMatch := false
	if readErr == nil {
		if entry, ok := decodeCacheEntry(cachedData); ok {
			cachedResponse = entry.Response
			cachedBoundsMatch = entry.Bounds == bounds
		}
		if info, err := os.Stat(cachePath); err == nil && !info.ModTime().Before(now.Add(-cacheMaxAge)) && cachedBoundsMatch {
			if nearby, err := parseNearby(cachedResponse, coord, bounds); err == nil {
				/*
					if len(nearby.Toilets) == 0 {
						log.Printf("OSM cache hit for %s: empty result (0 public toilets); skipping Overpass request", eventID)
					} else {
						log.Printf("OSM cache hit for %s: %d public toilets; skipping Overpass request", eventID, len(nearby.Toilets))
					}
				*/
				return nearby, nil
			}
		}
	}

	data, err := fetchResponse(ctx, eventID, bounds)
	if err != nil {
		if readErr == nil {
			if nearby, parseErr := parseNearby(cachedResponse, coord, bounds); parseErr == nil {
				return nearby, fmt.Errorf("refreshing OSM cache %s failed; using stale data: %w", cachePath, err)
			}
		}
		return Nearby{}, fmt.Errorf("fetching OSM data for cache %s: %w", cachePath, err)
	}

	nearby, err := parseNearby(data, coord, bounds)
	if err != nil {
		return Nearby{}, fmt.Errorf("parsing OSM response for cache %s: %w", cachePath, err)
	}
	cacheData, err := json.Marshal(cacheEntry{Version: 1, Bounds: bounds, Response: data})
	if err != nil {
		return nearby, fmt.Errorf("encoding OSM cache %s: %w", cachePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0770); err != nil {
		return nearby, fmt.Errorf("creating OSM cache directory: %w", err)
	}
	if err := os.WriteFile(cachePath, cacheData, 0644); err != nil {
		return nearby, fmt.Errorf("writing OSM cache %s: %w", cachePath, err)
	}
	return nearby, nil
}

func decodeCacheEntry(data []byte) (cacheEntry, bool) {
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil || entry.Version != 1 || len(entry.Response) == 0 {
		return cacheEntry{}, false
	}
	return entry, true
}

func fetch(ctx context.Context, eventID string, bounds boundingBox) ([]byte, error) {
	query := queryForBounds(bounds)

	form := url.Values{"data": {query}}
	client := &http.Client{Timeout: 45 * time.Second}
	// no fallbacks (temporarily)
	endpoints := append([]string{overpassURL}, overpassFallbackURLs...)
	//endpoints := append([]string{overpassURL})
	return fetchFromEndpoints(ctx, client, endpoints, eventID, form.Encode())
}

func queryForBounds(bounds boundingBox) string {
	return fmt.Sprintf(`[out:json][timeout:25];
nwr["amenity"="toilets"](%.7f,%.7f,%.7f,%.7f);
out center tags %d;`, bounds.South, bounds.West, bounds.North, bounds.East, maxPlacesPerType)
}

func boundingBoxForTracks(tracks [][]utils.Coordinates) (boundingBox, error) {
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

	latitudeBuffer := routeBufferMeters / metersPerDegree
	box.South = math.Max(-90, box.South-latitudeBuffer)
	box.North = math.Min(90, box.North+latitudeBuffer)
	longitudeScale := math.Cos(maxAbsLatitude * math.Pi / 180)
	if longitudeScale < 1e-6 {
		box.West = -180
		box.East = 180
	} else {
		longitudeBuffer := routeBufferMeters / (metersPerDegree * longitudeScale)
		box.West = math.Max(-180, box.West-longitudeBuffer)
		box.East = math.Min(180, box.East+longitudeBuffer)
	}
	return box, nil
}

func (box boundingBox) contains(point coordinates) bool {
	return point.Lat >= box.South && point.Lat <= box.North && point.Lon >= box.West && point.Lon <= box.East
}

type httpStatusError struct {
	code int
}

func (err httpStatusError) Error() string {
	return fmt.Sprintf("Overpass returned HTTP status %d", err.code)
}

func fetchFromEndpoints(ctx context.Context, client *http.Client, endpoints []string, eventID, encodedForm string) ([]byte, error) {
	for _, endpoint := range endpoints {
		if err := waitForOverpass(ctx, endpoint); err != nil {
			if errors.Is(err, errOverpassServerDisabled) {
				log.Printf("OSM Overpass skipped event=%s endpoint=%s: %v", eventID, endpoint, err)
				continue
			}
			logOverpassFailure(eventID, endpoint, err)
			return nil, err
		}
		data, err := requestOverpass(ctx, client, endpoint, eventID, encodedForm)

		if err == nil {
			storeLastRequestEnd(endpoint)
			return data, nil
		}
		if isRateLimited(err) {
			disableOverpassServer(endpoint, "HTTP 429 rate limited")
			log.Printf("OSM Overpass rate limited event=%s endpoint=%s; disabling this server for this process: %v", eventID, endpoint, err)
			continue
		}
		if isClientTimeoutAwaitingHeaders(err) {
			disableOverpassServer(endpoint, "client timeout while awaiting headers")
			log.Printf("OSM Overpass client timeout event=%s endpoint=%s; disabling this server for this process: %v", eventID, endpoint, err)
			continue
		}
		if isConnectionRefused(err) {
			disableOverpassServer(endpoint, "connection refused")
			log.Printf("OSM Overpass connection refused event=%s endpoint=%s; disabling this server for this process: %v", eventID, endpoint, err)
			continue
		}
		disableOverpassServer(endpoint, "unknown error")
		log.Printf("OSM Overpass unknown error event=%s endpoint=%s; disabling this server for this process: %v", eventID, endpoint, err)
		logOverpassFailure(eventID, endpoint, err)
	}
	return nil, fmt.Errorf("all Overpass instances failed")
}

func requestOverpass(ctx context.Context, client *http.Client, endpoint, eventID, encodedForm string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(encodedForm))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "parkrun-map/1.0 (+https://parkruns.de)")
	form, err := url.ParseQuery(encodedForm)
	if err != nil {
		return nil, fmt.Errorf("parsing Overpass request body: %w", err)
	}
	query := strings.Join(strings.Fields(form.Get("data")), " ")
	log.Printf("OSM Overpass POST event=%s endpoint=%s query=%q", eventID, endpoint, query)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpStatusError{code: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, err
	}
	var parsed response
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON from Overpass: %w", err)
	}
	if len(parsed.Elements) == 0 {
		log.Printf("OSM Overpass result event=%s endpoint=%s: empty result (0 toilet features)", eventID, endpoint)
	} else {
		log.Printf("OSM Overpass result event=%s endpoint=%s: %d toilet features", eventID, endpoint, len(parsed.Elements))
	}
	return data, nil
}

func logOverpassFailure(eventID, endpoint string, err error) {
	if isOverpassTimeout(err) {
		log.Printf("OSM Overpass timeout event=%s endpoint=%s: %v", eventID, endpoint, err)
		return
	}
	log.Printf("OSM Overpass request failed event=%s endpoint=%s: %v", eventID, endpoint, err)
}

func isOverpassTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var statusErr httpStatusError
	if errors.As(err, &statusErr) && statusErr.code == http.StatusGatewayTimeout {
		return true
	}
	var timeoutErr interface{ Timeout() bool }
	return errors.As(err, &timeoutErr) && timeoutErr.Timeout()
}

func isRateLimited(err error) bool {
	var statusErr httpStatusError
	return errors.As(err, &statusErr) && statusErr.code == http.StatusTooManyRequests
}

func isClientTimeoutAwaitingHeaders(err error) bool {
	return strings.Contains(err.Error(), "Client.Timeout exceeded while awaiting headers")
}

func isConnectionRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(strings.ToLower(err.Error()), "connection refused")
}

func disableOverpassServer(endpoint, reason string) {
	overpassThrottle.Lock()
	if overpassThrottle.disabledServers == nil {
		overpassThrottle.disabledServers = make(map[string]string)
	}
	overpassThrottle.disabledServers[endpoint] = reason
	overpassThrottle.Unlock()
}

func isOverpassServerDisabled(endpoint string) bool {
	overpassThrottle.Lock()
	defer overpassThrottle.Unlock()
	_, disabled := overpassThrottle.disabledServers[endpoint]
	return disabled
}

func waitForOverpass(ctx context.Context, endpoint string) error {
	overpassThrottle.Lock()
	defer overpassThrottle.Unlock()
	if reason, disabled := overpassThrottle.disabledServers[endpoint]; disabled {
		return fmt.Errorf("%w: %s", errOverpassServerDisabled, reason)
	}

	if last := overpassThrottle.lastRequestEnd[endpoint]; !last.IsZero() {
		if wait := time.Until(last.Add(overpassDelay)); wait > 0 {
			log.Printf("Waiting for %v before making request to Overpass server %s", wait, endpoint)
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}

func storeLastRequestEnd(endpoint string) {
	overpassThrottle.Lock()
	defer overpassThrottle.Unlock()

	if overpassThrottle.lastRequestEnd == nil {
		overpassThrottle.lastRequestEnd = make(map[string]time.Time)
	}
	overpassThrottle.lastRequestEnd[endpoint] = time.Now()
}

func parseNearby(data []byte, origin utils.Coordinates, bounds boundingBox) (Nearby, error) {
	var result response
	if err := json.Unmarshal(data, &result); err != nil {
		return Nearby{}, err
	}

	nearby := Nearby{Toilets: make([]Place, 0)}
	for _, item := range result.Elements {
		point := coordinates{Lat: item.Lat, Lon: item.Lon}
		if item.Center != nil {
			point = *item.Center
		}
		if !bounds.contains(point) {
			continue
		}
		distance := utils.DistanceMeters(origin, utils.Coordinates{Lat: point.Lat, Lon: point.Lon})

		place := Place{
			Lat:      point.Lat,
			Lon:      point.Lon,
			Name:     placeName(item.Tags),
			Distance: int(math.Round(distance)),
			URL:      fmt.Sprintf("https://www.openstreetmap.org/%s/%d", item.Type, item.ID),
		}
		if item.Tags["amenity"] == "toilets" && !restrictedAccess(item.Tags["access"]) {
			nearby.Toilets = append(nearby.Toilets, place)
		}
	}

	nearby.Toilets = nearest(nearby.Toilets)
	return nearby, nil
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

func nearest(places []Place) []Place {
	sort.Slice(places, func(i, j int) bool {
		if places[i].Distance == places[j].Distance {
			return places[i].Name < places[j].Name
		}
		return places[i].Distance < places[j].Distance
	})
	if len(places) > maxPlacesPerType {
		places = places[:maxPlacesPerType]
	}
	return places
}
