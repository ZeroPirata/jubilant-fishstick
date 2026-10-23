package scraper

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"go.uber.org/zap"
)

var ErrUnsupportedDomain = errors.New("domínio não suportado")

// ErrEmptyDescription: a página respondeu, mas sem texto da vaga (layout mudou, bloqueio
// por IP, vaga expirada). Sem descrição não há o que mandar para a IA.
var ErrEmptyDescription = errors.New("a página da vaga não retornou descrição (vaga expirada, layout mudou ou acesso bloqueado)")

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
		return BasicScraperResult{}, fmt.Errorf("%w: %s", ErrUnsupportedDomain, hostname)
	}

	return scraper()
}
