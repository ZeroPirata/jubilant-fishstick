package discovery

import (
	"context"
	"fmt"
	"hackton-treino/internal/scraper"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// O endpoint documentado (www.workable.com/api/accounts/<conta>) redireciona para este.
const workableJobsURL = "https://apply.workable.com/api/v1/widget/accounts/%s?details=true"

// Workable lê as vagas públicas das contas configuradas no painel. Não pede chave;
// ver docs/fontes.md.
type Workable struct {
	Client *http.Client
}

func NewWorkable() *Workable {
	return &Workable{Client: newBoardClient()}
}

func (w *Workable) Name() string { return "workable" }
func (w *Workable) UsesBoards()  {}

type workableJob struct {
	Title       string `json:"title"`
	Shortcode   string `json:"shortcode"`
	Telecommute bool   `json:"telecommuting"`
	Shortlink   string `json:"shortlink"`
	URL         string `json:"url"`
	PublishedOn string `json:"published_on"`
	Country     string `json:"country"`
	City        string `json:"city"`
	State       string `json:"state"`
	Description string `json:"description"`
	Locations   []struct {
		Country string `json:"country"`
		City    string `json:"city"`
		Region  string `json:"region"`
		Hidden  bool   `json:"hidden"`
	} `json:"locations"`
}

type workableAccount struct {
	Name string        `json:"name"`
	Jobs []workableJob `json:"jobs"`
}

func (w *Workable) fetch(ctx context.Context, token string) (workableAccount, error) {
	var acc workableAccount
	err := getJSON(ctx, w.Client, fmt.Sprintf(workableJobsURL, url.PathEscape(token)), &acc)
	return acc, err
}

func (w *Workable) List(ctx context.Context, boards []Board, step StepFunc) ListResult {
	return listBoards(ctx, boards, step, func(ctx context.Context, b Board) ([]Listing, error) {
		acc, err := w.fetch(ctx, b.Token)
		if err != nil {
			return nil, err
		}
		return parseWorkable(b, acc), nil
	})
}

// parseWorkable fica com as vagas no Brasil ou remotas abertas para cá.
func parseWorkable(b Board, acc workableAccount) []Listing {
	out := make([]Listing, 0, len(acc.Jobs))
	for _, j := range acc.Jobs {
		link := firstNonEmpty(j.Shortlink, j.URL)
		if link == "" {
			continue
		}
		places := []string{joinNonEmpty(", ", j.City, j.State, j.Country)}
		for _, l := range j.Locations {
			if !l.Hidden {
				places = append(places, joinNonEmpty(", ", l.City, l.Region, l.Country))
			}
		}
		workplace, international, keep := ClassifyPlace(joinNonEmpty("; ", places...), j.Telecommute)
		if !keep {
			continue
		}
		published, _ := time.Parse("2006-01-02", j.PublishedOn)
		out = append(out, Listing{
			Source:        "workable",
			ExternalID:    b.Token + "/" + j.Shortcode,
			Title:         strings.TrimSpace(j.Title),
			Company:       firstNonEmpty(b.Company, acc.Name, b.Token),
			Location:      places[0],
			WorkplaceType: workplace,
			International: international,
			URL:           link,
			Description:   scraper.HTMLToText(j.Description),
			PublishedAt:   published,
		})
	}
	return out
}

// Check confere a conta: nome, total de vagas e quantas entrariam no catálogo.
func (w *Workable) Check(ctx context.Context, token string) BoardCheck {
	acc, err := w.fetch(ctx, token)
	if err != nil {
		return checkResult(w.Name(), token, err)
	}
	kept := len(parseWorkable(Board{Token: token}, acc))
	return BoardCheck{Source: w.Name(), Token: token, Company: strings.TrimSpace(acc.Name), Found: true, Jobs: len(acc.Jobs), JobsKept: kept}
}
