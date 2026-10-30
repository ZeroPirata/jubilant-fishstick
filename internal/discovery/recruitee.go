package discovery

import (
	"context"
	"fmt"
	"hackton-treino/internal/scraper"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const recruiteeOffersURL = "https://%s.recruitee.com/api/offers/"

// Recruitee lê as vagas publicadas (Careers Site API) das empresas configuradas no
// painel. Não pede chave; ver docs/fontes.md.
type Recruitee struct {
	Client *http.Client
}

func NewRecruitee() *Recruitee {
	return &Recruitee{Client: &http.Client{Timeout: 30 * time.Second}}
}

func (r *Recruitee) Name() string { return "recruitee" }
func (r *Recruitee) UsesBoards()  {}

type recruiteeOffer struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	CompanyName  string `json:"company_name"`
	CareersURL   string `json:"careers_url"`
	Location     string `json:"location"`
	Country      string `json:"country"`
	Remote       bool   `json:"remote"`
	Hybrid       bool   `json:"hybrid"`
	Status       string `json:"status"`
	PublishedAt  string `json:"published_at"`
	Description  string `json:"description"`
	Requirements string `json:"requirements"`
}

type recruiteeOffers struct {
	Offers []recruiteeOffer `json:"offers"`
}

func (r *Recruitee) fetch(ctx context.Context, token string) (recruiteeOffers, error) {
	var page recruiteeOffers
	// O token vira subdomínio: só o formato validado pelo painel (boardTokenPattern),
	// sem ponto, para não sair de *.recruitee.com.
	if !boardTokenPattern.MatchString(token) || strings.Contains(token, ".") {
		return page, errBoardNotFound
	}
	err := getJSON(ctx, r.Client, fmt.Sprintf(recruiteeOffersURL, token), &page)
	return page, err
}

func (r *Recruitee) List(ctx context.Context, boards []Board, step StepFunc) ListResult {
	return listBoards(ctx, boards, step, func(ctx context.Context, b Board) ([]Listing, error) {
		page, err := r.fetch(ctx, b.Token)
		if err != nil {
			return nil, err
		}
		return parseRecruitee(b, page.Offers), nil
	})
}

// parseRecruitee fica com as vagas publicadas no Brasil ou remotas abertas para cá.
func parseRecruitee(b Board, offers []recruiteeOffer) []Listing {
	out := make([]Listing, 0, len(offers))
	for _, o := range offers {
		if o.CareersURL == "" || (o.Status != "" && o.Status != "published") {
			continue
		}
		workplace, international, keep := ClassifyPlace(joinNonEmpty(", ", o.Location, o.Country), o.Remote)
		if !keep {
			continue
		}
		if o.Hybrid {
			workplace = "hybrid"
		}
		published, _ := time.Parse("2006-01-02 15:04:05 MST", o.PublishedAt)
		out = append(out, Listing{
			Source:        "recruitee",
			ExternalID:    b.Token + "/" + strconv.FormatInt(o.ID, 10),
			Title:         strings.TrimSpace(o.Title),
			Company:       firstNonEmpty(b.Company, o.CompanyName, b.Token),
			Location:      o.Location,
			WorkplaceType: workplace,
			International: international,
			URL:           o.CareersURL,
			Description:   joinNonEmpty("\n\n", scraper.HTMLToText(o.Description), scraper.HTMLToText(o.Requirements)),
			PublishedAt:   published,
		})
	}
	return out
}

// Check confere a empresa: nome, total de vagas e quantas entrariam no catálogo.
func (r *Recruitee) Check(ctx context.Context, token string) BoardCheck {
	page, err := r.fetch(ctx, token)
	if err != nil {
		return checkResult(r.Name(), token, err)
	}
	company := ""
	if len(page.Offers) > 0 {
		company = strings.TrimSpace(page.Offers[0].CompanyName)
	}
	kept := len(parseRecruitee(Board{Token: token}, page.Offers))
	return BoardCheck{Source: r.Name(), Token: token, Company: company, Found: true, Jobs: len(page.Offers), JobsKept: kept}
}
