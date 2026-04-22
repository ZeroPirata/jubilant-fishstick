package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// jobRightNextData é a estrutura do __NEXT_DATA__ injetado pelo Next.js.
// Os campos refletem a estrutura observada em jobright.ai/jobs/info/{id}.
type jobRightNextData struct {
	Props struct {
		PageProps struct {
			Job struct {
				Title       string `json:"title"`
				CompanyName string `json:"companyName"`
				Description string `json:"description"`
				// fallbacks que o site pode usar
				JobTitle string `json:"jobTitle"`
				Company  string `json:"company"`
			} `json:"job"`
			// alguns sites embutem direto em pageProps
			Title       string `json:"title"`
			CompanyName string `json:"companyName"`
			Description string `json:"description"`
		} `json:"pageProps"`
	} `json:"props"`
}

func (s *Scraper) scrapJobRight() (BasicScraperResult, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest("GET", s.Url, nil)
	if err != nil {
		return BasicScraperResult{}, err
	}

	resp, err := DoRequestWithRetry(client, req, 3)
	if err != nil {
		return BasicScraperResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return BasicScraperResult{}, fmt.Errorf("jobright retornou status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return BasicScraperResult{}, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return BasicScraperResult{}, err
	}

	// Tenta extrair do __NEXT_DATA__ primeiro (Next.js SSR)
	if result, ok := s.parseJobRightNextData(doc); ok {
		return result, nil
	}

	// Fallback: CSS selectors direto no HTML
	return s.parseJobRightHTML(doc)
}

func (s *Scraper) parseJobRightNextData(doc *goquery.Document) (BasicScraperResult, bool) {
	raw := doc.Find(`script#__NEXT_DATA__`).Text()
	if raw == "" {
		return BasicScraperResult{}, false
	}

	var nd jobRightNextData
	if err := json.Unmarshal([]byte(raw), &nd); err != nil {
		s.Logger.Warn("jobright: falha ao parsear __NEXT_DATA__")
		return BasicScraperResult{}, false
	}

	pp := nd.Props.PageProps
	job := pp.Job

	title := firstNonEmpty(job.Title, job.JobTitle, pp.Title)
	company := firstNonEmpty(job.CompanyName, job.Company, pp.CompanyName)
	description := firstNonEmpty(job.Description, pp.Description)

	if title == "" {
		return BasicScraperResult{}, false
	}

	return BasicScraperResult{
		Title:            CleanString(title),
		Company:          CleanString(company),
		BasicDescription: CleanString(description),
	}, true
}

func (s *Scraper) parseJobRightHTML(doc *goquery.Document) (BasicScraperResult, error) {
	var result BasicScraperResult

	// Seletores observados no HTML estático do jobright.ai (sujeito a mudança)
	result.Title = CleanString(doc.Find("h1").First().Text())
	result.Company = CleanString(doc.Find(`[class*="company"]`).First().Text())

	var sb strings.Builder
	doc.Find(`[class*="description"], [class*="job-detail"], [class*="jobDetail"]`).Each(func(_ int, sel *goquery.Selection) {
		sel.Find("p, li, h2, h3").Each(func(_ int, el *goquery.Selection) {
			if t := strings.TrimSpace(el.Text()); t != "" {
				sb.WriteString(t)
				sb.WriteString("\n")
			}
		})
	})
	result.BasicDescription = strings.Join(strings.Fields(sb.String()), " ")

	if result.Title == "" {
		return BasicScraperResult{}, fmt.Errorf("jobright: não foi possível extrair título da vaga")
	}

	return result, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
