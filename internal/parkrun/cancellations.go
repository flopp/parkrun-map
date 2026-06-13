package parkrun

import (
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"
	"time"
)

type Cancellation struct {
	Date        time.Time
	Description string
}

var cancellationReasonGerman = map[string]string{
	"AED unavailable":           "Defibrillator nicht verfügbar",
	"Air quality":               "Luftqualität",
	"Course unsafe or blocked":  "Strecke unsicher oder gesperrt",
	"Equipment unavailable":     "Ausrüstung nicht verfügbar",
	"Incident after start":      "Vorfall nach dem Start",
	"Local health restrictions": "Örtliche Gesundheitsschutzmaßnahmen",
	"No participants":           "Keine Teilnehmer*innen",
	"Permission issue":          "Genehmigungsproblem",
	"Shortage of volunteers":    "Zu wenig Helfer*innen",
	"Venue unavailable":         "Standort nicht verfügbar",
	"Weather":                   "Wetter",
}

func (c Cancellation) DateF() string {
	return c.Date.Format("02.01.2006")
}

// ReasonGerman maps the cancellation description from the wiki (english) to German.
func (c Cancellation) ReasonGerman() string {
	if reason, found := cancellationReasonGerman[c.Description]; found {
		return reason
	}
	return c.Description
}

// ParseCancellationsWiki returns a map of event name to list of cancellations for that event, parsed from the given wiki file.
func ParseCancellationsWiki(cancellationsFilePath string) (map[string][]Cancellation, error) {
	data, err := os.ReadFile(cancellationsFilePath)
	if err != nil {
		return nil, fmt.Errorf("while reading cancellations wiki file: %w", err)
	}
	wikiFileData := string(data)

	cancellations := make(map[string][]Cancellation)

	// Extract table rows with date, event and note cells.
	rowRe := regexp.MustCompile(`(?is)<tr>\s*<td>\s*([0-9]{4}-[0-9]{2}-[0-9]{2})\s*</td>\s*<td>\s*(.*?)\s*</td>\s*<td>\s*.*?\s*</td>\s*<td>\s*(.*?)\s*</td>\s*</tr>`)
	matches := rowRe.FindAllStringSubmatch(wikiFileData, -1)
	for _, match := range matches {
		date, err := time.Parse("2006-01-02", strings.TrimSpace(match[1]))
		if err != nil {
			return nil, fmt.Errorf("while parsing cancellation date %q: %w", match[1], err)
		}

		eventName := cleanupWikiCell(match[2])
		description := cleanupWikiCell(match[3])
		if eventName == "" {
			continue
		}

		cancellations[eventName] = append(cancellations[eventName], Cancellation{
			Date:        date,
			Description: description,
		})
	}

	return cancellations, nil
}

func cleanupWikiCell(cell string) string {
	withoutTags := regexp.MustCompile(`(?is)<[^>]*>`).ReplaceAllString(cell, "")
	decoded := html.UnescapeString(withoutTags)
	return strings.Join(strings.Fields(decoded), " ")
}
