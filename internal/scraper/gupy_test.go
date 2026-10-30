package scraper

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestReadGupyJobPage(t *testing.T) {
	html := `<html><body><script id="__NEXT_DATA__" type="application/json">
	{"props":{"pageProps":{"job":{
		"name":"Golang Back-End Developer",
		"description":"<p><span>Estamos em busca de uma pessoa Back-End.</span></p>",
		"responsibilities":"<ul><li>Projetar serviços em Go</li><li>Manter APIs REST</li></ul>",
		"prerequisites":"",
		"careerPage":{"name":"Invillia"}}}}}
	</script></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	got, err := readGupyJobPage(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Golang Back-End Developer" || got.Company != "Invillia" {
		t.Errorf("título/empresa errados: %+v", got)
	}
	want := "Estamos em busca de uma pessoa Back-End.\nProjetar serviços em Go\nManter APIs REST"
	if got.BasicDescription != want {
		t.Errorf("descrição:\n%q\nquer\n%q", got.BasicDescription, want)
	}
}

func TestReadGupyJobPage_SemNextData(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader("<html><body>bloqueado</body></html>"))
	if _, err := readGupyJobPage(doc); err == nil {
		t.Error("página sem __NEXT_DATA__ deveria dar erro")
	}
}
