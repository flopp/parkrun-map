package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"

	"encoding/json"

	"github.com/flopp/go-googlesheetswrapper"
	"github.com/flopp/parkrun-map/internal/parkrun"
	"github.com/flopp/parkrun-map/internal/utils"
)

type RenderData struct {
	Config         *Config
	Event          *parkrun.Event
	Events         []*parkrun.Event
	ActiveEvents   int
	PlannedEvents  int
	ArchivedEvents int
	JsFiles        []string
	CssFiles       []string
	Title          string
	Description    string
	Canonical      string
	Nav            string
	Timestamp      string
	CanonicalUrls  []string
}

func (data *RenderData) set(title, description, canonical, nav string) {
	data.Title = title
	data.Description = description
	data.Canonical = canonical
	data.Nav = nav
}

func (data *RenderData) render(outputFile string, templateFiles ...string) error {
	// fmt.Printf("-- rendering to %s\n", outputFile)

	tmpl, err := template.ParseFiles(templateFiles...)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0770); err != nil {
		return err
	}

	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	if err = tmpl.Execute(f, data); err != nil {
		return err
	}

	data.CanonicalUrls = append(data.CanonicalUrls, data.Canonical)
	return nil
}

func (data RenderData) writeSitemap(filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, url := range data.CanonicalUrls {
		if _, err = f.WriteString(url); err != nil {
			return err
		}
		if _, err = f.WriteString("\n"); err != nil {
			return err
		}
	}
	return nil
}

type PathBuilder string

func (p PathBuilder) Path(items ...string) string {
	joined := string(p)
	for _, item := range items {
		joined = fmt.Sprintf("%s/%s", joined, item)
	}
	return joined
}

func randomDuration(min, max time.Duration) time.Duration {
	delta := max - min
	return min + time.Duration(rand.Int63n(int64(delta+1)))
}

func mustCreateIndexNow(indexnow string, outputDir string) {
	indexNowFile := filepath.Join(outputDir, fmt.Sprintf("%s.txt", indexnow))
	if err := os.WriteFile(indexNowFile, []byte(indexnow), 0644); err != nil {
		panic(fmt.Errorf("while writing indexnow file %s: %w", indexNowFile, err))
	}
}

type GoogleConfig struct {
	ApiKey   string `json:"ApiKey"`
	SheetsId string `json:"SheetsId"`
}

type Config struct {
	Domain         string       `json:"domain"`
	IndexNow       string       `json:"indexnow"`
	Google         GoogleConfig `json:"google"`
	UmamiWebsiteID string       `json:"UmamiWebsiteID"`
}

func unmarshalConfig(content []byte, config *Config) error {
	if err := json.Unmarshal(content, config); err != nil {
		return fmt.Errorf("while unmarshalling config: %w", err)
	}
	return nil
}

func val(cols map[string]int, row []string, name string) string {
	idx, found := cols[name]
	if !found {
		panic(fmt.Sprintf("column %s not found", name))
	}
	if idx >= len(row) {
		return ""
	}
	return row[idx]
}

func loadGoopgleSheetsData(apiKey, sheetsId string) (map[string]*parkrun.ParkrunInfo, error) {
	ctx := context.Background()
	client, err := googlesheetswrapper.New(apiKey, sheetsId)
	if err != nil {
		return nil, fmt.Errorf("creating sheets client: %w", err)
	}
	allSheets, err := client.ReadAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading all sheets: %w", err)
	}

	sheet, found := allSheets["data"]
	if !found {
		return nil, fmt.Errorf("sheet 'data' not found")
	}

	// parse & validate columns
	columns, err := googlesheetswrapper.ExtractHeader(sheet, []string{"id", "name", "city", "location", "status", "first", "coordinates", "route_type", "google_route_id", "google_maps_url", "link1", "link2", "link3", "link4", "link5"}, false)
	if err != nil {
		return nil, fmt.Errorf("extracting header: %w", err)
	}

	parkrunInfos := make(map[string]*parkrun.ParkrunInfo)
	for i, row := range sheet[1:] {
		id := val(columns, row, "id")
		name := val(columns, row, "name")
		city := val(columns, row, "city")
		location := val(columns, row, "location")
		routeType := val(columns, row, "route_type")
		routeId := val(columns, row, "google_route_id")
		googleMaps := val(columns, row, "google_maps_url")
		first := val(columns, row, "first")
		status := val(columns, row, "status")
		coordinates := val(columns, row, "coordinates")

		links := make([]parkrun.Link, 0)
		for j := 1; j <= 5; j++ {
			linkStr := val(columns, row, fmt.Sprintf("link%d", j))
			log.Printf("row %d link%d: %s", i+2, j, linkStr)
			if linkStr != "" {
				link, err := parkrun.ParseLink(linkStr)
				if err != nil {
					return nil, fmt.Errorf("parsing link in row %d: %w", i+2, err)
				}
				links = append(links, link)
			}
		}
		log.Printf("links: %d\n", len(links))

		parkrunInfos[id] = &parkrun.ParkrunInfo{
			Id:          id,
			Name:        name,
			City:        city,
			Location:    location,
			RouteType:   routeType,
			RouteID:     routeId,
			GoogleMaps:  googleMaps,
			First:       first,
			Status:      status,
			Coordinates: coordinates,
			Links:       links,
		}
	}

	return parkrunInfos, nil
}

