package osm

import (
	"testing"
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
