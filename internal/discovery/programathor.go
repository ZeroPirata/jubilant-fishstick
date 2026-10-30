package discovery

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	programathorBase    = "https://programathor.com.br"
	programathorPageURL = programathorBase + "/jobs/page/%d"
	// As vagas abertas vêm primeiro; depois vem o histórico de vencidas (mais de mil
	// páginas). A leitura para na primeira página só de vencidas.
	programathorMaxPages = 40
	programathorDelay    = time.Second
)

// ProgramaThor lê a listagem de vagas de tecnologia do programathor.com.br (o
// robots.txt libera /jobs). O card não traz a descrição, mas traz as tecnologias da
// vaga, que é o que o casamento com as skills usa; a descrição completa é lida só
// quando o usuário manda a vaga para a pipeline.
type ProgramaThor struct {
	Client *http.Client
}

func NewProgramaThor() *ProgramaThor {
	return &ProgramaThor{Client: &http.Client{Timeout: 20 * time.Second}}
}

func (p *ProgramaThor) Name() string { return "programathor" }

func (p *ProgramaThor) List(ctx context.Context, _ []Board, step StepFunc) ListResult {
	var res ListResult
	for page := 1; page <= programathorMaxPages; page++ {
		if page > 1 && !wait(ctx, programathorDelay) {
			res.Failed++
			res.LastError = ctx.Err().Error()
			return res
		}
		step(page-1, 0, fmt.Sprintf("página %d", page))
		doc, err := p.fetch(ctx, fmt.Sprintf(programathorPageURL, page))
		if err != nil {
			res.Failed++
			res.LastError = fmt.Sprintf("página %d: %v", page, err)
			return res
		}
		listings, open := parseProgramaThor(doc)
		res.Listings = append(res.Listings, listings...)
		if open == 0 {
			break
		}
	}
	res.OK = 1
	return res
}

func (p *ProgramaThor) fetch(ctx context.Context, url string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")
	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return goquery.NewDocumentFromReader(resp.Body)
}

var programathorJobPath = regexp.MustCompile(`^/jobs/(\d+)-`)

// Local do card: "Remoto", "Vinhedo (Presencial)", "São Paulo (Híbrido)".
var placeWithMode = regexp.MustCompile(`^(.*?)\s*\(([^)]*)\)\s*$`)

// parseProgramaThor devolve as vagas abertas da página e quantas eram (as vencidas
// ficam de fora).
func parseProgramaThor(doc *goquery.Document) ([]Listing, int) {
	var out []Listing
	doc.Find(".cell-list a[href^='/jobs/']").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		m := programathorJobPath.FindStringSubmatch(href)
		h3 := a.Find("h3").First()
		if m == nil || h3.Length() == 0 {
			return
		}
		if strings.Contains(h3.Text(), "Vencida") {
			return
		}
		title := strings.TrimSpace(h3.Text())

		info := map[string]string{}
		a.Find(".cell-list-content-icon span").Each(func(_ int, s *goquery.Selection) {
			class, _ := s.Find("i").Attr("class")
			info[iconKey(class)] = strings.Join(strings.Fields(s.Text()), " ")
		})
		var tags []string
		a.Find(".tag-list").Each(func(_ int, s *goquery.Selection) {
			if t := strings.TrimSpace(s.Text()); t != "" {
				tags = append(tags, t)
			}
		})

		location, workplace := info["place"], ""
		if mm := placeWithMode.FindStringSubmatch(location); mm != nil {
			location, workplace = strings.TrimSpace(mm[1]), workplaceFromWord(mm[2])
		} else if wt := workplaceFromWord(location); wt != "" {
			location, workplace = "", wt
		}

		description := joinNonEmpty("\n",
			labelled("Nível", info["level"]),
			labelled("Contratação", info["contract"]),
			labelled("Empresa", info["size"]),
			labelled("Tecnologias", strings.Join(tags, ", ")),
		)
		out = append(out, Listing{
			Source:        "programathor",
			ExternalID:    m[1],
			Title:         title,
			Company:       info["company"],
			Location:      location,
			WorkplaceType: workplace,
			URL:           programathorBase + href,
			Description:   description,
			Seniority:     ClassifySeniority(info["level"]),
		})
	})
	return out, len(out)
}

// iconKey traduz o ícone do card no campo que ele representa.
func iconKey(class string) string {
	switch {
	case strings.Contains(class, "fa-briefcase"):
		return "company"
	case strings.Contains(class, "fa-map-marker"):
		return "place"
	case strings.Contains(class, "fa-building"):
		return "size"
	case strings.Contains(class, "fa-chart-bar"):
		return "level"
	case strings.Contains(class, "fa-file-alt"):
		return "contract"
	}
	return class
}

func labelled(label, value string) string {
	if value == "" {
		return ""
	}
	return label + ": " + value
}
