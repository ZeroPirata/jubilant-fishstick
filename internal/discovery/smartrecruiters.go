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

const (
	// Só as vagas no Brasil: a Posting API filtra por país.
	smartRecruitersListURL   = "https://api.smartrecruiters.com/v1/companies/%s/postings?country=br&limit=%d&offset=%d"
	smartRecruitersCountURL  = "https://api.smartrecruiters.com/v1/companies/%s/postings?limit=1"
	smartRecruitersDetailURL = "https://api.smartrecruiters.com/v1/companies/%s/postings/%s"

	smartRecruitersPage = 100
	// A lista não traz a descrição: cada vaga é um pedido a mais. O teto e a pausa
	// limitam o custo de uma empresa grande.
	smartRecruitersMaxJobs     = 200
	smartRecruitersDetailDelay = 150 * time.Millisecond
)

// SmartRecruiters lê as vagas públicas (Posting API, sem chave) das empresas
// configuradas no painel; ver docs/fontes.md. Só os endpoints sem autenticação.
type SmartRecruiters struct {
	Client *http.Client
}

func NewSmartRecruiters() *SmartRecruiters {
	return &SmartRecruiters{Client: &http.Client{Timeout: 30 * time.Second}}
}

func (s *SmartRecruiters) Name() string { return "smartrecruiters" }
func (s *SmartRecruiters) UsesBoards()  {}

type smartRecruitersLocation struct {
	City         string `json:"city"`
	Region       string `json:"region"`
	Country      string `json:"country"`
	Remote       bool   `json:"remote"`
	Hybrid       bool   `json:"hybrid"`
	FullLocation string `json:"fullLocation"`
}

type smartRecruitersPosting struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Company struct {
		Name string `json:"name"`
	} `json:"company"`
	ReleasedDate string                  `json:"releasedDate"`
	Location     smartRecruitersLocation `json:"location"`
}

type smartRecruitersList struct {
	TotalFound int                      `json:"totalFound"`
	Content    []smartRecruitersPosting `json:"content"`
}

type smartRecruitersDetail struct {
	smartRecruitersPosting
	PostingURL string `json:"postingUrl"`
	JobAd      struct {
		Sections struct {
			JobDescription        struct{ Text string } `json:"jobDescription"`
			Qualifications        struct{ Text string } `json:"qualifications"`
			AdditionalInformation struct{ Text string } `json:"additionalInformation"`
		} `json:"sections"`
	} `json:"jobAd"`
}

func (s *SmartRecruiters) List(ctx context.Context, boards []Board, step StepFunc) ListResult {
	return listBoards(ctx, boards, step, func(ctx context.Context, b Board) ([]Listing, error) {
		postings, err := s.listBrazil(ctx, b.Token)
		if err != nil {
			return nil, err
		}
		var out []Listing
		for i, p := range postings {
			if i > 0 && !wait(ctx, smartRecruitersDetailDelay) {
				return out, nil
			}
			var d smartRecruitersDetail
			if err := getJSON(ctx, s.Client, fmt.Sprintf(smartRecruitersDetailURL, url.PathEscape(b.Token), url.PathEscape(p.ID)), &d); err != nil {
				// Vaga que saiu do ar entre a lista e o detalhe: segue com as outras.
				continue
			}
			if l, ok := parseSmartRecruiters(b, d); ok {
				out = append(out, l)
			}
		}
		return out, nil
	})
}

// listBrazil pagina a lista de vagas no Brasil, até smartRecruitersMaxJobs.
func (s *SmartRecruiters) listBrazil(ctx context.Context, token string) ([]smartRecruitersPosting, error) {
	var all []smartRecruitersPosting
	for offset := 0; offset < smartRecruitersMaxJobs; offset += smartRecruitersPage {
		var page smartRecruitersList
		if err := getJSON(ctx, s.Client, fmt.Sprintf(smartRecruitersListURL, url.PathEscape(token), smartRecruitersPage, offset), &page); err != nil {
			return nil, err
		}
		all = append(all, page.Content...)
		if len(page.Content) < smartRecruitersPage || offset+len(page.Content) >= page.TotalFound {
			break
		}
	}
	if len(all) > smartRecruitersMaxJobs {
		all = all[:smartRecruitersMaxJobs]
	}
	return all, nil
}

func parseSmartRecruiters(b Board, d smartRecruitersDetail) (Listing, bool) {
	if d.PostingURL == "" {
		return Listing{}, false
	}
	loc := d.Location
	place := firstNonEmpty(loc.FullLocation, joinNonEmpty(", ", loc.City, loc.Region))
	if strings.EqualFold(loc.Country, "br") {
		place = joinNonEmpty(", ", place, "Brasil")
	}
	workplace, international, keep := ClassifyPlace(place, loc.Remote)
	if !keep {
		return Listing{}, false
	}
	if loc.Hybrid {
		workplace = "hybrid"
	}
	sec := d.JobAd.Sections
	return Listing{
		Source:        "smartrecruiters",
		ExternalID:    b.Token + "/" + d.ID,
		Title:         strings.TrimSpace(d.Name),
		Company:       firstNonEmpty(b.Company, d.Company.Name, b.Token),
		Location:      firstNonEmpty(joinNonEmpty(", ", loc.City, loc.Region), place),
		WorkplaceType: workplace,
		International: international,
		URL:           d.PostingURL,
		Description: joinNonEmpty("\n\n", scraper.HTMLToText(sec.JobDescription.Text),
			scraper.HTMLToText(sec.Qualifications.Text), scraper.HTMLToText(sec.AdditionalInformation.Text)),
		PublishedAt: parseTime(d.ReleasedDate),
	}, true
}

// Check confere a empresa sem baixar as descrições. A API responde 200 com lista vazia
// até para identificador que não existe: a empresa só conta como achada se tiver vagas.
func (s *SmartRecruiters) Check(ctx context.Context, token string) BoardCheck {
	var all smartRecruitersList
	if err := getJSON(ctx, s.Client, fmt.Sprintf(smartRecruitersCountURL, url.PathEscape(token)), &all); err != nil {
		return checkResult(s.Name(), token, err)
	}
	if all.TotalFound == 0 {
		return checkResult(s.Name(), token, nil)
	}
	var br smartRecruitersList
	if err := getJSON(ctx, s.Client, fmt.Sprintf(smartRecruitersListURL, url.PathEscape(token), 1, 0), &br); err != nil {
		return checkResult(s.Name(), token, err)
	}
	company := ""
	if len(all.Content) > 0 {
		company = strings.TrimSpace(all.Content[0].Company.Name)
	}
	return BoardCheck{Source: s.Name(), Token: token, Company: company, Found: true, Jobs: all.TotalFound, JobsKept: br.TotalFound}
}
