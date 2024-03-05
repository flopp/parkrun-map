package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/flopp/parkrun-map/internal/parkrun"
	"github.com/flopp/parkrun-map/internal/utils"
)

type RenderData struct {
	Events         []*parkrun.Event
	ActiveEvents   int
	ArchivedEvents int
	JsFiles        []string
	CssFiles       []string
	StatsJs        string
	Title          string
	Description    string
	Canonical      string
	Nav            string
	Timestamp      string
}

func (data *RenderData) set(title, description, canonical, nav string) {
	data.Title = title
	data.Description = description
	data.Canonical = canonical
	data.Nav = nav
}

func (data RenderData) render(outputFile string, templateFiles ...string) error {
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

	return tmpl.Execute(f, data)
}

type PathBuilder string

func (p PathBuilder) Path(items ...string) string {
	joined := string(p)
	for _, item := range items {
		joined = fmt.Sprintf("%s/%s", joined, item)
	}
	return joined
}

func modifyGoatcounterLinkSelector(dir, file string) string {
	path := filepath.Join(dir, file)
	data, err := os.ReadFile(path)
	if err != nil {
		return file
	}

	data = bytes.ReplaceAll(data, []byte(`querySelectorAll("*[data-goatcounter-click]")`), []byte(`querySelectorAll("a[target=_blank]")`))
	data = bytes.ReplaceAll(data, []byte(`(elem.dataset.goatcounterClick || elem.name || elem.id || '')`), []byte(`(elem.dataset.goatcounterClick || elem.name || elem.id || elem.href || '')`))
	data = bytes.ReplaceAll(data, []byte(`(elem.dataset.goatcounterReferrer || elem.dataset.goatcounterReferral || '')`), []byte(`(elem.dataset.goatcounterReferrer || elem.dataset.goatcounterReferral || window.location.href || '')`))
	file2 := fmt.Sprintf("mod-%s", file)
	path2 := filepath.Join(dir, file2)
	os.WriteFile(path2, data, 0770)
	return file2
}

func randomDuration(min, max time.Duration) time.Duration {
	delta := max - min
	return min + time.Duration(rand.Int63n(int64(delta+1)))
}

func renderJs(events []*parkrun.Event, filePath string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0770); err != nil {
		return err
	}

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	fmt.Fprintf(out, "var parkruns = [\n")
	for i, event := range events {
		if i != 0 {
			fmt.Fprintf(out, ",\n")
		}
		fmt.Fprintf(out, "{\n")
		fmt.Fprintf(out, "\"url\": \"%s\",\n", event.Url())
		fmt.Fprintf(out, "\"name\": \"%s\",\n", event.FixedName())
		fmt.Fprintf(out, "\"lat\": %f, \"lon\": %f,\n", event.Lat, event.Lon)
		fmt.Fprintf(out, "\"location\": \"%s\",\n", event.FixedLocation())
		fmt.Fprintf(out, "\"googleMapsUrl\": \"%s\",\n", event.GoogleMapsUrl())
		fmt.Fprintf(out, "\"tracks\": [\n")
		for it, track := range event.Tracks {
			if it != 0 {
				fmt.Fprintf(out, ",\n")
			}
			fmt.Fprintf(out, "[\n")
			for ic, coord := range track {
				if ic != 0 {
					fmt.Fprintf(out, ",")
				}
				fmt.Fprintf(out, "[%f, %f]", coord.Lat, coord.Lon)
			}
			fmt.Fprintf(out, "]\n")
		}
		fmt.Fprintf(out, "],\n")

		if event.Active() {
			fmt.Fprintf(out, "\"active\": true,\n")
		} else {
			fmt.Fprintf(out, "\"active\": false,\n")
		}
		if event.LatestRun != nil {
			fmt.Fprintf(out, "\"latest\": {\n")
			fmt.Fprintf(out, "\"index\": %d,\n", event.LatestRun.Index)
			fmt.Fprintf(out, "\"url\": \"%s\",\n", event.LatestRun.Url())
			fmt.Fprintf(out, "\"date\": \"%s\",\n", event.LatestRun.DateF())
			fmt.Fprintf(out, "\"runners\": %d\n", event.LatestRun.RunnerCount)
			fmt.Fprintf(out, "}\n")
		} else {
			fmt.Fprintf(out, "\"latest\": null\n")
		}

		/*
		   "tracks" : "{{.EncodedTracks}}",

		*/
		fmt.Fprintf(out, "}\n")
	}
	fmt.Fprintf(out, "];")

	return nil
}

