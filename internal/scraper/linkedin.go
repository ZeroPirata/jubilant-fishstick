package scraper

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func (s *Scraper) scrapLinkedIn() (ScraperResult, error) {
	jobId := extractJobIdFromLinkedInUrl(s.Url)
	if jobId == "" {
		return ScraperResult{}, fmt.Errorf("não foi possível extrair o ID")
	}

	urlLink := fmt.Sprintf(
		"https://www.linkedin.com/jobs-guest/jobs/api/jobPosting/%s",
		jobId,
	)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", urlLink, nil)
	if err != nil {
		return ScraperResult{}, err
	}

	resp, err := doRequestWithRetry(client, req, 3)
	if err != nil {
		return ScraperResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ScraperResult{}, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return ScraperResult{}, err
	}

	return s.readHtmlGoquery(doc)
}

func extractJobIdFromLinkedInUrl(link string) string {
	parsed, err := url.Parse(link)
	if err != nil {
		return ""
	}
	return parsed.Query().Get("currentJobId")
}

func (s *Scraper) readHtmlGoquery(doc *goquery.Document) (ScraperResult, error) {
	var result ScraperResult

	result.Company = strings.TrimSpace(doc.Find(".topcard__org-name-link").First().Text())
	result.Title = strings.TrimSpace(doc.Find(".top-card-layout__title").First().Text())

	doc.Find(".description__text--rich li").Each(func(i int, sel *goquery.Selection) {
		text := strings.TrimSpace(sel.Text())
		if text == "" {
			return
		}
		result.Requirements = append(result.Requirements, text)
	})

	var bodyBuilder strings.Builder
	doc.Find(".show-more-less-html__markup").Each(func(i int, sel *goquery.Selection) {
		bodyBuilder.WriteString(sel.Text())
		bodyBuilder.WriteString("\n")
	})
	result.Description = strings.Join(strings.Fields(bodyBuilder.String()), " ")

	return result, nil
}
