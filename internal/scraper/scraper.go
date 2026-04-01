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

type ScraperResult struct {
	Title        string
	Company      string
	Location     string
	Description  string
	Requirements []string
	CompanySize  string
	Industry     string
	Stack        []string
}

func NewScraper(url string, logger *zap.Logger) Scraper {
	return Scraper{
		Url:    url,
		Logger: logger,
	}
}

func (s *Scraper) Scrape() (ScraperResult, error) {
	domain, err := url.Parse(s.Url)
	if err != nil {
		return ScraperResult{}, err
	}

	s.Logger.Info("Iniciando scrape", zap.String("url", s.Url), zap.String("domain", domain.Hostname()))
	hostname := strings.Split(domain.Hostname(), ".")[1]

	switch hostname {
	case "linkedin":
		return s.scrapLinkedIn()
	case "amazon":
		return s.scrapAmazon()
	case "geekhunter":
		return s.scrapGeekHunter()
	default:
		return ScraperResult{}, fmt.Errorf("domínio não suportado: %s", hostname)
	}
}
