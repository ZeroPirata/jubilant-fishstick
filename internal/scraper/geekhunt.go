package scraper

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func (s *Scraper) scrapGeekHunter() (BasicScraperResult, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

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
		return BasicScraperResult{}, fmt.Errorf("GeekHunter retornou status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return BasicScraperResult{}, err
	}

	return s.readGeekHunterHtml(doc)
}

func (s *Scraper) readGeekHunterHtml(doc *goquery.Document) (BasicScraperResult, error) {
	var result BasicScraperResult
	result.Company = strings.TrimSpace(doc.Find(".css-1mo43q4").First().Text())
	result.Title = strings.TrimSpace(doc.Find(".css-jpi4pv").First().Text())

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

	result.BasicDescription = strings.TrimSpace(bodyBuilder.String())

	if result.Title == "" || result.BasicDescription == "" {
		if result.Title == "" {
			result.Title = strings.TrimSpace(doc.Find("h1").First().Text())
		}
		if result.BasicDescription == "" {
			result.BasicDescription = strings.TrimSpace(doc.Find("#job-details").Text())
		}
	}

	result.BasicDescription = strings.Join(strings.Fields(result.BasicDescription), " ")

	if result.Title == "" {
		return BasicScraperResult{}, fmt.Errorf("falha ao extrair dados essenciais da GeekHunter")
	}

	return result, nil
}
