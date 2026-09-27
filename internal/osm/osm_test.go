package osm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flopp/parkrun-map/internal/utils"
)

const testResponse = `{"elements":[
	{"type":"node","id":1,"lat":52.52,"lon":13.405,"tags":{"amenity":"toilets","name":"WC"}},
	{"type":"node","id":2,"lat":52.521,"lon":13.405,"tags":{"amenity":"cafe","name":"Café am Park"}},
	{"type":"node","id":3,"lat":52.522,"lon":13.405,"tags":{"highway":"bus_stop"}},
	{"type":"node","id":4,"lat":52.52,"lon":13.41,"tags":{"amenity":"toilets","access":"private"}}
]}`

var testBounds = boundingBox{South: 52.5, West: 13.4, North: 52.54, East: 13.42}

func marshalTestCache(t *testing.T, bounds boundingBox, data []byte) []byte {
	t.Helper()
	contents, err := json.Marshal(cacheEntry{Version: 1, Bounds: bounds, Response: json.RawMessage(data)})
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func TestLoadNearbyUsesFreshCache(t *testing.T) {
	now := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	cachePath := filepath.Join(t.TempDir(), "event")
	if err := os.WriteFile(cachePath, marshalTestCache(t, testBounds, []byte(testResponse)), 0644); err != nil {
		t.Fatal(err)
	}
	mtime := now.Add(-99 * 24 * time.Hour)
	if err := os.Chtimes(cachePath, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	fetched := false
	nearby, err := loadNearby(context.Background(), "event", cachePath, utils.Coordinates{Lat: 52.52, Lon: 13.405}, testBounds, now, func(context.Context, string, boundingBox) ([]byte, error) {
		fetched = true
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if fetched {
		t.Fatal("fresh cache unexpectedly triggered a fetch")
	}
	if len(nearby.Toilets) != 1 {
		t.Fatalf("unexpected nearby results: %+v", nearby)
	}
}

func TestLoadNearbyRefreshesStaleCache(t *testing.T) {
	now := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	cachePath := filepath.Join(t.TempDir(), "osm", "event")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0770); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, []byte(`{"elements":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	mtime := now.Add(-101 * 24 * time.Hour)
	if err := os.Chtimes(cachePath, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	fetched := false
	nearby, err := loadNearby(context.Background(), "event", cachePath, utils.Coordinates{Lat: 52.52, Lon: 13.405}, testBounds, now, func(context.Context, string, boundingBox) ([]byte, error) {
		fetched = true
		return []byte(testResponse), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !fetched {
		t.Fatal("stale cache did not trigger a fetch")
	}
	if len(nearby.Toilets) != 1 {
		t.Fatalf("unexpected nearby results after refresh: %+v", nearby)
	}
	contents, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	var expectedResponse bytes.Buffer
	if err := json.Compact(&expectedResponse, []byte(testResponse)); err != nil {
		t.Fatal(err)
	}
	entry, ok := decodeCacheEntry(contents)
	if !ok || !bytes.Equal(entry.Response, expectedResponse.Bytes()) || entry.Bounds != testBounds {
		t.Fatal("refreshed OSM response and route bounds were not written to cache")
	}
}

func TestQueryChecksOnlyForToilets(t *testing.T) {
	query := queryForBounds(testBounds)
	if !strings.Contains(query, `["amenity"="toilets"]`) {
		t.Fatalf("query does not search for toilets: %s", query)
	}
	wantBounds := fmt.Sprintf("(%.7f,%.7f,%.7f,%.7f)", testBounds.South, testBounds.West, testBounds.North, testBounds.East)
	if !strings.Contains(query, wantBounds) {
		t.Fatalf("query does not use route bounding box %s: %s", wantBounds, query)
	}
	if !strings.Contains(query, fmt.Sprintf("out center tags %d;", maxPlacesPerType)) {
		t.Fatalf("query does not cap output at %d features: %s", maxPlacesPerType, query)
	}
	for _, tag := range []string{`"cafe"`, `"highway"`, `"public_transport"`, `"railway"`} {
		if strings.Contains(query, tag) {
			t.Errorf("query unexpectedly includes %s: %s", tag, query)
		}
	}
}

func TestBoundingBoxForTracksAdds100mBuffer(t *testing.T) {
	tracks := [][]utils.Coordinates{{
		{Lat: 52.52, Lon: 13.405},
		{Lat: 52.52, Lon: 13.415},
	}}
	box, err := boundingBoxForTracks(tracks)
	if err != nil {
		t.Fatal(err)
	}
	latBuffer := (52.52 - box.South) * metersPerDegree
	lonBuffer := (13.405 - box.West) * metersPerDegree * math.Cos(52.52*math.Pi/180)
	if math.Abs(latBuffer-routeBufferMeters) > 0.01 || math.Abs(lonBuffer-routeBufferMeters) > 0.01 {
		t.Fatalf("route buffers are %.3fm latitude and %.3fm longitude, want 100m", latBuffer, lonBuffer)
	}
}

func TestLoadNearbyRefreshesCacheWhenRouteBoundsChange(t *testing.T) {
	now := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	cachePath := filepath.Join(t.TempDir(), "event")
	if err := os.WriteFile(cachePath, marshalTestCache(t, testBounds, []byte(testResponse)), 0644); err != nil {
		t.Fatal(err)
	}
	mtime := now.Add(-24 * time.Hour)
	if err := os.Chtimes(cachePath, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	newBounds := testBounds
	newBounds.West -= 0.001
	fetched := false
	_, err := loadNearby(context.Background(), "event", cachePath, utils.Coordinates{Lat: 52.52, Lon: 13.405}, newBounds, now, func(context.Context, string, boundingBox) ([]byte, error) {
		fetched = true
		return []byte(testResponse), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !fetched {
		t.Fatal("fresh cache with different route bounds did not trigger a fetch")
	}
}

func TestLoadNearbyRefreshesLegacyPointCache(t *testing.T) {
	now := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	cachePath := filepath.Join(t.TempDir(), "event")
	if err := os.WriteFile(cachePath, []byte(testResponse), 0644); err != nil {
		t.Fatal(err)
	}
	mtime := now.Add(-24 * time.Hour)
	if err := os.Chtimes(cachePath, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	fetched := false
	_, err := loadNearby(context.Background(), "event", cachePath, utils.Coordinates{Lat: 52.52, Lon: 13.405}, testBounds, now, func(context.Context, string, boundingBox) ([]byte, error) {
		fetched = true
		return []byte(testResponse), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !fetched {
		t.Fatal("legacy cache without route bounds did not trigger a fetch")
	}
}

func TestRequestOverpassLogsEmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer server.Close()

	previousOutput := log.Writer()
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(previousOutput)

	_, err := requestOverpass(context.Background(), server.Client(), server.URL, "test-event", "data=query")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "empty result (0 toilet features)") {
		t.Fatalf("expected explicit empty-result log, got: %s", logs.String())
	}
}

func TestOverpassTimeoutClassification(t *testing.T) {
	previousOutput := log.Writer()
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(previousOutput)

	if !isOverpassTimeout(httpStatusError{code: http.StatusGatewayTimeout}) {
		t.Fatal("HTTP 504 should be classified as a timeout")
	}
	logOverpassFailure("test-event", "https://overpass.example", httpStatusError{code: http.StatusGatewayTimeout})
	if !strings.Contains(logs.String(), "OSM Overpass timeout") {
		t.Fatalf("expected timeout log, got: %s", logs.String())
	}
	if !isOverpassTimeout(context.DeadlineExceeded) {
		t.Fatal("context deadline should be classified as a timeout")
	}
	logs.Reset()
	logOverpassFailure("test-event", "https://overpass.example", httpStatusError{code: http.StatusTooManyRequests})
	if strings.Contains(logs.String(), "OSM Overpass timeout") || !strings.Contains(logs.String(), "OSM Overpass request failed") {
		t.Fatalf("HTTP 429 should log as a non-timeout request failure, got: %s", logs.String())
	}
	if isOverpassTimeout(httpStatusError{code: http.StatusTooManyRequests}) {
		t.Fatal("HTTP 429 should not be classified as a timeout")
	}
}

func TestFetchOverpassFallsBackAfterTransientFailures(t *testing.T) {
	for _, statusCode := range []int{http.StatusTooManyRequests, http.StatusGatewayTimeout} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			primaryRequests := 0
			primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				primaryRequests++
				http.Error(w, http.StatusText(statusCode), statusCode)
			}))
			defer primary.Close()

			fallbackRequests := 0
			fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fallbackRequests++
				_, _ = w.Write([]byte(`{"elements":[]}`))
			}))
			defer fallback.Close()

			client := &http.Client{Timeout: time.Second}
			data, err := fetchFromEndpoints(context.Background(), client, []string{primary.URL, fallback.URL}, "test-event", "data=query")
			if err != nil {
				t.Fatal(err)
			}
			if primaryRequests != 1 || fallbackRequests != 1 {
				t.Fatalf("unexpected request counts: primary=%d fallback=%d", primaryRequests, fallbackRequests)
			}
			if string(data) != `{"elements":[]}` {
				t.Fatalf("unexpected fallback response: %s", data)
			}
		})
	}
}

func TestConnectionRefusedDisablesOnlyThatOverpassServer(t *testing.T) {
	overpassThrottle.Lock()
	previousLastRequestEnd := overpassThrottle.lastRequestEnd
	previousDisabled := overpassThrottle.disabledServers
	overpassThrottle.lastRequestEnd = make(map[string]time.Time)
	overpassThrottle.disabledServers = make(map[string]string)
	overpassThrottle.Unlock()
	t.Cleanup(func() {
		overpassThrottle.Lock()
		overpassThrottle.lastRequestEnd = previousLastRequestEnd
		overpassThrottle.disabledServers = previousDisabled
		overpassThrottle.Unlock()
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	refusedEndpoint := "http://" + listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	fallbackRequests := 0
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackRequests++
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer fallback.Close()

	client := &http.Client{Timeout: time.Second}
	data, err := fetchFromEndpoints(context.Background(), client, []string{refusedEndpoint, fallback.URL}, "test-event", "data=query")
	if err != nil {
		t.Fatalf("fallback after connection refusal failed: %v", err)
	}
	if fallbackRequests != 1 || string(data) != `{"elements":[]}` {
		t.Fatalf("fallback response = %q with %d requests, want one successful request", data, fallbackRequests)
	}
	if !isOverpassServerDisabled(refusedEndpoint) {
		t.Fatal("connection-refused server was not marked disabled")
	}

	data, err = fetchFromEndpoints(context.Background(), client, []string{refusedEndpoint, fallback.URL}, "next-event", "data=query")
	if err != nil {
		t.Fatalf("request to another server failed after disabling refused server: %v", err)
	}
	if fallbackRequests != 2 || string(data) != `{"elements":[]}` {
		t.Fatalf("second fallback response = %q with %d requests, want two successful fallback requests", data, fallbackRequests)
	}
}

func TestRateLimitedOverpassServerIsSkippedLater(t *testing.T) {
	overpassThrottle.Lock()
	previousLastRequestEnd := overpassThrottle.lastRequestEnd
	previousDisabled := overpassThrottle.disabledServers
	overpassThrottle.lastRequestEnd = make(map[string]time.Time)
	overpassThrottle.disabledServers = make(map[string]string)
	overpassThrottle.Unlock()
	t.Cleanup(func() {
		overpassThrottle.Lock()
		overpassThrottle.lastRequestEnd = previousLastRequestEnd
		overpassThrottle.disabledServers = previousDisabled
		overpassThrottle.Unlock()
	})

	primaryRequests := 0
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryRequests++
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer primary.Close()

	fallbackRequests := 0
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackRequests++
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer fallback.Close()

	client := &http.Client{Timeout: time.Second}
	endpoints := []string{primary.URL, fallback.URL}
	for request := 1; request <= 2; request++ {
		data, err := fetchFromEndpoints(context.Background(), client, endpoints, "test-event", "data=query")
		if err != nil {
			t.Fatalf("request %d failed: %v", request, err)
		}
		if string(data) != `{"elements":[]}` {
			t.Fatalf("request %d response = %s, want empty result", request, data)
		}
	}
	if primaryRequests != 1 {
		t.Fatalf("rate-limited server received %d requests, want 1", primaryRequests)
	}
	if fallbackRequests != 2 {
		t.Fatalf("fallback server received %d requests, want 2", fallbackRequests)
	}
	if !isOverpassServerDisabled(primary.URL) {
		t.Fatal("HTTP 429 server was not marked disabled")
	}
}

func TestClientTimeoutServerIsSkippedLater(t *testing.T) {
	overpassThrottle.Lock()
	previousLastRequestEnd := overpassThrottle.lastRequestEnd
	previousDisabled := overpassThrottle.disabledServers
	overpassThrottle.lastRequestEnd = make(map[string]time.Time)
	overpassThrottle.disabledServers = make(map[string]string)
	overpassThrottle.Unlock()
	t.Cleanup(func() {
		overpassThrottle.Lock()
		overpassThrottle.lastRequestEnd = previousLastRequestEnd
		overpassThrottle.disabledServers = previousDisabled
		overpassThrottle.Unlock()
	})

	primaryRequests := 0
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryRequests++
		<-time.After(200 * time.Millisecond)
	}))
	defer primary.Close()

	fallbackRequests := 0
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackRequests++
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer fallback.Close()

	client := &http.Client{Timeout: 50 * time.Millisecond}
	endpoints := []string{primary.URL, fallback.URL}
	for request := 1; request <= 2; request++ {
		data, err := fetchFromEndpoints(context.Background(), client, endpoints, "test-event", "data=query")
		if err != nil {
			t.Fatalf("request %d failed: %v", request, err)
		}
		if string(data) != `{"elements":[]}` {
			t.Fatalf("request %d response = %s, want empty result", request, data)
		}
	}
	if primaryRequests != 1 {
		t.Fatalf("timed-out server received %d requests, want 1", primaryRequests)
	}
	if fallbackRequests != 2 {
		t.Fatalf("fallback server received %d requests, want 2", fallbackRequests)
	}
	if !isOverpassServerDisabled(primary.URL) {
		t.Fatal("client-timeout server was not marked disabled")
	}
}
