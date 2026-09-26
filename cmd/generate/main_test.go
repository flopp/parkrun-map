package main

import (
	"encoding/xml"
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
