package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
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
		CanonicalUrls: []string{
			"https://example.com/",
			"https://example.com/articles/a?x=1&y=2",
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
