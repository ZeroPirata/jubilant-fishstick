package scraper

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func (s *Scraper) scrapGeekHunter() (ScraperResult, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest("GET", s.Url, nil)
	if err != nil {
		return ScraperResult{}, err
	}

	resp, err := doRequestWithRetry(client, req, 3)
	if err != nil {
		return ScraperResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ScraperResult{}, fmt.Errorf("GeekHunter retornou status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return ScraperResult{}, err
	}

	return s.readGeekHunterHtml(doc)
}

func (s *Scraper) readGeekHunterHtml(doc *goquery.Document) (ScraperResult, error) {
	var result ScraperResult

	result.Company = strings.TrimSpace(doc.Find(".css-1mo43q4").First().Text())
	result.Title = strings.TrimSpace(doc.Find(".css-jpi4pv").First().Text())
	doc.Find(".css-1y9svmk span.css-1szoa3k").Each(func(i int, sel *goquery.Selection) {
		req := strings.TrimSpace(sel.Text())
		if req != "" {
			result.Requirements = append(result.Requirements, req)
		}
	})

	var bodyBuilder strings.Builder
	doc.Find(".css-1htysii").Each(func(i int, sel *goquery.Selection) {
		sel.Find("p, li, h3, h2").Each(func(j int, item *goquery.Selection) {
			text := strings.TrimSpace(item.Text())
			if text != "" {
				bodyBuilder.WriteString(text)
				bodyBuilder.WriteString("\n")
			}
		})
	})

	result.Description = strings.TrimSpace(bodyBuilder.String())

	if result.Title == "" || result.Description == "" {
		if result.Title == "" {
			result.Title = strings.TrimSpace(doc.Find("h1").First().Text())
		}
		if result.Description == "" {
			result.Description = strings.TrimSpace(doc.Find("#job-details").Text())
		}
	}

	result.Description = strings.Join(strings.Fields(result.Description), " ")

	if result.Title == "" {
		return ScraperResult{}, fmt.Errorf("falha ao extrair dados essenciais da GeekHunter")
	}

	return result, nil
}
