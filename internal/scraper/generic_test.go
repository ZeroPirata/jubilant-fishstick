package scraper

import (
	"errors"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
)

func docOf(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

var longText = strings.Repeat("Experiência com Go, Kubernetes e mensageria. ", 20)

func TestReadGeneric_JSONLDNoGraph(t *testing.T) {
	doc := docOf(t, `<html><head><script type="application/ld+json">{"@context":"https://schema.org","@graph":[
		{"@type":"Organization","name":"X"},
		{"@type":["JobPosting"],"title":"Engenheiro Go","hiringOrganization":"Acme &amp; Co","description":"<p>`+longText+`</p>"}]}</script></head></html>`)
	got, err := readGeneric(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Engenheiro Go" || got.Company != "Acme & Co" || !strings.Contains(got.BasicDescription, "Kubernetes") {
		t.Errorf("vaga: %+v", got)
	}
}

func TestReadGeneric_TextoDaPagina(t *testing.T) {
	doc := docOf(t, `<html><head><title>Vaga | Empresa</title><meta property="og:title" content="Dev Back-end Pleno">
		<meta property="og:site_name" content="Empresa X"></head><body>
		<nav><p>Menu Início Vagas Sobre</p></nav>
		<main><h1>Dev Back-end Pleno</h1><p>`+longText+`</p><ul><li>Remoto</li></ul></main>
		<footer><p>© Empresa X</p></footer><script>var x = 1;</script></body></html>`)
	got, err := readGeneric(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Dev Back-end Pleno" || got.Company != "Empresa X" {
		t.Errorf("título/empresa: %+v", got)
	}
	if strings.Contains(got.BasicDescription, "Menu") || strings.Contains(got.BasicDescription, "©") || !strings.Contains(got.BasicDescription, "Remoto") {
		t.Errorf("devia ficar só o conteúdo principal: %q", got.BasicDescription)
	}
}

func TestReadGeneric_PaginaSoJavaScript(t *testing.T) {
	doc := docOf(t, `<html><head><title>Carreiras</title></head><body><div id="root"></div><script src="app.js"></script></body></html>`)
	if _, err := readGeneric(doc); !errors.Is(err, ErrNeedsBrowser) {
		t.Errorf("página vazia sem JavaScript deveria pedir a descrição colada, deu %v", err)
	}
}

func TestScrape_SiteBloqueado(t *testing.T) {
	s := NewScraper("https://paypal.eightfold.ai/careers/job/123", zap.NewNop())
	_, err := s.Scrape()
	if !errors.Is(err, ErrUnsupportedDomain) || !errors.Is(err, ErrNeedsBrowser) {
		t.Errorf("Eightfold deveria responder na hora pedindo a descrição: %v", err)
	}
}

func TestATSPaths(t *testing.T) {
	if m := greenhousePath.FindStringSubmatch("/stone/jobs/8040000003"); m == nil || m[1] != "stone" || m[2] != "8040000003" {
		t.Errorf("greenhouse: %v", m)
	}
	if m := smartRecruitersPath.FindStringSubmatch("/BoschGroup/744000158794999-devops-intern"); m == nil || m[1] != "BoschGroup" || m[2] != "744000158794999" {
		t.Errorf("smartrecruiters: %v", m)
	}
	if m := workablePath.FindStringSubmatch("/huggingface/j/DB4D7C0EC8/"); m == nil || m[1] != "huggingface" || m[2] != "DB4D7C0EC8" {
		t.Errorf("workable: %v", m)
	}
}
