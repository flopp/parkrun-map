package parkrun

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flopp/parkrun-map/internal/osm"
	"github.com/flopp/parkrun-map/internal/utils"
)

func TestRenderJsIncludesToiletCoordinates(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "data.js")
	events := []*Event{{
		Id:       "test-event",
		Name:     "Test parkrun",
		Location: "Test city",
		Coords:   utils.Coordinates{Lat: 52.52, Lon: 13.405},
		OSMNearby: osm.Nearby{Toilets: []osm.Place{{
			Lat:  52.521,
			Lon:  13.406,
			Name: "Public toilet",
			URL:  "https://www.openstreetmap.org/node/1",
		}}},
	}}

	if err := RenderJs(events, filePath); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	start := strings.IndexByte(string(content), '[')
	end := strings.LastIndex(string(content), "];")
	if start < 0 || end <= start {
		t.Fatalf("unexpected generated JavaScript: %s", content)
	}

	var output []struct {
		Toilets []osm.Place `json:"toilets"`
	}
	if err := json.Unmarshal(content[start:end+1], &output); err != nil {
		t.Fatalf("parsing generated event data: %v", err)
	}
	if len(output) != 1 || len(output[0].Toilets) != 1 {
		t.Fatalf("generated toilets = %+v, want one toilet", output)
	}
	toilet := output[0].Toilets[0]
	if toilet.Lat != 52.521 || toilet.Lon != 13.406 {
		t.Fatalf("generated toilet coordinates = (%f, %f), want (52.521, 13.406)", toilet.Lat, toilet.Lon)
	}
}
