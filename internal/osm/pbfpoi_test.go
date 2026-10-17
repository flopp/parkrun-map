package osm

import (
	"testing"

	"github.com/flopp/parkrun-map/internal/utils"
)

func TestNearestSortsAndCaps(t *testing.T) {
	candidates := []placeCandidate{
		{place: Place{Name: "b"}, distance: 50},
		{place: Place{Name: "a"}, distance: 50},
		{place: Place{Name: "c"}, distance: 10},
		{place: Place{Name: "d"}, distance: 30},
		{place: Place{Name: "e"}, distance: 40},
		{place: Place{Name: "f"}, distance: 20},
	}
	got := nearest(candidates)
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

func TestNearbyPlacesIncludesNearbyToiletWay(t *testing.T) {
	index := &PBFIndex{pois: []pbfPOI{{
		id:    42,
		lat:   0,
		lon:   0,
		tags:  map[string]string{"amenity": "toilets", "name": "Park toilets"},
		isWay: true,
	}}}
	track := [][]utils.Coordinates{{{Lat: -0.001, Lon: 0}, {Lat: 0.001, Lon: 0}}}
	bounds := boundingBox{South: -0.002, West: -0.002, North: 0.002, East: 0.002}

	got := index.NearbyPlaces(utils.Coordinates{}, bounds, track, 100)
	if len(got) != 1 {
		t.Fatalf("NearbyPlaces() returned %d places, want 1", len(got))
	}
	if got[0].Name != "Park toilets" {
		t.Errorf("NearbyPlaces()[0].Name = %q, want %q", got[0].Name, "Park toilets")
	}
	if got[0].URL != "https://www.openstreetmap.org/way/42" {
		t.Errorf("NearbyPlaces()[0].URL = %q, want a way URL", got[0].URL)
	}
}
