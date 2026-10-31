package discovery

import (
	"context"
	"fmt"
	"hackton-treino/internal/scraper"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const leverPostingsURL = "https://api.lever.co/v0/postings/%s?mode=json"

// Código interno da vaga no começo do título: "[ job -  32032] Senior Developer".
var leverJobCode = regexp.MustCompile(`(?i)^\s*\[\s*job\s*-\s*\d+\s*\]\s*`)

// Lever lê as vagas públicas (Postings API) das empresas configuradas no painel.
type Lever struct {
	Client *http.Client
}

func NewLever() *Lever {
	return &Lever{Client: newBoardClient()}
}

func (l *Lever) Name() string { return "lever" }
func (l *Lever) UsesBoards()  {}

type leverPosting struct {
	ID               string `json:"id"`
	Text             string `json:"text"`
	HostedURL        string `json:"hostedUrl"`
	DescriptionPlain string `json:"descriptionPlain"`
	AdditionalPlain  string `json:"additionalPlain"`
	Country          string `json:"country"`
	WorkplaceType    string `json:"workplaceType"`
	CreatedAt        int64  `json:"createdAt"` // milissegundos
	Categories       struct {
		Location string `json:"location"`
	} `json:"categories"`
	Lists []struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	} `json:"lists"`
}

func (l *Lever) List(ctx context.Context, boards []Board, step StepFunc) ListResult {
	return listBoards(ctx, boards, step, func(ctx context.Context, b Board) ([]Listing, error) {
		var postings []leverPosting
		if err := getJSON(ctx, l.Client, fmt.Sprintf(leverPostingsURL, url.PathEscape(b.Token)), &postings); err != nil {
			return nil, err
		}
		return parseLever(b, postings), nil
	})
}

// parseLever usa o país e o modo que a Lever já informa; sem país BR, a vaga só entra
// se for remota aberta à América Latina ou ao mundo.
func parseLever(b Board, postings []leverPosting) []Listing {
	out := make([]Listing, 0, len(postings))
	for _, p := range postings {
		if p.HostedURL == "" {
			continue
		}
		remote := p.WorkplaceType == "remote"
		workplace, international, keep := ClassifyPlace(p.Categories.Location, remote)
		if strings.EqualFold(p.Country, "BR") {
			international, keep = false, true
		}
		if !keep {
			continue
		}
		if wt := normalizeWorkplace(p.WorkplaceType); wt == "remote" || wt == "hybrid" || wt == "onsite" {
			workplace = wt
		}

		parts := []string{p.DescriptionPlain}
		for _, list := range p.Lists {
			parts = append(parts, list.Text+"\n"+scraper.HTMLToText(list.Content))
		}
		parts = append(parts, p.AdditionalPlain)

		out = append(out, Listing{
			Source:        "lever",
			ExternalID:    b.Token + "/" + p.ID,
			Title:         strings.TrimSpace(leverJobCode.ReplaceAllString(p.Text, "")),
			Company:       firstNonEmpty(b.Company, b.Token),
			Location:      p.Categories.Location,
			WorkplaceType: workplace,
			International: international,
			URL:           p.HostedURL,
			Description:   joinNonEmpty("\n\n", parts...),
			PublishedAt:   unixTime(p.CreatedAt / 1000),
		})
	}
	return out
}

// Check confere a empresa: total de vagas e quantas entrariam no catálogo. A Lever não
// informa o nome da empresa.
func (l *Lever) Check(ctx context.Context, token string) BoardCheck {
	var postings []leverPosting
	if err := getJSON(ctx, l.Client, fmt.Sprintf(leverPostingsURL, url.PathEscape(token)), &postings); err != nil {
		return checkResult(l.Name(), token, err)
	}
	kept := len(parseLever(Board{Token: token}, postings))
	return BoardCheck{Source: l.Name(), Token: token, Found: true, Jobs: len(postings), JobsKept: kept}
}