func main() {
	dataDir := flag.String("data", "data", "the data directory")
	downloadDir := flag.String("download", ".download", "the download directory")
	outputDir := flag.String("output", ".output", "the output directory")
	flag.Parse()

	now := time.Now()
	fileAge1d := now.Add(-24 * time.Hour)
	fileAge1w := now.Add(-24 * 7 * time.Hour)

	utils.SetDownloadDelay(1 * time.Second)

	data := PathBuilder(*dataDir)
	download := PathBuilder(*downloadDir)

	// fetch parkrun events
	events_json_url := "https://images.parkrun.com/events.json"
	events_json_file := download.Path("parkrun", "events.json.gz")
	if err := utils.DownloadFileIfOlder(events_json_url, events_json_file, fileAge1d); err != nil {
		panic(fmt.Errorf("while downloading %s to %s: %w", events_json_url, events_json_file, err))
	}

	parkruns_json_file := data.Path("parkruns.json")

	// parse parkrun events (only returns German events!)
	events, err := parkrun.LoadEvents(events_json_file, parkruns_json_file, true /* germanOnly */)
	if err != nil {
		panic(fmt.Errorf("while parsing %s: %w", events_json_file, err))
	}

	// pull latest results
	for _, event := range events {
		wiki_url := event.WikiUrl()
		wiki_file := download.Path("parkrun", event.Id, "wiki")
		utils.MustDownloadFileIfOlder(wiki_url, wiki_file, fileAge1d)
		if err := event.LoadWiki(wiki_file); err != nil {
			panic(fmt.Errorf("file parsing %s: %w", wiki_file, err))
		}

		/*
			report_url := event.ReportUrl()
			report_file := download.Path("parkrun", event.Id, "report")
			utils.MustDownloadFileIfOlder(report_url, report_file, fileAge1d)
			if err := event.LoadReport(report_file); err != nil {
				panic(fmt.Errorf("file parsing %s: %w", report_file, err))
			}
		*/

		course_page_url := event.CoursePageUrl()
		course_page_file := download.Path("parkrun", event.Id, "course_page")
		utils.MustDownloadFileIfOlder(course_page_url, course_page_file, now.Add(randomDuration(-24*14*time.Hour, -24*7*time.Hour)))
		if err := event.LoadCoursePage(course_page_file); err != nil {
			panic(fmt.Errorf("file parsing %s: %w", course_page_file, err))
		}

		kml_url := fmt.Sprintf("https://www.google.com/maps/d/kml?mid=%s&forcekml=1", event.GoogleMapsId)
		kml_file := download.Path("parkrun", event.Id, "kml")
		utils.MustDownloadFileIfOlder(kml_url, kml_file, now.Add(randomDuration(-24*14*time.Hour, -24*7*time.Hour)))

		if err := event.LoadKML(kml_file); err != nil {
			panic(fmt.Errorf("file parsing %s: %w", kml_file, err))
		}
	}

	// fetch external assets (bulma, leaflet)

	// renovate: datasource=npm depName=bulma
	bulma_version := "0.9.4"
	// renovate: datasource=npm depName=leaflet
	leaflet_version := "1.9.4"

	bulma_url := PathBuilder(fmt.Sprintf("https://unpkg.com/bulma@%s", bulma_version))
	leaflet_url := PathBuilder(fmt.Sprintf("https://unpkg.com/leaflet@%s", leaflet_version))

	// download leaflet
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/leaflet.js"), download.Path("leaflet", "leaflet.js"), fileAge1w)
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/leaflet.css"), download.Path("leaflet", "leaflet.css"), fileAge1w)
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/images/marker-icon.png"), download.Path("leaflet", "marker-icon.png"), fileAge1w)
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/images/marker-icon-2x.png"), download.Path("leaflet", "marker-icon-2x.png"), fileAge1w)
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/images/marker-shadow.png"), download.Path("leaflet", "marker-shadow.png"), fileAge1w)

	// download bulma
	utils.MustDownloadFileIfOlder(bulma_url.Path("css/bulma.min.css"), download.Path("bulma", "bulma.css"), fileAge1w)

	// download goatcounter
	utils.MustDownloadFileIfOlder("https://gc.zgo.at/count.js", download.Path("goatcounter", "stats.js"), fileAge1w)

	// render data
	if err := renderJs(events, download.Path("data.js")); err != nil {
		panic(fmt.Errorf("failed to render data: %v", err))
	}

	js_files := make([]string, 0)
	js_files = append(js_files, utils.MustCopyHash(download.Path("data.js"), "data-HASH.js", *outputDir))
	js_files = append(js_files, utils.MustCopyHash(download.Path("leaflet/leaflet.js"), "leaflet-HASH.js", *outputDir))
	js_files = append(js_files, utils.MustCopyHash(data.Path("static", "main.js"), "main-HASH.js", *outputDir))
	css_files := make([]string, 0)
	css_files = append(css_files, utils.MustCopyHash(download.Path("bulma/bulma.css"), "bulma-HASH.css", *outputDir))
	css_files = append(css_files, utils.MustCopyHash(download.Path("leaflet/leaflet.css"), "leaflet-HASH.css", *outputDir))
	css_files = append(css_files, utils.MustCopyHash(data.Path("static", "style.css"), "style-HASH.css", *outputDir))
	utils.MustCopyHash(download.Path("leaflet/marker-icon.png"), "images/marker-icon.png", *outputDir)
	utils.MustCopyHash(download.Path("leaflet/marker-icon-2x.png"), "images/marker-icon-2x.png", *outputDir)
	utils.MustCopyHash(download.Path("leaflet/marker-shadow.png"), "images/marker-shadow.png", *outputDir)
	utils.MustCopyHash(data.Path("static/marker-red-icon.png"), "images/marker-red-icon.png", *outputDir)
	utils.MustCopyHash(data.Path("static/marker-red-icon-2x.png"), "images/marker-red-icon-2x.png", *outputDir)
	statsJs := modifyGoatcounterLinkSelector(download.Path("goatcounter"), "stats.js")
	statsJs = utils.MustCopyHash(download.Path("goatcounter", statsJs), "stats-HASH.js", *outputDir)
	// render templates to output folder
	active := 0
	archived := 0
	for _, event := range events {
		if event.Active() {
			active += 1
		} else {
			archived += 1
		}
	}
	renderData := RenderData{events, active, archived, js_files, css_files, statsJs, "", "", "", "", now.Format("2006-01-02 15:04:05")}
	t := PathBuilder(filepath.Join(*dataDir, "templates"))
	renderData.set("parkrun Karte", "Karte aller deutschen parkruns mit Anzeige der einzelnen Laufstrecken und Informationen zum letzten Event", "https://parkrun.flopp.net/", "map")
	if err := renderData.render(filepath.Join(*outputDir, "index.html"), t.Path("index.html"), t.Path("header.html"), t.Path("footer.html"), t.Path("tail.html")); err != nil {
		panic(fmt.Errorf("while rendering 'index.html': %v", err))
	}
	renderData.set("parkrun Liste", "Liste aller deutschen parkruns mit Informationen zum letzten Event", "https://parkrun.flopp.net/liste.html", "list")
	if err := renderData.render(filepath.Join(*outputDir, "liste.html"), t.Path("liste.html"), t.Path("header.html"), t.Path("footer.html"), t.Path("tail.html")); err != nil {
		panic(fmt.Errorf("while rendering 'list.html': %v", err))
	}
	renderData.set("parkruns Karte - Info", "Informationen", "https://parkrun.flopp.net/info.html", "info")
	if err := renderData.render(filepath.Join(*outputDir, "info.html"), t.Path("info.html"), t.Path("header.html"), t.Path("footer.html"), t.Path("tail.html")); err != nil {
		panic(fmt.Errorf("while rendering 'info.html': %v", err))
	}
	renderData.set("parkruns Karte - Datenschutz", "Datenschutzinformationen", "https://parkrun.flopp.net/datenschutz.html", "datenschutz")
	if err := renderData.render(filepath.Join(*outputDir, "datenschutz.html"), t.Path("datenschutz.html"), t.Path("header.html"), t.Path("footer.html"), t.Path("tail.html")); err != nil {
		panic(fmt.Errorf("while rendering 'datenschutz.html': %v", err))
	}
	renderData.set("parkruns Karte - Impressum", "Impressum", "https://parkrun.flopp.net/impressum.html", "impressum")
	if err := renderData.render(filepath.Join(*outputDir, "impressum.html"), t.Path("impressum.html"), t.Path("header.html"), t.Path("footer.html"), t.Path("tail.html")); err != nil {
		panic(fmt.Errorf("while rendering 'impressum.html': %v", err))
	}
}
