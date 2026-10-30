package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const githubIssuesJSON = `[
  {"number": 12486, "title": "[Remoto] Desenvolvedor(a) Full Stack Sênior na WeatherWise",
   "body": "Stack: Python, TypeScript e AWS.", "html_url": "https://github.com/backend-br/vagas/issues/12486",
   "created_at": "2026-10-27T15:59:39Z", "labels": [{"name": "PJ"}, {"name": "Python"}, {"name": "Remoto"}]},
  {"number": 12481, "title": "[Joinville/SC] Back-end Developer (Node.js + React)",
   "body": "Vaga para Node.js.", "html_url": "https://github.com/backend-br/vagas/issues/12481",
   "created_at": "2026-10-23T20:20:43Z", "labels": [{"name": "Node.js"}, {"name": "Híbrido"}]},
  {"number": 99, "title": "Atualiza README", "body": "", "html_url": "https://github.com/backend-br/vagas/pull/99",
   "created_at": "2026-10-20T10:00:00Z", "labels": [], "pull_request": {"url": "x"}}
]`

func TestParseGitHubIssues(t *testing.T) {
	var issues []githubIssue
	if err := json.Unmarshal([]byte(githubIssuesJSON), &issues); err != nil {
		t.Fatal(err)
	}
	got := parseGitHubIssues("backend-br/vagas", issues)
	if len(got) != 2 {
		t.Fatalf("esperava 2 vagas (PR fica de fora), veio %d", len(got))
	}

	remote := got[0]
	if remote.Title != "Desenvolvedor(a) Full Stack Sênior na WeatherWise" || remote.WorkplaceType != "remote" || remote.Location != "" {
		t.Errorf("título/modo/local errados: %+v", remote)
	}
	if remote.ExternalID != "backend-br/vagas#12486" || remote.PublishedAt.IsZero() {
		t.Errorf("id/data errados: %+v", remote)
	}
	if !strings.Contains(remote.Description, "Tags: PJ, Python, Remoto") {
		t.Errorf("etiquetas deviam ir para a descrição: %q", remote.Description)
	}

	hybrid := got[1]
	if hybrid.Location != "Joinville/SC" || hybrid.WorkplaceType != "hybrid" || hybrid.Title != "Back-end Developer (Node.js + React)" {
		t.Errorf("local pelo título e modo pela etiqueta: %+v", hybrid)
	}
}

func TestGitHubVagasList(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if strings.Contains(r.URL.Path, "quebrado") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Write([]byte(githubIssuesJSON))
	}))
	defer srv.Close()

	g := &GitHubVagas{Client: srv.Client(), Repos: []string{"backend-br/vagas", "x/quebrado"}}
	g.Client.Transport = rewriteTo(srv.URL)

	var steps []string
	res := g.List(context.Background(), nil, func(done, total int, current string) {
		steps = append(steps, current)
	})
	if len(res.Listings) != 2 || res.OK != 1 || res.Failed != 1 || !strings.Contains(res.LastError, "limite") {
		t.Fatalf("resultado: %+v", res)
	}
	if len(steps) != 3 || steps[0] != "backend-br/vagas" {
		t.Errorf("passos: %v", steps)
	}
}

// rewriteTo manda todo pedido para o servidor de teste, mantendo caminho e query.
type rewriteTo string

func (base rewriteTo) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.URL.Scheme = "http"
	r2.URL.Host = strings.TrimPrefix(string(base), "http://")
	return http.DefaultTransport.RoundTrip(r2)
}
