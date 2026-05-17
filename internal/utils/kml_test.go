package utils

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseKML_DietenbachFixture(t *testing.T) {
	fixturePath := kmlFixturePath(t, "dietenbach.kml")

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixturePath, err)
	}

	tracks, points, err := ParseKML(data)
	if err != nil {
		t.Fatalf("ParseKML failed: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	if got := len(tracks[0]); got != 68 {
		t.Fatalf("expected 68 coordinates in main track, got %d", got)
	}

	assertLatLonClose(t, "track first", tracks[0][0], 48.001264, 7.806406)
	assertLatLonClose(t, "track last", tracks[0][67], 48.001318, 7.806358)

	if len(points) != 4 {
		t.Fatalf("expected 4 named points, got %d", len(points))
	}

	assertPoint(t, points, "Ziel + Treffpunkt", 48.001173, 7.80651)
	assertPoint(t, points, "Start", 48.0015924, 7.8060492)
	assertPoint(t, points, "Parkplatz", 48.001195, 7.809355)
	assertPoint(t, points, "Haltestelle \"Rohrgraben\"", 47.999385, 7.810232)
}

func TestParseKML_GeorgengartenFixture(t *testing.T) {
	fixturePath := kmlFixturePath(t, "georgengarten.kml")

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixturePath, err)
	}

	tracks, points, err := ParseKML(data)
	if err != nil {
		t.Fatalf("ParseKML failed: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	if got := len(tracks[0]); got != 117 {
		t.Fatalf("expected 117 coordinates in main track, got %d", got)
	}

	assertLatLonClose(t, "track first", tracks[0][0], 52.39026, 9.70281)
	assertLatLonClose(t, "track last", tracks[0][116], 52.38911, 9.70461)

	// The source contains 7 point placemarks, but two share the same name.
	if len(points) != 6 {
		t.Fatalf("expected 6 unique named points, got %d", len(points))
	}

	assertPoint(t, points, "Parkplatz", 52.390182, 9.704029)
	assertPoint(t, points, "Toiletten", 52.390899, 9.702519)
	assertPoint(t, points, "Café Steinecke", 52.390723, 9.706321)
	assertPoint(t, points, "Finish", 52.38911, 9.70461)
	assertPoint(t, points, "Start", 52.39026, 9.70281)

	// Later placemark with identical name wins in map assignment.
	assertPoint(t, points, "Straßenbahn Haltestelle", 52.389909, 9.706501)
}

func TestParseKML_HasenheideFixture(t *testing.T) {
	fixturePath := kmlFixturePath(t, "hasenheide.kml")

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixturePath, err)
	}

	tracks, points, err := ParseKML(data)
	if err != nil {
		t.Fatalf("ParseKML failed: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	if got := len(tracks[0]); got != 70 {
		t.Fatalf("expected 70 coordinates in main track, got %d", got)
	}

	assertLatLonClose(t, "track first", tracks[0][0], 52.482166, 13.410809)
	assertLatLonClose(t, "track last", tracks[0][69], 52.481905, 13.420014)

	if len(points) != 3 {
		t.Fatalf("expected 3 named points, got %d", len(points))
	}

	assertPoint(t, points, "Start", 52.482924, 13.416045)
	assertPoint(t, points, "Finish", 52.482885, 13.416302)
	assertPoint(t, points, "car park", 52.487118, 13.42289)
}

func kmlFixturePath(t *testing.T, name string) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test file location")
	}

	return filepath.Join(filepath.Dir(currentFile), "..", "..", "test-data", name)
}

func assertPoint(t *testing.T, points map[string]Coordinates, name string, wantLat float64, wantLon float64) {
	t.Helper()

	got, ok := points[name]
	if !ok {
		t.Fatalf("missing point %q", name)
	}

	assertLatLonClose(t, name, got, wantLat, wantLon)
}

func assertLatLonClose(t *testing.T, label string, got Coordinates, wantLat float64, wantLon float64) {
	t.Helper()

	const epsilon = 1e-7
	if math.Abs(got.Lat-wantLat) > epsilon || math.Abs(got.Lon-wantLon) > epsilon {
		t.Fatalf("%s mismatch: got (%f,%f), want (%f,%f)", label, got.Lat, got.Lon, wantLat, wantLon)
	}
}
