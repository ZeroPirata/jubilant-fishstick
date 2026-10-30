package scraper

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestReadJobPostingLD(t *testing.T) {
	page := `<html><head>
<script type="application/ld+json">{"@type": "BreadcrumbList"}</script>
<script type="application/ld+json">{"@context": "http://schema.org/", "@type": "JobPosting",
 "title": "Desenvolvedor Back-End Senior",
 "description": "<p><strong>Habilidades</strong> <br> Python, Kafka
</p>",
 "hiringOrganization": {"@type": "Organization", "name": "LOLDESIGN"}}</script>
</head></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	got, err := readJobPostingLD(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Desenvolvedor Back-End Senior" || got.Company != "LOLDESIGN" || !strings.Contains(got.BasicDescription, "Python, Kafka") {
		t.Errorf("vaga: %+v", got)
	}

	empty, _ := goquery.NewDocumentFromReader(strings.NewReader(`<html></html>`))
	if _, err := readJobPostingLD(empty); err == nil {
		t.Error("página sem JobPosting deveria dar erro")
	}
}
