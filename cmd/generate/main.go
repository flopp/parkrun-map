package main

import (
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"

	"github.com/flopp/parkrun-map/internal/parkrun"
	"github.com/flopp/parkrun-map/internal/utils"
)

type RenderData struct {
	Events    []*parkrun.Event
	JsFiles   []string
	CssFiles  []string
	Title     string
	Canonical string
	Nav       string
}

func (data *RenderData) set(title, canonical, nav string) {
	data.Title = title
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

func main() {
	dataDir := flag.String("data", "data", "the data directory")
	downloadDir := flag.String("download", ".download", "the download directory")
	outputDir := flag.String("output", ".output", "the output directory")
	flag.Parse()

	fileAge := time.Now().Add(-24 * time.Hour)
	utils.SetDownloadDelay(1 * time.Second)

	data := PathBuilder(*dataDir)
	download := PathBuilder(*downloadDir)

	// fetch parkrun events
	events_json_url := "https://images.parkrun.com/events.json"
	events_json_file := download.Path("parkrun", "events.json.gz")
	if err := utils.DownloadFileIfOlder(events_json_url, events_json_file, fileAge); err != nil {
		panic(fmt.Errorf("while downloading %s to %s: %w", events_json_url, events_json_file, err))
	}

	// parse parkrun events (only returns German events!)
	events, err := parkrun.LoadEvents(events_json_file)
	if err != nil {
		panic(fmt.Errorf("while parsing %s: %w", events_json_file, err))
	}

	// pull latest results
	for _, event := range events {
		wiki_url := event.WikiUrl()
		wiki_file := download.Path("parkrun", event.Id, "wiki")
		utils.MustDownloadFileIfOlder(wiki_url, wiki_file, fileAge)
		if err := event.LoadWiki(wiki_file); err != nil {
			panic(fmt.Errorf("file parsing %s: %w", wiki_file, err))
		}

		/*
			report_url := event.ReportUrl()
			report_file := download.Path("parkrun", event.Id, "report")
			utils.MustDownloadFileIfOlder(report_url, report_file, fileAge)
			if err := event.LoadReport(report_file); err != nil {
				panic(fmt.Errorf("file parsing %s: %w", report_file, err))
			}
		*/
	}

	// fetch external assets (bulma, leaflet)

	// renovate: datasource=npm depName=bulma
	bulma_version := "0.9.4"
	// renovate: datasource=npm depName=leaflet
	leaflet_version := "1.9.4"

	bulma_url := PathBuilder(fmt.Sprintf("https://unpkg.com/bulma@%s", bulma_version))
	leaflet_url := PathBuilder(fmt.Sprintf("https://unpkg.com/leaflet@%s", leaflet_version))

	// download leaflet
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/leaflet.js"), download.Path("leaflet", "leaflet.js"), fileAge)
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/leaflet.css"), download.Path("leaflet", "leaflet.css"), fileAge)
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/images/marker-icon.png"), download.Path("leaflet", "marker-icon.png"), fileAge)
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/images/marker-icon-2x.png"), download.Path("leaflet", "marker-icon-2x.png"), fileAge)
	utils.MustDownloadFileIfOlder(leaflet_url.Path("dist/images/marker-shadow.png"), download.Path("leaflet", "marker-shadow.png"), fileAge)

	// download bulma
	utils.MustDownloadFileIfOlder(bulma_url.Path("css/bulma.min.css"), download.Path("bulma", "bulma.css"), fileAge)

	js_files := make([]string, 0)
	js_files = append(js_files, utils.MustCopyHash(download.Path("leaflet/leaflet.js"), "leaflet-HASH.js", *outputDir))
	js_files = append(js_files, utils.MustCopyHash(data.Path("static", "main.js"), "main-HASH.js", *outputDir))
	css_files := make([]string, 0)
	css_files = append(css_files, utils.MustCopyHash(download.Path("bulma/bulma.css"), "bulma-HASH.css", *outputDir))
	css_files = append(css_files, utils.MustCopyHash(download.Path("leaflet/leaflet.css"), "leaflet-HASH.css", *outputDir))
	css_files = append(css_files, utils.MustCopyHash(data.Path("static", "style.css"), "style-HASH.css", *outputDir))
	utils.MustCopyHash(download.Path("leaflet/marker-icon.png"), "images/marker-icon.png", *outputDir)
	utils.MustCopyHash(download.Path("leaflet/marker-icon-2x.png"), "images/marker-icon-2x.png", *outputDir)
	utils.MustCopyHash(download.Path("leaflet/marker-shadow.png"), "images/marker-shadow.png", *outputDir)

	// render templates to output folder
	renderData := RenderData{events, js_files, css_files, "", "", ""}
	t := PathBuilder(filepath.Join(*dataDir, "templates"))
	renderData.set("parkruns in Deutschland - Karte", "https://parkrun.flopp.net/", "map")
	if err := renderData.render(filepath.Join(*outputDir, "index.html"), t.Path("index.html"), t.Path("header.html"), t.Path("footer.html")); err != nil {
		panic(fmt.Errorf("while rendering 'index.html': %v", err))
	}
	renderData.set("parkruns in Deutschland - Liste", "https://parkrun.flopp.net/liste.html", "list")
	if err := renderData.render(filepath.Join(*outputDir, "liste.html"), t.Path("liste.html"), t.Path("header.html"), t.Path("footer.html")); err != nil {
		panic(fmt.Errorf("while rendering 'list.html': %v", err))
	}
	renderData.set("parkruns in Deutschland - Info", "https://parkrun.flopp.net/info.html", "info")
	if err := renderData.render(filepath.Join(*outputDir, "info.html"), t.Path("info.html"), t.Path("header.html"), t.Path("footer.html")); err != nil {
		panic(fmt.Errorf("while rendering 'info.html': %v", err))
	}
	renderData.set("parkruns in Deutschland - Datenschutz", "https://parkrun.flopp.net/datenschutz.html", "datenschutz")
	if err := renderData.render(filepath.Join(*outputDir, "datenschutz.html"), t.Path("datenschutz.html"), t.Path("header.html"), t.Path("footer.html")); err != nil {
		panic(fmt.Errorf("while rendering 'datenschutz.html': %v", err))
	}
	renderData.set("parkruns in Deutschland - Impressum", "https://parkrun.flopp.net/impressum.html", "impressum")
	if err := renderData.render(filepath.Join(*outputDir, "impressum.html"), t.Path("impressum.html"), t.Path("header.html"), t.Path("footer.html")); err != nil {
		panic(fmt.Errorf("while rendering 'impressum.html': %v", err))
	}
}
