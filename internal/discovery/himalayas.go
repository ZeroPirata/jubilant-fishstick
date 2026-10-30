package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hackton-treino/internal/scraper"
	"net/http"
	"os"
	"strings"
	"time"
)

// Sem termo de busca, a API devolve todas as vagas que aceitam candidatos no Brasil.
const himalayasListURL = "https://himalayas.app/jobs/api/search?country=Brazil&page=%d"

const (
	// ~6 mil vagas a 20 por página: ~300 pedidos. Sem chave, o limite é compartilhado
	// com todo mundo que usa a API (com chave são 60 por minuto): uma página por
	// segundo fica abaixo disso e leva ~5 min, em paralelo com os outros sites.
	himalayasPageDelay = 1100 * time.Millisecond
	himalayasMaxPages  = 600
)

// Himalayas lê as vagas remotas da API pública da Himalayas que aceitam candidatos no
// Brasil (country=Brazil), de empresas de fora e também brasileiras.
//
// Termos da API (himalayas.app/terms#api): o botão de candidatura precisa usar o
// applicationLink, a Himalayas aparece como fonte com link para a vaga, as vagas não
// são repassadas a terceiros e saem quando expiram. HIMALAYAS_API_KEY, se definida,
// dá um limite próprio em vez do compartilhado entre todos que usam sem chave.
type Himalayas struct {
	Client *http.Client
	APIKey string
}

func NewHimalayas() *Himalayas {
	return &Himalayas{
		Client: &http.Client{Timeout: 20 * time.Second},
		APIKey: os.Getenv("HIMALAYAS_API_KEY"),
	}
}

func (h *Himalayas) Name() string { return "himalayas" }

type himalayasPage struct {
	Offset     int            `json:"offset"`
	TotalCount int            `json:"totalCount"`
	Jobs       []himalayasJob `json:"jobs"`
}

type himalayasJob struct {
	GUID                 string   `json:"guid"`
	Title                string   `json:"title"`
	CompanyName          string   `json:"companyName"`
	Description          string   `json:"description"`
	ApplicationLink      string   `json:"applicationLink"`
	LocationRestrictions []string `json:"locationRestrictions"`
	Categories           []string `json:"categories"`
	Seniority            []string `json:"seniority"`
	PubDate              int64    `json:"pubDate"`
	ExpiryDate           int64    `json:"expiryDate"`
}

// List lê página por página até a última. Uma página com erro encerra a leitura: o
// que veio até ali é gravado e o resto continua valendo pela última varredura boa.
func (h *Himalayas) List(ctx context.Context, _ []Board, step StepFunc) ListResult {
	var res ListResult
	totalPages := 0
	for page := 1; page <= himalayasMaxPages; page++ {
		if page > 1 && !wait(ctx, himalayasPageDelay) {
			res.Failed++
			res.LastError = ctx.Err().Error()
			return res
		}
		label := fmt.Sprintf("página %d", page)
		step(page-1, totalPages, label)
		p, err := h.fetchPage(ctx, page, func(d time.Duration) {
			step(page-1, totalPages, fmt.Sprintf("%s · limite do site, aguardando %s", label, d.Round(time.Second)))
		})
		if err != nil {
			res.Failed++
			res.LastError = fmt.Sprintf("página %d: %v", page, err)
			return res
		}
		listings, hasMore := parseHimalayas(p)
		res.Listings = append(res.Listings, listings...)
		if len(p.Jobs) > 0 {
			totalPages = min((p.TotalCount+len(p.Jobs)-1)/len(p.Jobs), himalayasMaxPages)
		}
		if !hasMore {
			break
		}
	}
	res.OK = 1
	step(totalPages, totalPages, "")
	return res
}

func (h *Himalayas) fetchPage(ctx context.Context, page int, onWait func(time.Duration)) (himalayasPage, error) {
	var p himalayasPage
	header := http.Header{"Accept": {"application/json"}}
	if h.APIKey != "" {
		header.Set("Authorization", "Bearer "+h.APIKey)
	}
	resp, err := getWithRetry(ctx, h.Client, fmt.Sprintf(himalayasListURL, page), header, onWait)
	if errors.Is(err, errRateLimited) {
		return p, fmt.Errorf("himalayas: limite de pedidos atingido mesmo esperando (defina HIMALAYAS_API_KEY)")
	}
	if err != nil {
		return p, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return p, fmt.Errorf("himalayas: status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return p, fmt.Errorf("himalayas: JSON inválido: %w", err)
	}
	return p, nil
}

func parseHimalayas(p himalayasPage) ([]Listing, bool) {
	listings := make([]Listing, 0, len(p.Jobs))
	for _, j := range p.Jobs {
		if j.GUID == "" || j.ApplicationLink == "" {
			continue
		}
		description := scraper.HTMLToText(j.Description)
		if len(j.Categories) > 0 {
			description += "\n\nCategorias: " + strings.ReplaceAll(strings.Join(j.Categories, ", "), "-", " ")
		}
		location := himalayasLocation(j.LocationRestrictions)
		listings = append(listings, Listing{
			Source:        "himalayas",
			ExternalID:    j.GUID,
			Title:         strings.TrimSpace(j.Title),
			Company:       strings.TrimSpace(j.CompanyName),
			Location:      location,
			WorkplaceType: "remote",
			// Vaga só para o Brasil é, na prática, remota nacional.
			International: location != onlyBrazil,
			URL:           j.ApplicationLink,
			Description:   description,
			Seniority:     ClassifySeniority(j.Seniority...),
			PublishedAt:   unixTime(j.PubDate),
			ExpiresAt:     unixTime(j.ExpiryDate),
		})
	}
	hasMore := len(p.Jobs) > 0 && p.Offset+len(p.Jobs) < p.TotalCount
	return listings, hasMore
}

// himalayasLocation resume os países aceitos: lista curta por extenso, longa pela contagem.
func himalayasLocation(countries []string) string {
	switch {
	case len(countries) == 0:
		return "qualquer país"
	case len(countries) == 1 && countries[0] == "Brazil":
		return onlyBrazil
	case len(countries) <= 3:
		return strings.Join(countries, ", ")
	default:
		return fmt.Sprintf("%d países, incluindo Brasil", len(countries))
	}
}

func unixTime(sec int64) time.Time {
	if sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}
