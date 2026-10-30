package discovery

import (
	"context"
	"fmt"
	"hackton-treino/internal/scraper"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	greenhouseJobsURL  = "https://boards-api.greenhouse.io/v1/boards/%s/jobs?content=true"
	greenhouseBoardURL = "https://boards-api.greenhouse.io/v1/boards/%s"
	// Sem a descrição: para conferir a empresa basta o local de cada vaga.
	greenhouseJobsLightURL = "https://boards-api.greenhouse.io/v1/boards/%s/jobs"
)

// Greenhouse lê os quadros de vagas públicos (Job Board API) das empresas configuradas
// no painel. A API é feita para as empresas exibirem as vagas em outros sites e não
// pede chave.
type Greenhouse struct {
	Client *http.Client
}

func NewGreenhouse() *Greenhouse {
	return &Greenhouse{Client: &http.Client{Timeout: 30 * time.Second}}
}

func (g *Greenhouse) Name() string { return "greenhouse" }
func (g *Greenhouse) UsesBoards()  {}

type greenhouseJob struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	AbsoluteURL string `json:"absolute_url"`
	CompanyName string `json:"company_name"`
	Content     string `json:"content"`
	FirstPub    string `json:"first_published"`
	UpdatedAt   string `json:"updated_at"`
	Location    struct {
		Name string `json:"name"`
	} `json:"location"`
}

func (g *Greenhouse) List(ctx context.Context, boards []Board, step StepFunc) ListResult {
	return listBoards(ctx, boards, step, func(ctx context.Context, b Board) ([]Listing, error) {
		var page struct {
			Jobs []greenhouseJob `json:"jobs"`
		}
		if err := getJSON(ctx, g.Client, fmt.Sprintf(greenhouseJobsURL, url.PathEscape(b.Token)), &page); err != nil {
			return nil, err
		}
		return parseGreenhouse(b, page.Jobs), nil
	})
}

// parseGreenhouse fica só com as vagas no Brasil ou remotas abertas para cá.
func parseGreenhouse(b Board, jobs []greenhouseJob) []Listing {
	out := make([]Listing, 0, len(jobs))
	for _, j := range jobs {
		workplace, international, keep := ClassifyPlace(j.Location.Name, false)
		if !keep || j.AbsoluteURL == "" {
			continue
		}
		published := parseTime(j.FirstPub)
		if published.IsZero() {
			published = parseTime(j.UpdatedAt)
		}
		out = append(out, Listing{
			Source:        "greenhouse",
			ExternalID:    b.Token + "/" + strconv.FormatInt(j.ID, 10),
			Title:         strings.TrimSpace(j.Title),
			Company:       firstNonEmpty(b.Company, j.CompanyName, b.Token),
			Location:      j.Location.Name,
			WorkplaceType: workplace,
			International: international,
			URL:           j.AbsoluteURL,
			// O content vem com o HTML escapado (&lt;p&gt;).
			Description: scraper.HTMLToText(html.UnescapeString(j.Content)),
			PublishedAt: published,
		})
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// Check confere a empresa: nome no ATS, total de vagas e quantas entrariam no catálogo.
func (g *Greenhouse) Check(ctx context.Context, token string) BoardCheck {
	var board struct {
		Name string `json:"name"`
	}
	if err := getJSON(ctx, g.Client, fmt.Sprintf(greenhouseBoardURL, url.PathEscape(token)), &board); err != nil {
		return checkResult(g.Name(), token, err)
	}
	var page struct {
		Jobs []greenhouseJob `json:"jobs"`
	}
	if err := getJSON(ctx, g.Client, fmt.Sprintf(greenhouseJobsLightURL, url.PathEscape(token)), &page); err != nil {
		return checkResult(g.Name(), token, err)
	}
	kept := 0
	for _, j := range page.Jobs {
		if _, _, keep := ClassifyPlace(j.Location.Name, false); keep {
			kept++
		}
	}
	return BoardCheck{Source: g.Name(), Token: token, Company: strings.TrimSpace(board.Name), Found: true, Jobs: len(page.Jobs), JobsKept: kept}
}
