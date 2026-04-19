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

func (s *Scraper) scrapLinkedIn() (BasicScraperResult, error) {

	urlLink := FormatUrlLinkedinToApi(s.Url)
	if urlLink == "" {
		return BasicScraperResult{}, fmt.Errorf("erro no id da vaga")
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", urlLink, nil)
	if err != nil {
		return BasicScraperResult{}, err
	}

	resp, err := DoRequestWithRetry(client, req, 3)
	if err != nil {
		return BasicScraperResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return BasicScraperResult{}, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return BasicScraperResult{}, err
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

func (s *Scraper) readHtmlGoquery(doc *goquery.Document) (BasicScraperResult, error) {
	var result BasicScraperResult

	result.Company = strings.TrimSpace(doc.Find(".topcard__org-name-link").First().Text())
	result.Title = strings.TrimSpace(doc.Find(".top-card-layout__title").First().Text())

	var bodyBuilder strings.Builder
	doc.Find(".show-more-less-html__markup").Each(func(i int, sel *goquery.Selection) {
		bodyBuilder.WriteString(sel.Text())
		bodyBuilder.WriteString("\n")
	})
	result.BasicDescription = strings.Join(strings.Fields(bodyBuilder.String()), " ")

	return result, nil
}

func FormatUrlLinkedinToApi(url string) string {
	jobId := extractJobIdFromLinkedInUrl(url)
	urlLink := fmt.Sprintf(
		"https://www.linkedin.com/jobs-guest/jobs/api/jobPosting/%s",
		jobId,
	)
	return urlLink
}
