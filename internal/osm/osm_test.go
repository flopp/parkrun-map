package osm

import (
	"math"
	"testing"

	"github.com/flopp/parkrun-map/internal/utils"
)

func TestBoundingBoxForTracksWithBufferAddsBuffer(t *testing.T) {
	tracks := [][]utils.Coordinates{{
		{Lat: 52.52, Lon: 13.405},
		{Lat: 52.52, Lon: 13.415},
	}}
	box, err := boundingBoxForTracksWithBuffer(tracks, 100.0)
	if err != nil {
		t.Fatal(err)
	}
	latBuffer := (52.52 - box.South) * metersPerDegree
	lonBuffer := (13.405 - box.West) * metersPerDegree * math.Cos(52.52*math.Pi/180)
	if math.Abs(latBuffer-100.0) > 0.01 || math.Abs(lonBuffer-100.0) > 0.01 {
		t.Fatalf("route buffers are %.3fm latitude and %.3fm longitude, want 100m", latBuffer, lonBuffer)
	}
}

func TestBoundingBoxForTracksWithBufferRejectsEmptyTracks(t *testing.T) {
	if _, err := boundingBoxForTracksWithBuffer(nil, 100.0); err == nil {
		t.Fatal("expected an error for tracks without valid coordinates")
	}
}

func TestBoundingBoxContains(t *testing.T) {
	box := boundingBox{South: 52.5, West: 13.4, North: 52.54, East: 13.42}
	if !box.contains(coordinates{Lat: 52.52, Lon: 13.405}) {
		t.Fatal("expected point inside bounds to be contained")
	}
	if box.contains(coordinates{Lat: 52.6, Lon: 13.405}) {
		t.Fatal("expected point outside bounds to not be contained")
	}
}

func TestRestrictedAccess(t *testing.T) {
	for _, access := range []string{"private", "no", "customers"} {
		if !restrictedAccess(access) {
			t.Errorf("expected access=%q to be restricted", access)
		}
	}
	for _, access := range []string{"", "yes", "public"} {
		if restrictedAccess(access) {
			t.Errorf("expected access=%q to not be restricted", access)
		}
	}
}

func TestPlaceNameFallsBackWithoutName(t *testing.T) {
	if got := placeName(map[string]string{"name": "WC"}); got != "WC" {
		t.Fatalf("placeName() = %q, want %q", got, "WC")
	}
	if got := placeName(map[string]string{}); got != "Toilette" {
		t.Fatalf("placeName() = %q, want fallback %q", got, "Toilette")
	}
}

func TestNearestSortsAndCaps(t *testing.T) {
	places := []Place{
		{Name: "b", Distance: 50},
		{Name: "a", Distance: 50},
		{Name: "c", Distance: 10},
		{Name: "d", Distance: 30},
		{Name: "e", Distance: 40},
		{Name: "f", Distance: 20},
	}
	got := nearest(places)
	if len(got) != maxPlacesPerType {
		t.Fatalf("nearest() returned %d places, want %d", len(got), maxPlacesPerType)
	}
	wantOrder := []string{"c", "f", "d", "e", "a"}
	for i, name := range wantOrder {
		if got[i].Name != name {
			t.Fatalf("nearest()[%d].Name = %q, want %q", i, got[i].Name, name)
		}
	}
}
