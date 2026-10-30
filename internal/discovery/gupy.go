package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"hackton-treino/internal/scraper"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const gupySearchURL = "https://portal.gupy.io/job-search/term=%s&page=%d"

// Gupy busca no portal de vagas da Gupy (portal.gupy.io). A página de busca é
// renderizada no servidor e traz a lista, com a descrição completa, no __NEXT_DATA__:
// 12 vagas por página.
type Gupy struct {
	Client *http.Client
}

func NewGupy() *Gupy {
	return &Gupy{Client: &http.Client{Timeout: 20 * time.Second}}
}

func (g *Gupy) Name() string { return "gupy" }

type gupySearchPage struct {
	Props struct {
		PageProps struct {
			InitialJobList struct {
				Data       []gupyJob `json:"data"`
				Pagination struct {
					Total  int `json:"total"`
					Limit  int `json:"limit"`
					Offset int `json:"offset"`
				} `json:"pagination"`
			} `json:"initialJobList"`
		} `json:"pageProps"`
	} `json:"props"`
}

type gupyJob struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	CareerPageName      string `json:"careerPageName"`
	PublishedDate       string `json:"publishedDate"`
	ApplicationDeadline string `json:"applicationDeadline"`
	WorkplaceType       string `json:"workplaceType"`
	City                string `json:"city"`
	State               string `json:"state"`
	JobURL              string `json:"jobUrl"`
}

func (g *Gupy) Search(ctx context.Context, keyword string, page int) ([]Listing, bool, error) {
	endpoint := fmt.Sprintf(gupySearchURL, url.PathEscape(keyword), page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("gupy: status %d na busca", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, false, err
	}
	raw, err := scraper.NextData(doc)
	if err != nil {
		return nil, false, fmt.Errorf("gupy: %w", err)
	}
	return parseGupySearch(raw)
}

func parseGupySearch(raw []byte) ([]Listing, bool, error) {
	var page gupySearchPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, false, fmt.Errorf("gupy: JSON da busca inválido: %w", err)
	}

	list := page.Props.PageProps.InitialJobList
	listings := make([]Listing, 0, len(list.Data))
	for _, j := range list.Data {
		if j.ID == 0 || j.JobURL == "" {
			continue
		}
		listings = append(listings, Listing{
			Source:        "gupy",
			ExternalID:    strconv.FormatInt(j.ID, 10),
			Title:         strings.TrimSpace(j.Name),
			Company:       strings.TrimSpace(j.CareerPageName),
			Location:      joinNonEmpty(", ", j.City, j.State),
			WorkplaceType: normalizeWorkplace(j.WorkplaceType),
			URL:           j.JobURL,
			Description:   strings.TrimSpace(j.Description),
			PublishedAt:   parseTime(j.PublishedDate),
			ExpiresAt:     parseTime(j.ApplicationDeadline),
		})
	}

	p := list.Pagination
	hasMore := len(list.Data) > 0 && p.Offset+len(list.Data) < p.Total
	return listings, hasMore, nil
}

// normalizeWorkplace deixa o modo de trabalho em remote | hybrid | onsite.
func normalizeWorkplace(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "on-site" || s == "on_site" || s == "presencial" {
		return "onsite"
	}
	return s
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func joinNonEmpty(sep string, parts ...string) string {
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}
