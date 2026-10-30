package scraper

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// scrapProgramaThor lê a página de uma vaga em programathor.com.br/jobs/<id>. A vaga
// inteira vem no JSON-LD (schema.org JobPosting) da página.
func (s *Scraper) scrapProgramaThor() (BasicScraperResult, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", s.Url, nil)
	if err != nil {
		return BasicScraperResult{}, err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return BasicScraperResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return BasicScraperResult{}, fmt.Errorf("ProgramaThor retornou status %d", resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return BasicScraperResult{}, err
	}
	return readJobPostingLD(doc)
}

// Alguns sites põem quebras de linha cruas dentro das strings do JSON-LD, o que é JSON
// inválido; no HTML da descrição elas não fazem diferença.
var rawControl = regexp.MustCompile(`[\x00-\x1f]`)
