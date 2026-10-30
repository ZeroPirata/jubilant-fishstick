package scraper

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Leitura de qualquer site de vagas sem leitor próprio. Primeiro o JSON-LD
// JobPosting (o formato do Google for Jobs, presente em Lever, Ashby, Workday e em
// muitos sites de carreira); sem ele, o texto principal da página.

// Mínimo de texto para considerar que a página trouxe a vaga, e não só menu e rodapé.
const minGenericText = 400

// Teto do HTML lido: página de vaga passa longe disso.
const maxPageBytes = 5 << 20

// ErrNeedsBrowser: a página monta a vaga por JavaScript (ou bloqueia leitura
// automática). Sem navegador não dá para ler; o usuário pode colar a descrição.
var ErrNeedsBrowser = errors.New("este site monta a vaga só no navegador ou bloqueia leitura automática: cadastre a vaga colando título, empresa e descrição")

// Identifica a leitura como feita pelo sistema, a pedido do usuário, uma página por vez.
const readerUserAgent = "Mozilla/5.0 (compatible; hackton-treino/1.0; leitura de vaga a pedido do usuario)"

func (s *Scraper) scrapGeneric() (BasicScraperResult, error) {
	doc, err := fetchHTML(s.Url)
	if err != nil {
		return BasicScraperResult{}, err
	}
	return readGeneric(doc)
}

func fetchHTML(url string) (*goquery.Document, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", readerUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf("o site recusou o acesso (status %d): %w", resp.StatusCode, ErrNeedsBrowser)
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return nil, fmt.Errorf("a página da vaga não existe mais (status %d)", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("o site respondeu com status %d", resp.StatusCode)
	}
	return goquery.NewDocumentFromReader(io.LimitReader(resp.Body, maxPageBytes))
}

func readGeneric(doc *goquery.Document) (BasicScraperResult, error) {
	if res, err := readJobPostingLD(doc); err == nil && len(res.BasicDescription) >= minGenericText/2 {
		return res, nil
	}

	title := firstText(
		attr(doc, `meta[property="og:title"]`, "content"),
		doc.Find("h1").First().Text(),
		doc.Find("title").First().Text(),
	)
	company := firstText(attr(doc, `meta[property="og:site_name"]`, "content"))

	doc.Find("script, style, noscript, template, svg, nav, header, footer, form, button, iframe, [aria-hidden='true']").Remove()
	text := ""
	for _, sel := range []string{"main", "article", "[role='main']", "#content", ".job-description", "body"} {
		if node := doc.Find(sel).First(); node.Length() > 0 {
			if t := blockText(node); len(t) >= minGenericText {
				text = t
				break
			}
		}
	}
	if text == "" {
		return BasicScraperResult{}, ErrNeedsBrowser
	}
	return BasicScraperResult{Title: title, Company: company, BasicDescription: text}, nil
}

// blockText extrai o texto em parágrafos e itens (como HTMLToText) de um trecho já
// sem menus e scripts.
func blockText(node *goquery.Selection) string {
	html, err := goquery.OuterHtml(node)
	if err != nil {
		return ""
	}
	return HTMLToText(html)
}

func attr(doc *goquery.Document, selector, name string) string {
	v, _ := doc.Find(selector).First().Attr(name)
	return v
}

func firstText(values ...string) string {
	for _, v := range values {
		if v = strings.Join(strings.Fields(v), " "); v != "" {
			return v
		}
	}
	return ""
}

// ── JSON-LD ──

// readJobPostingLD acha o JobPosting entre os scripts application/ld+json da página:
// sozinho, numa lista ou dentro de um @graph.
func readJobPostingLD(doc *goquery.Document) (BasicScraperResult, error) {
	var found map[string]any
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		var data any
		// Alguns sites põem quebras de linha cruas dentro das strings, o que é JSON
		// inválido; no HTML da descrição elas não fazem diferença.
		if json.Unmarshal([]byte(rawControl.ReplaceAllString(s.Text(), " ")), &data) != nil {
			return true
		}
		found = findJobPosting(data)
		return found == nil
	})
	if found == nil {
		return BasicScraperResult{}, fmt.Errorf("a página não tem os dados da vaga (JobPosting)")
	}
	title := strings.TrimSpace(html.UnescapeString(str(found["title"])))
	if title == "" {
		return BasicScraperResult{}, fmt.Errorf("a página não tem os dados da vaga (JobPosting)")
	}
	company := ""
	switch org := found["hiringOrganization"].(type) {
	case map[string]any:
		company = str(org["name"])
	case string:
		company = org
	}
	return BasicScraperResult{
		Title:            title,
		Company:          strings.TrimSpace(html.UnescapeString(company)),
		BasicDescription: HTMLToText(str(found["description"])),
	}, nil
}

func findJobPosting(data any) map[string]any {
	switch v := data.(type) {
	case []any:
		for _, item := range v {
			if jp := findJobPosting(item); jp != nil {
				return jp
			}
		}
	case map[string]any:
		if isType(v["@type"], "JobPosting") {
			return v
		}
		if graph, ok := v["@graph"]; ok {
			return findJobPosting(graph)
		}
	}
	return nil
}

// isType: "@type" pode ser texto ou lista ("JobPosting" ou ["JobPosting"]).
func isType(t any, want string) bool {
	switch v := t.(type) {
	case string:
		return v == want
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

func str(v any) string {
	s, _ := v.(string)
	return s
}
