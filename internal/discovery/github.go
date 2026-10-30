package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const githubIssuesURL = "https://api.github.com/repos/%s/issues?state=open&per_page=100&page=%d"

// Repositórios da comunidade em que cada issue aberta é uma vaga. Os de React e QA
// estão parados há anos e ficaram de fora.
var githubVagasRepos = []string{"backend-br/vagas", "frontendbr/vagas"}

// GitHubVagas lê as issues abertas dos repositórios de vagas (backend-br, frontendbr).
// São poucas dezenas por repositório: a lista inteira é baixada a cada varredura.
// GITHUB_TOKEN, se definido, sobe o limite de 60 para 5000 pedidos por hora.
type GitHubVagas struct {
	Client *http.Client
	Repos  []string
	Token  string
}

func NewGitHubVagas() *GitHubVagas {
	return &GitHubVagas{
		Client: &http.Client{Timeout: 20 * time.Second},
		Repos:  githubVagasRepos,
		Token:  os.Getenv("GITHUB_TOKEN"),
	}
}

func (g *GitHubVagas) Name() string { return "github" }

type githubIssue struct {
	Number      int64  `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	CreatedAt   string `json:"created_at"`
	PullRequest *struct {
		URL string `json:"url"`
	} `json:"pull_request"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// List lê um repositório por passo.
func (g *GitHubVagas) List(ctx context.Context, _ []Board, step StepFunc) ListResult {
	var res ListResult
	for i, repo := range g.Repos {
		step(i, len(g.Repos), repo)
		listings, err := g.fetchRepo(ctx, repo)
		if err != nil {
			res.Failed++
			res.LastError = err.Error()
			continue
		}
		res.OK++
		res.Listings = append(res.Listings, listings...)
	}
	step(len(g.Repos), len(g.Repos), "")
	return res
}

func (g *GitHubVagas) fetchRepo(ctx context.Context, repo string) ([]Listing, error) {
	var out []Listing
	for page := 1; page <= 10; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(githubIssuesURL, repo, page), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if g.Token != "" {
			req.Header.Set("Authorization", "Bearer "+g.Token)
		}
		resp, err := g.Client.Do(req)
		if err != nil {
			return nil, err
		}
		var issues []githubIssue
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
				return nil, fmt.Errorf("github: limite de pedidos atingido em %s (defina GITHUB_TOKEN)", repo)
			}
			return nil, fmt.Errorf("github: status %d em %s", resp.StatusCode, repo)
		}
		err = json.NewDecoder(resp.Body).Decode(&issues)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("github: JSON inválido em %s: %w", repo, err)
		}
		out = append(out, parseGitHubIssues(repo, issues)...)
		if len(issues) < 100 {
			break
		}
	}
	return out, nil
}

// Grupos entre colchetes no começo do título: "[Remoto] [São Paulo/SP] Dev Go".
var titleTags = regexp.MustCompile(`^\s*[\[(]([^\])]*)[\])]\s*`)

func parseGitHubIssues(repo string, issues []githubIssue) []Listing {
	out := make([]Listing, 0, len(issues))
	for _, is := range issues {
		if is.PullRequest != nil || is.HTMLURL == "" {
			continue
		}
		title := is.Title
		var places []string
		workplace := ""
		for {
			m := titleTags.FindStringSubmatch(title)
			if m == nil {
				break
			}
			title = title[len(m[0]):]
			if wt := workplaceFromWord(m[1]); wt != "" {
				workplace = wt
			} else if tag := strings.TrimSpace(m[1]); tag != "" {
				places = append(places, tag)
			}
		}

		labels := make([]string, 0, len(is.Labels))
		for _, l := range is.Labels {
			labels = append(labels, l.Name)
			if wt := workplaceFromWord(l.Name); wt != "" && workplace == "" {
				workplace = wt
			}
		}

		description := strings.TrimSpace(is.Body)
		if len(labels) > 0 {
			description += "\n\nTags: " + strings.Join(labels, ", ")
		}
		out = append(out, Listing{
			Source:        "github",
			ExternalID:    fmt.Sprintf("%s#%d", repo, is.Number),
			Title:         strings.TrimSpace(title),
			Location:      strings.Join(places, ", "),
			WorkplaceType: workplace,
			URL:           is.HTMLURL,
			Description:   description,
			PublishedAt:   parseTime(is.CreatedAt),
		})
	}
	return out
}

// workplaceFromWord reconhece o modo de trabalho em etiquetas como "Remoto" ou "Híbrido".
func workplaceFromWord(s string) string {
	switch foldAccents(strings.ToLower(strings.TrimSpace(s))) {
	case "remoto", "remote", "100% remoto", "home office":
		return "remote"
	case "hibrido", "hybrid":
		return "hybrid"
	case "presencial", "on-site", "onsite":
		return "onsite"
	}
	return ""
}
