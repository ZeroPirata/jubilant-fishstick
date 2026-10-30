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

// Sites que não deixam ler a vaga sem um navegador de verdade (a API recusa acesso
// automatizado). Responde na hora, com a saída: colar a descrição.
var blockedHosts = map[string]string{
	"eightfold": "Eightfold",
	"indeed":    "Indeed",
	"glassdoor": "Glassdoor",
}

// Scrape escolhe o leitor pelo endereço: leitor próprio para os sites e ATS
// conhecidos e o leitor genérico (JSON-LD ou texto da página) para os demais.
func (s *Scraper) Scrape() (BasicScraperResult, error) {
	u, err := url.Parse(s.Url)
	if err != nil {
		return BasicScraperResult{}, err
	}
	host := strings.ToLower(u.Hostname())
	s.Logger.Info("Iniciando scrape", zap.String("url", s.Url), zap.String("domain", host))
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return BasicScraperResult{}, fmt.Errorf("%w: %s", ErrUnsupportedDomain, host)
	}
	site := parts[len(parts)-2]

	if name, ok := blockedHosts[site]; ok {
		return BasicScraperResult{}, fmt.Errorf("%w: %s — %w", ErrUnsupportedDomain, name, ErrNeedsBrowser)
	}

	scraps := map[string]func() (BasicScraperResult, error){
		"linkedin":        s.scrapLinkedIn,
		"amazon":          s.scrapAmazon,
		"geekhunter":      s.scrapGeekHunter,
		"jobright":        s.scrapJobRight,
		"gupy":            s.scrapGupy,
		"programathor":    s.scrapProgramaThor,
		"greenhouse":      func() (BasicScraperResult, error) { return s.scrapGreenhouse(u) },
		"smartrecruiters": func() (BasicScraperResult, error) { return s.scrapSmartRecruiters(u) },
		"workable":        func() (BasicScraperResult, error) { return s.scrapWorkable(u) },
	}
	if scraper, ok := scraps[site]; ok {
		return scraper()
	}
	return s.scrapGeneric()
}