func main() {
	dataDir := flag.String("data", "data", "the data directory")
	downloadDir := flag.String("download", ".download", "the download directory")
	outputDir := flag.String("output", ".output", "the output directory")
	configFile := flag.String("config", "config.json", "the config file with Google API key and Sheets ID")
	exportCsvFile := flag.String("export-csv", "", "export CSV file with all parkrun events")
	verbose := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()

	if !*verbose {
		log.SetOutput(io.Discard)
	}

	now := time.Now()
	//fileAge30min := now.Add(30 * time.Minute)
	fileAge1d := now.Add(-24 * time.Hour)
	fileAge1w := now.Add(-24 * 7 * time.Hour)

	// Saturday, October 3rd, January 1st (German special days)
	isSaturday := now.Weekday() == time.Saturday
	isOctober3rd := now.Day() == 3 && now.Month() == time.October
	isJanuary1st := now.Day() == 1 && now.Month() == time.January
	isParkrunDay := (isSaturday || isOctober3rd || isJanuary1st) && now.Hour() >= 10

	utils.SetDownloadDelay(2 * time.Second)

	data := PathBuilder(*dataDir)
	download := PathBuilder(*downloadDir)
	output := PathBuilder(*outputDir)

	// fetch data from Google Sheets
	var parkrun_infos map[string]*parkrun.ParkrunInfo
	var config Config
	if configContent, err := os.ReadFile(*configFile); err != nil {
		panic(fmt.Errorf("while reading config file %s: %v", *configFile, err))
	} else if err := unmarshalConfig(configContent, &config); err != nil {
		panic(fmt.Errorf("while parsing config file %s: %v", *configFile, err))
	} else if googleInfos, err := loadGoopgleSheetsData(config.Google.ApiKey, config.Google.SheetsId); err != nil {
		panic(fmt.Errorf("while loading Google Sheets data: %v", err))
	} else {
		parkrun_infos = googleInfos
	}

	// fetch parkrun events
	events_json_url := "https://images.parkrun.com/events.json"
	events_json_file := download.Path("parkrun", "events.json.gz")
	if err := utils.DownloadFileIfOlder(events_json_url, events_json_file, fileAge1d); err != nil {
		panic(fmt.Errorf("while downloading %s to %s: %w", events_json_url, events_json_file, err))
	}

	// parse parkrun events (only returns German events!)
	events, err := parkrun.LoadEvents(events_json_file, parkrun_infos, true /* germanOnly */)
	if err != nil {
		panic(fmt.Errorf("loading parkrun events: %w", err))
	}

	latestDate := time.Time{}
	dates := make(map[*parkrun.Event]time.Time)
	for _, event := range events {
		if !event.Archived() {
			wiki_file := download.Path("parkrun", event.Id, "wiki")
			if utils.FileExists(wiki_file) {
				if err := event.LoadWiki(wiki_file); err == nil && event.LatestRun != nil {
					if event.LatestRun.Date.After(latestDate) {
						latestDate = event.LatestRun.Date
					}
					dates[event] = event.LatestRun.Date
					// force download if there's something wrong with the numbers
					if event.LatestRun.RunnerCount == 0 {
						dates[event] = time.Time{}
					}
				} else if err != nil {
					if err := os.Remove(wiki_file); err != nil {
						panic(err)
					}
				}
			}
		}
	}
	log.Printf("lastest existing date: %v", latestDate)

	// Pull latest results, force update for all events that are definitely outdated
	for _, event := range events {
		isOutdated := false
		if !event.Archived() {
			if date, found := dates[event]; found && (latestDate.After(date) || (isParkrunDay && date.Format("2006-01-02") != now.Format("2006-01-02"))) {
				log.Printf("%s: outdated! date=%v latestafter=%v parkrunday=%v notatparkrunday=%v", event.Id, date, latestDate.After(date), isParkrunDay, date.Format("2006-01-02") != now.Format("2006-01-02"))
				isOutdated = true
			}
			if event.Planned() && isParkrunDay {
				log.Printf("%s: outdated! planned & parkrunday", event.Id)
				isOutdated = true
			}
		}
		if !event.Active() {
			continue
		}
		wiki_url := event.WikiUrl()
		wiki_file := download.Path("parkrun", event.Id, "wiki")
		if isOutdated {
			utils.MustDownloadFile(wiki_url, wiki_file)
		} else {
			utils.MustDownloadFileIfOlder(wiki_url, wiki_file, fileAge1d)
		}
		if err := event.LoadWiki(wiki_file); err != nil {
			log.Printf("while parsing %s: %v", wiki_file, err)
		} else if event.LatestRun != nil && event.LatestRun.Date.After(latestDate) {
			latestDate = event.LatestRun.Date
		}
	}

	for _, event := range events {
		event.Current = !event.Archived() && event.LatestRun != nil && event.LatestRun.Date.Equal(latestDate)
	}
	// Determine order
	orderedEvents := make([]*parkrun.Event, 0, len(events))
	for _, event := range events {
		event.Order = 0
		if event.Current {
			orderedEvents = append(orderedEvents, event)
		}
	}
	sort.Slice(orderedEvents, func(i, j int) bool {
		return orderedEvents[i].LatestRun.RunnerCount > orderedEvents[j].LatestRun.RunnerCount
	})
	order := 0
	orderStep := 1
	last := 0
	for _, event := range orderedEvents {
		if event.LatestRun.RunnerCount != last {
			order += orderStep
			last = event.LatestRun.RunnerCount
			orderStep = 0
		}
		orderStep += 1
		event.Order = order
	}

	for _, event := range events {
		kml_url := fmt.Sprintf("https://www.google.com/maps/d/kml?mid=%s&forcekml=1", event.GoogleMapsId)
		kml_file := download.Path("parkrun", event.Id, "kml")
		utils.MustDownloadFileIfOlder(kml_url, kml_file, now.Add(randomDuration(-24*200*time.Hour, -24*100*time.Hour)))

		if err := event.LoadKML(kml_file); err != nil {
			panic(fmt.Errorf("file parsing %s: %w", kml_file, err))
		}
	}

	// fetch external assets (bulma, leaflet)

	// renovate: datasource=npm depName=leaflet
	leaflet_version := "1.9.4"

	leaflet_url := PathBuilder(fmt.Sprintf("https://unpkg.com/leaflet@%s", leaflet_version))
	picocss_url := PathBuilder("https://cdn.jsdelivr.net/npm/@picocss/pico@2/css")

	// download leaflet
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/leaflet.js"), download.Path("leaflet", "leaflet.js"), fileAge1w)
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/leaflet.css"), download.Path("leaflet", "leaflet.css"), fileAge1w)
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/images/marker-icon.png"), download.Path("leaflet", "marker-icon.png"), fileAge1w)
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/images/marker-icon-2x.png"), download.Path("leaflet", "marker-icon-2x.png"), fileAge1w)
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/images/marker-shadow.png"), download.Path("leaflet", "marker-shadow.png"), fileAge1w)

	// download picocss
	utils.MustDownloadFileIfOlder(picocss_url.Path("pico.min.css"), download.Path("picocss", "pico.css"), fileAge1w)

	// render data
	if err := parkrun.RenderJs(events, download.Path("data.js")); err != nil {
		panic(fmt.Errorf("failed to render data: %v", err))
	}

	js_files := make([]string, 0)
	js_files = append(js_files, utils.MustCopyHash(download.Path("data.js"), "data-HASH.js", *outputDir))
	js_files = append(js_files, utils.MustCopyHash(download.Path("leaflet/leaflet.js"), "leaflet-HASH.js", *outputDir))
	js_files = append(js_files, utils.MustCopyHash(data.Path("static", "main.js"), "main-HASH.js", *outputDir))
	css_files := make([]string, 0)
	css_files = append(css_files, utils.MustCopyHash(download.Path("picocss/pico.css"), "pico-HASH.css", *outputDir))
	css_files = append(css_files, utils.MustCopyHash(download.Path("leaflet/leaflet.css"), "leaflet-HASH.css", *outputDir))
	css_files = append(css_files, utils.MustCopyHash(data.Path("static", "style.css"), "style-HASH.css", *outputDir))

	utils.MustCopyHash(download.Path("leaflet/marker-icon.png"), "images/marker-icon.png", *outputDir)
	utils.MustCopyHash(download.Path("leaflet/marker-icon-2x.png"), "images/marker-icon-2x.png", *outputDir)
	utils.MustCopyHash(download.Path("leaflet/marker-shadow.png"), "images/marker-shadow.png", *outputDir)
	for _, color := range []string{"red", "green", "grey"} {
		utils.MustCopyHash(data.Path(fmt.Sprintf("static/marker-%s-icon.png", color)), fmt.Sprintf("images/marker-%s-icon.png", color), *outputDir)
		utils.MustCopyHash(data.Path(fmt.Sprintf("static/marker-%s-icon-2x.png", color)), fmt.Sprintf("images/marker-%s-icon-2x.png", color), *outputDir)
	}
	utils.MustCopyHash(data.Path("static", "favicon.ico"), "favicon.ico", *outputDir)
	utils.MustCopyHash(data.Path("static", "favicon.svg"), "favicon.svg", *outputDir)
	mustCreateIndexNow(config.IndexNow, *outputDir)

	// render templates to output folder
	active := 0
	planned := 0
	archived := 0
	for _, event := range events {
		if event.Active() {
			active += 1
		} else if event.Planned() {
			planned += 1
		} else {
			archived += 1
		}
	}

	canonical := func(path string) string {
		return fmt.Sprintf("https://%s/%s", config.Domain, path)
	}

	renderData := RenderData{&config, nil, events, active, planned, archived, js_files, css_files, "", "", "", "", now.Format("2006-01-02 15:04:05"), nil}
	t := PathBuilder(filepath.Join(*dataDir, "templates"))
	renderData.set("Karte aller deutschen parkruns", "Karte aller deutschen parkruns mit Anzeige der einzelnen Laufstrecken und Informationen zum letzten Event", canonical(""), "map")
	if err := renderData.render(output.Path("index.html"), t.Path("index.html"), t.Path("header.html"), t.Path("footer.html"), t.Path("tail.html")); err != nil {
		panic(fmt.Errorf("while rendering 'index.html': %v", err))
	}
	renderData.set("Liste aller deutschen parkruns", "Liste aller deutschen parkruns mit Informationen zum letzten Event", canonical("liste.html"), "list")
	if err := renderData.render(output.Path("liste.html"), t.Path("liste.html"), t.Path("header.html"), t.Path("footer.html"), t.Path("tail.html")); err != nil {
		panic(fmt.Errorf("while rendering 'list.html': %v", err))
	}
	renderData.set("parkruns Karte - Info", "Informationen", canonical("info.html"), "info")
	if err := renderData.render(output.Path("info.html"), t.Path("info.html"), t.Path("header.html"), t.Path("footer.html"), t.Path("tail.html")); err != nil {
		panic(fmt.Errorf("while rendering 'info.html': %v", err))
	}
	renderData.set("parkruns Karte - Datenschutz", "Datenschutzinformationen", canonical("datenschutz.html"), "datenschutz")
	if err := renderData.render(output.Path("datenschutz.html"), t.Path("datenschutz.html"), t.Path("header.html"), t.Path("footer.html"), t.Path("tail.html")); err != nil {
		panic(fmt.Errorf("while rendering 'datenschutz.html': %v", err))
	}
	renderData.set("parkruns Karte - Impressum", "Impressum", canonical("impressum.html"), "impressum")
	if err := renderData.render(output.Path("impressum.html"), t.Path("impressum.html"), t.Path("header.html"), t.Path("footer.html"), t.Path("tail.html")); err != nil {
		panic(fmt.Errorf("while rendering 'impressum.html': %v", err))
	}

	for _, event := range events {
		renderData.Event = event
		title := fmt.Sprintf("%s, %s", event.FixedName(), event.FixedLocation())
		description := fmt.Sprintf("Informationen zum %s in %s; Streckenkarte, Statistiken, Links", event.FixedName(), event.FixedLocation())
		file := fmt.Sprintf("%s.html", event.Id)
		renderData.set(title, description, canonical(file), "list")
		if err := renderData.render(output.Path(file), t.Path("parkrun.html"), t.Path("header.html"), t.Path("footer.html"), t.Path("tail.html")); err != nil {
			panic(fmt.Errorf("while rendering '%s': %v", file, err))
		}
	}

	if err := renderData.writeSitemap(output.Path("sitemap.txt")); err != nil {
		panic(fmt.Errorf("while writing sitemap: %w", err))
	}

	if *exportCsvFile != "" {
		if err := parkrun.ExportCsv(events, *exportCsvFile); err != nil {
			panic(fmt.Errorf("while exporting CSV: %w", err))
		}
	}
}
