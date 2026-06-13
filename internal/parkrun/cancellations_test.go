package parkrun

import (
	"testing"
	"time"
)

func TestCancellationReasonGerman(t *testing.T) {
	testCases := []struct {
		name        string
		description string
		want        string
	}{
		{name: "AED unavailable", description: "AED unavailable", want: "Defibrillator nicht verfügbar"},
		{name: "Air quality", description: "Air quality", want: "Luftqualität"},
		{name: "Course unsafe or blocked", description: "Course unsafe or blocked", want: "Strecke unsicher oder gesperrt"},
		{name: "Equipment unavailable", description: "Equipment unavailable", want: "Ausrüstung nicht verfügbar"},
		{name: "Incident after start", description: "Incident after start", want: "Vorfall nach dem Start"},
		{name: "Local health restrictions", description: "Local health restrictions", want: "Örtliche Gesundheitsschutzmaßnahmen"},
		{name: "No participants", description: "No participants", want: "Keine Teilnehmer*innen"},
		{name: "Permission issue", description: "Permission issue", want: "Genehmigungsproblem"},
		{name: "Shortage of volunteers", description: "Shortage of volunteers", want: "Zu wenig Helfer*innen"},
		{name: "Venue unavailable", description: "Venue unavailable", want: "Standort nicht verfügbar"},
		{name: "Weather", description: "Weather", want: "Wetter"},
		{name: "unknown falls back", description: "Other reason", want: "Other reason"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			c := Cancellation{Description: tc.description}
			if got := c.ReasonGerman(); got != tc.want {
				t.Fatalf("ReasonGerman() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseCancellationsWiki(t *testing.T) {
	testCases := []struct {
		wikiFile          string
		wantErr           bool
		wantCancellations map[string][]Cancellation
	}{
		{
			wikiFile: "../../test-data/cancellations-germany-2026-05-24.html",
			wantErr:  false,
			wantCancellations: map[string][]Cancellation{
				"Kulturpark Neubrandenburg parkrun": {
					{Date: mustParseDate(t, "2026-05-30"), Description: "Venue unavailable"},
					{Date: mustParseDate(t, "2026-06-13"), Description: "Venue unavailable"},
					{Date: mustParseDate(t, "2026-06-27"), Description: "Venue unavailable"},
				},
				"Fuldaaue parkrun": {
					{Date: mustParseDate(t, "2026-06-06"), Description: "Course unsafe or blocked"},
					{Date: mustParseDate(t, "2026-06-13"), Description: "Course unsafe or blocked"},
					{Date: mustParseDate(t, "2026-06-20"), Description: "Course unsafe or blocked"},
				},
				"Uerdinger Stadtpark parkrun": {
					{Date: mustParseDate(t, "2026-06-13"), Description: "Venue unavailable"},
				},
				"Unisee parkrun": {
					{Date: mustParseDate(t, "2026-06-27"), Description: "Venue unavailable"},
				},
				"Dreiländergarten parkrun": {
					{Date: mustParseDate(t, "2026-07-18"), Description: "Venue unavailable"},
					{Date: mustParseDate(t, "2026-08-15"), Description: "Venue unavailable"},
				},
				"Prestelsee parkrun": {
					{Date: mustParseDate(t, "2026-08-08"), Description: "Venue unavailable"},
				},
				"Kiessee parkrun": {
					{Date: mustParseDate(t, "2026-08-22"), Description: "Venue unavailable"},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.wikiFile, func(t *testing.T) {
			cancellations, err := ParseCancellationsWiki(tc.wikiFile)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseCancellationsWiki() error = %v, wantErr %v", err, tc.wantErr)
			}

			// check that the expected event names are present in the cancellations map
			for eventName := range tc.wantCancellations {
				if _, found := cancellations[eventName]; !found {
					t.Fatalf("ParseCancellationsWiki() missing event name %s in cancellations map", eventName)
				}
			}

			for eventName, wantEntries := range tc.wantCancellations {
				gotEntries, found := cancellations[eventName]
				if !found {
					t.Fatalf("ParseCancellationsWiki() missing event %s for detailed checks", eventName)
				}

				if len(gotEntries) != len(wantEntries) {
					t.Fatalf("ParseCancellationsWiki() event %s has %d cancellations, want %d", eventName, len(gotEntries), len(wantEntries))
				}

				for i, want := range wantEntries {
					got := gotEntries[i]
					if !got.Date.Equal(want.Date) {
						t.Fatalf("ParseCancellationsWiki() event %s cancellation %d date = %s, want %s", eventName, i, got.Date.Format("2006-01-02"), want.Date.Format("2006-01-02"))
					}
					if got.Description != want.Description {
						t.Fatalf("ParseCancellationsWiki() event %s cancellation %d description = %q, want %q", eventName, i, got.Description, want.Description)
					}
				}
			}
		})
	}
}

func mustParseDate(t *testing.T, value string) time.Time {
	t.Helper()

	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("invalid test date %q: %v", value, err)
	}

	return date
}
