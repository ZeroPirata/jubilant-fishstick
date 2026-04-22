package scraper

import (
	"fmt"
	"net/url"
	"strings"

	"go.uber.org/zap"
)

type Scraper struct {
	Url    string
	Logger *zap.Logger
}

type BasicScraperResult struct {
	Title            string
	Company          string
	BasicDescription string
}

type NLScraperResult struct {
	Stack                 []string `json:"Stack"`
	Requirements          []string `json:"Requirements"`
	CompressedDescription string   `json:"Description"`
}

type ResultScraper struct {
	BasicScraperResult
	NLScraperResult
}

func NewScraper(url string, logger *zap.Logger) Scraper {
	return Scraper{
		Url:    url,
		Logger: logger,
	}
}

func (s *Scraper) Scrape() (BasicScraperResult, error) {
	domain, err := url.Parse(s.Url)
	if err != nil {
		return BasicScraperResult{}, err
	}
	s.Logger.Info("Iniciando scrape", zap.String("url", s.Url), zap.String("domain", domain.Hostname()))
	parts := strings.Split(domain.Hostname(), ".")
	hostname := parts[len(parts)-2]
	scraps := map[string]func() (BasicScraperResult, error){
		"linkedin":   s.scrapLinkedIn,
		"amazon":     s.scrapAmazon,
		"geekhunter": s.scrapGeekHunter,
		"jobright":   s.scrapJobRight,
	}

	scraper, ok := scraps[hostname]
	if !ok {
		return BasicScraperResult{}, fmt.Errorf("domínio não suportado: %s", hostname)
	}

	return scraper()
}
