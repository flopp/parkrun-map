package main

import (
	"encoding/xml"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flopp/parkrun-map/internal/parkrun"
)

type testSitemapURL struct {
	Loc string `xml:"loc"`
}

type testSitemapURLSet struct {
	XMLName xml.Name         `xml:"urlset"`
	Xmlns   string           `xml:"xmlns,attr"`
	URLs    []testSitemapURL `xml:"url"`
}

func TestWriteSitemap(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "sitemap.xml")

	data := RenderData{
		CanonicalUrls: []CanonicalUrl{
			{Url: "https://example.com/"},
			{Url: "https://example.com/articles/a?x=1&y=2"},
		},
	}

	if err := data.writeSitemap(filePath); err != nil {
		t.Fatalf("writeSitemap() error = %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if got, want := string(content[:len(xml.Header)]), xml.Header; got != want {
		t.Fatalf("xml header mismatch\nwant: %q\ngot:  %q", want, got)
	}

	var sitemap testSitemapURLSet
	if err := xml.Unmarshal(content, &sitemap); err != nil {
		t.Fatalf("Unmarshal() error = %v\ncontent:\n%s", err, string(content))
	}

	if sitemap.Xmlns != "http://www.sitemaps.org/schemas/sitemap/0.9" {
		t.Fatalf("xmlns mismatch: %q", sitemap.Xmlns)
	}

	if len(sitemap.URLs) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(sitemap.URLs))
	}

	if sitemap.URLs[0].Loc != "https://example.com/" {
		t.Fatalf("first URL mismatch: %q", sitemap.URLs[0].Loc)
	}

	if sitemap.URLs[1].Loc != "https://example.com/articles/a?x=1&y=2" {
		t.Fatalf("second URL mismatch: %q", sitemap.URLs[1].Loc)
	}
}

func TestWriteHtaccessRedirectsArticleIndex(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, ".htaccess")

	if err := (RenderData{}).writeHtaccess(filePath); err != nil {
		t.Fatalf("writeHtaccess() error = %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	got := string(content)
	want := "RewriteCond %{THE_REQUEST} \\s/+articles/index\\.html(?:[\\s?]|$) [NC]\n" +
		"RewriteRule ^articles/index\\.html$ /articles/ [R=301,L]\n"
	if !strings.Contains(got, want) {
		t.Fatalf("article index redirect missing from .htaccess:\n%s", got)
	}
}

func TestRenderCancellationsPage(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "cancellations.html")

	data := RenderData{
		Events: []*parkrun.Event{
			{Id: "event-a", Name: "Event A", Location: "Berlin", Status: "", Cancellations: []parkrun.Cancellation{{Date: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Description: "Weather"}}},
			{Id: "event-b", Name: "Event B", Location: "Hamburg", Status: ""},
		},
		Config: &Config{},
	}
	data.set("Absagen", "Absagen", "https://example.com/cancellations.html", "2026-09-09", "")

	if err := data.render(outputFile,
		"../../data/templates/cancellations.html",
		"../../data/templates/header.html",
		"../../data/templates/footer.html",
		"../../data/templates/tail.html",
	); err != nil {
		t.Fatalf("render() error = %v", err)
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	got := string(content)
	if !strings.Contains(got, "Event A") {
		t.Fatalf("rendered page did not include the cancelled event: %s", got)
	}
	if strings.Contains(got, "Event B") {
		t.Fatalf("rendered page unexpectedly included a non-cancelled event: %s", got)
	}
}

func TestRender404Page(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "404.html")

	data := RenderData{}
	data.set("404 - Seite nicht gefunden", "Die angeforderte Seite wurde nicht gefunden.", "", "", "404")

	if err := data.render(outputFile,
		"../../data/templates/404.html",
		"../../data/templates/header.html",
		"../../data/templates/footer.html",
		"../../data/templates/tail.html",
	); err != nil {
		t.Fatalf("render() error = %v", err)
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	got := string(content)
	if strings.Contains(got, "rel=\"canonical\"") {
		t.Fatalf("404 page unexpectedly included a canonical URL: %s", got)
	}
	if !strings.Contains(got, `<meta name="robots" content="noindex">`) {
		t.Fatalf("404 page did not include noindex: %s", got)
	}
	if len(data.CanonicalUrls) != 0 {
		t.Fatalf("404 page unexpectedly added a sitemap URL: %#v", data.CanonicalUrls)
	}
}

func TestCopyResizedImages(t *testing.T) {
	sourceDir := t.TempDir()
	imagePath := filepath.Join(sourceDir, "location.jpg")
	sourceImage := image.NewRGBA(image.Rect(0, 0, 640, 320))
	for y := 0; y < 320; y++ {
		for x := 0; x < 640; x++ {
			sourceImage.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), A: 255})
		}
	}
	sourceFile, err := os.Create(imagePath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := jpeg.Encode(sourceFile, sourceImage, nil); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	if err := sourceFile.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	destinationDir := filepath.Join(t.TempDir(), "images", "parkruns")
	if err := copyResizedImages(sourceDir, destinationDir); err != nil {
		t.Fatalf("copyResizedImages() error = %v", err)
	}

	outputPath := filepath.Join(destinationDir, "location.jpg")
	outputFile, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("Open() output image error = %v", err)
	}
	outputImage, err := jpeg.Decode(outputFile)
	if err != nil {
		t.Fatalf("jpeg.Decode() error = %v", err)
	}
	if err := outputFile.Close(); err != nil {
		t.Fatalf("Close() output image error = %v", err)
	}

	if got, want := outputImage.Bounds().Dx(), 256; got != want {
		t.Errorf("output width = %d, want %d", got, want)
	}
	if got, want := outputImage.Bounds().Dy(), 128; got != want {
		t.Errorf("output height = %d, want %d", got, want)
	}
}

func TestRenderPlannedPageImages(t *testing.T) {
	outputFile := filepath.Join(t.TempDir(), "planned.html")
	data := RenderData{
		Config: &Config{},
		PlannedDataTermin: []*PlannedData{
			{City: "Termin", Image: "images/parkruns/termin.jpg"},
		},
		PlannedDataTest: []*PlannedData{
			{City: "Test", Image: "images/parkruns/test.jpg"},
		},
		PlannedDataOther: []*PlannedData{
			{City: "Weitere", Image: "images/parkruns/weitere.jpg"},
			{City: "Ohne Bild"},
		},
	}
	data.set("Geplante parkruns", "Geplante parkruns", "https://example.com/planned.html", "", "planned")

	if err := data.render(outputFile,
		"../../data/templates/planned.html",
		"../../data/templates/header.html",
		"../../data/templates/footer.html",
		"../../data/templates/tail.html",
	); err != nil {
		t.Fatalf("render() error = %v", err)
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(content)
	for _, image := range []string{
		"/images/parkruns/termin.jpg",
		"/images/parkruns/test.jpg",
		"/images/parkruns/weitere.jpg",
	} {
		if !strings.Contains(got, image) {
			t.Errorf("rendered page does not include image %q", image)
		}
	}
	if got, want := strings.Count(got, `class="planned-card-image"`), 3; got != want {
		t.Errorf("rendered image count = %d, want %d", got, want)
	}
}
