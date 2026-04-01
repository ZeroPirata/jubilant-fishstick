package scraper

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func (s *Scraper) scrapAmazon() (ScraperResult, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
	}

	req, err := http.NewRequest("GET", s.Url, nil)
	if err != nil {
		return ScraperResult{}, fmt.Errorf("erro ao criar requisição: %v", err)
	}

	resp, err := doRequestWithRetry(client, req, 3)
	if err != nil {
		return ScraperResult{}, fmt.Errorf("falha na rede ao acessar Amazon: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ScraperResult{}, fmt.Errorf("Amazon jobs retornou erro: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return ScraperResult{}, fmt.Errorf("erro ao processar HTML da Amazon: %v", err)
	}

	return s.readAmazonHtml(doc)
}

func (s *Scraper) readAmazonHtml(doc *goquery.Document) (ScraperResult, error) {
	var res ScraperResult

	res.Title = strings.TrimSpace(doc.Find("h1.title").First().Text())
	res.Company = "Amazon"
	metaText := doc.Find(".details-line .meta").First().Text()
	res.Industry = strings.TrimSpace(metaText)
	res.Location = strings.TrimSpace(doc.Find(".location-icon .association-content li").First().Text())

	doc.Find(".section h2").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(strings.ToLower(sel.Text()), "basic qualifications") {
			qualificationsText := sel.Next().Text()
			lines := strings.SplitSeq(qualificationsText, "-")
			for line := range lines {
				cleanLine := strings.TrimSpace(line)
				if cleanLine != "" {
					res.Requirements = append(res.Requirements, cleanLine)
				}
			}
		}
	})

	doc.Find(".section h2").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(strings.ToLower(sel.Text()), "description") {
			res.Description = strings.TrimSpace(sel.Next().Text())
		}
	})

	if res.Title == "" {
		res.Title = strings.TrimSpace(doc.Find("h1").First().Text())
	}

	if res.Title == "" {
		return ScraperResult{}, fmt.Errorf("falha ao parsear Amazon: título não encontrado")
	}

	return res, nil
}
