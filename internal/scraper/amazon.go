package scraper

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func (s *Scraper) scrapAmazon() (BasicScraperResult, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
	}

	req, err := http.NewRequest("GET", s.Url, nil)
	if err != nil {
		return BasicScraperResult{}, fmt.Errorf("erro ao criar requisição: %v", err)
	}

	resp, err := DoRequestWithRetry(client, req, 3)
	if err != nil {
		return BasicScraperResult{}, fmt.Errorf("falha na rede ao acessar Amazon: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return BasicScraperResult{}, fmt.Errorf("Amazon jobs retornou erro: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return BasicScraperResult{}, fmt.Errorf("erro ao processar HTML da Amazon: %v", err)
	}

	return s.readAmazonHtml(doc)
}

func (s *Scraper) readAmazonHtml(doc *goquery.Document) (BasicScraperResult, error) {
	var res BasicScraperResult

	res.Title = strings.TrimSpace(doc.Find("h1.title").First().Text())
	res.Company = "Amazon"

	doc.Find(".section h2").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(strings.ToLower(sel.Text()), "description") {
			res.BasicDescription = strings.TrimSpace(sel.Next().Text())
		}
	})

	if res.Title == "" {
		res.Title = strings.TrimSpace(doc.Find("h1").First().Text())
	}

	if res.Title == "" {
		res.Title = "Amazon"
	}

	return res, nil
}
