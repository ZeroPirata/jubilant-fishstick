package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type gupyJobPage struct {
	Props struct {
		PageProps struct {
			Job struct {
				Name             string `json:"name"`
				Description      string `json:"description"`
				Responsibilities string `json:"responsibilities"`
				Prerequisites    string `json:"prerequisites"`
				CareerPage       struct {
					Name string `json:"name"`
				} `json:"careerPage"`
			} `json:"job"`
		} `json:"pageProps"`
	} `json:"props"`
}

// scrapGupy lê a página de uma vaga em <empresa>.gupy.io/job/<id>. A Gupy é o ATS
// mais usado no Brasil e a página traz a vaga inteira no __NEXT_DATA__.
func (s *Scraper) scrapGupy() (BasicScraperResult, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

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
		return BasicScraperResult{}, fmt.Errorf("Gupy retornou status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return BasicScraperResult{}, err
	}

	return readGupyJobPage(doc)
}

func readGupyJobPage(doc *goquery.Document) (BasicScraperResult, error) {
	raw, err := NextData(doc)
	if err != nil {
		return BasicScraperResult{}, err
	}

	var page gupyJobPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return BasicScraperResult{}, fmt.Errorf("JSON da vaga Gupy inválido: %w", err)
	}

	job := page.Props.PageProps.Job
	if job.Name == "" {
		return BasicScraperResult{}, fmt.Errorf("falha ao extrair dados essenciais da Gupy")
	}

	var parts []string
	for _, section := range []string{job.Description, job.Responsibilities, job.Prerequisites} {
		if text := HTMLToText(section); text != "" {
			parts = append(parts, text)
		}
	}

	return BasicScraperResult{
		Title:            strings.TrimSpace(job.Name),
		Company:          strings.TrimSpace(job.CareerPage.Name),
		BasicDescription: strings.Join(parts, "\n"),
	}, nil
}
