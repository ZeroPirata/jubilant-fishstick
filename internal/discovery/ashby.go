package discovery

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const ashbyJobsURL = "https://api.ashbyhq.com/posting-api/job-board/%s"

// Ashby lê o quadro de vagas público (Public Job Posting API) das empresas
// configuradas no painel. A API não pede chave; ver docs/fontes.md.
type Ashby struct {
	Client *http.Client
}

func NewAshby() *Ashby {
	return &Ashby{Client: &http.Client{Timeout: 30 * time.Second}}
}

func (a *Ashby) Name() string { return "ashby" }
func (a *Ashby) UsesBoards()  {}

type ashbyAddress struct {
	PostalAddress struct {
		AddressLocality string `json:"addressLocality"`
		AddressRegion   string `json:"addressRegion"`
		AddressCountry  string `json:"addressCountry"`
	} `json:"postalAddress"`
}

func (a ashbyAddress) String() string {
	p := a.PostalAddress
	return joinNonEmpty(", ", p.AddressLocality, p.AddressRegion, p.AddressCountry)
}

type ashbyJob struct {
	ID                 string       `json:"id"`
	Title              string       `json:"title"`
	Location           string       `json:"location"`
	Address            ashbyAddress `json:"address"`
	SecondaryLocations []struct {
		Location string       `json:"location"`
		Address  ashbyAddress `json:"address"`
	} `json:"secondaryLocations"`
	PublishedAt      string `json:"publishedAt"`
	IsListed         bool   `json:"isListed"`
	IsRemote         bool   `json:"isRemote"`
	WorkplaceType    string `json:"workplaceType"`
	JobURL           string `json:"jobUrl"`
	DescriptionPlain string `json:"descriptionPlain"`
}

type ashbyBoard struct {
	Jobs []ashbyJob `json:"jobs"`
}

func (a *Ashby) fetch(ctx context.Context, token string) (ashbyBoard, error) {
	var board ashbyBoard
	err := getJSON(ctx, a.Client, fmt.Sprintf(ashbyJobsURL, url.PathEscape(token)), &board)
	return board, err
}

func (a *Ashby) List(ctx context.Context, boards []Board, step StepFunc) ListResult {
	return listBoards(ctx, boards, step, func(ctx context.Context, b Board) ([]Listing, error) {
		board, err := a.fetch(ctx, b.Token)
		if err != nil {
			return nil, err
		}
		return parseAshby(b, board.Jobs), nil
	})
}

// parseAshby fica com as vagas no Brasil ou remotas abertas para cá. Vaga com
// isListed=false é só para quem tem o link (a documentação pede para não listá-la).
func parseAshby(b Board, jobs []ashbyJob) []Listing {
	out := make([]Listing, 0, len(jobs))
	for _, j := range jobs {
		if !j.IsListed || j.JobURL == "" {
			continue
		}
		places := []string{j.Location, j.Address.String()}
		for _, s := range j.SecondaryLocations {
			places = append(places, s.Location, s.Address.String())
		}
		place := joinNonEmpty("; ", places...)
		// isRemote vem true até em vaga híbrida: vale o workplaceType quando existe.
		remoteHint := j.IsRemote && (j.WorkplaceType == "" || strings.EqualFold(j.WorkplaceType, "remote"))
		workplace, international, keep := ClassifyPlace(place, remoteHint)
		if !keep {
			continue
		}
		if wt := normalizeWorkplace(j.WorkplaceType); wt == "remote" || wt == "hybrid" || wt == "onsite" {
			workplace = wt
		}
		out = append(out, Listing{
			Source:        "ashby",
			ExternalID:    b.Token + "/" + j.ID,
			Title:         strings.TrimSpace(j.Title),
			Company:       firstNonEmpty(b.Company, b.Token),
			Location:      firstNonEmpty(j.Location, j.Address.String()),
			WorkplaceType: workplace,
			International: international,
			URL:           j.JobURL,
			Description:   strings.TrimSpace(j.DescriptionPlain),
			PublishedAt:   parseTime(j.PublishedAt),
		})
	}
	return out
}

// Check confere a empresa: total de vagas e quantas entrariam no catálogo. O Ashby não
// informa o nome da empresa.
func (a *Ashby) Check(ctx context.Context, token string) BoardCheck {
	board, err := a.fetch(ctx, token)
	if err != nil {
		return checkResult(a.Name(), token, err)
	}
	kept := len(parseAshby(Board{Token: token}, board.Jobs))
	return BoardCheck{Source: a.Name(), Token: token, Found: true, Jobs: len(board.Jobs), JobsKept: kept}
}
