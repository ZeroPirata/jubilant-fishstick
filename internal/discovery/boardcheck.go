package discovery

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

// BoardCheck é o resultado de conferir uma empresa num ATS.
type BoardCheck struct {
	Source string `json:"source"`
	Token  string `json:"token"`
	// Company é o nome que o ATS informa (a Greenhouse informa; a Lever não).
	Company string `json:"company"`
	Found   bool   `json:"found"`
	Jobs    int    `json:"jobs"`
	// JobsKept: as que entrariam no catálogo (no Brasil ou remotas abertas para cá).
	JobsKept int    `json:"jobs_kept"`
	Error    string `json:"error,omitempty"`
}

// BoardChecker é a fonte de ATS que sabe conferir uma empresa sem gravar nada.
type BoardChecker interface {
	BoardSource
	Check(ctx context.Context, token string) BoardCheck
}

var errBoardNotFound = errors.New("quadro de vagas não encontrado (token errado ou a empresa saiu do ATS)")

func checkResult(source, token string, err error) BoardCheck {
	c := BoardCheck{Source: source, Token: token}
	if err != nil && !errors.Is(err, errBoardNotFound) {
		c.Error = err.Error()
	}
	return c
}

// CheckBoard confere uma empresa num ATS específico.
func (s *Service) CheckBoard(ctx context.Context, source, token string) (BoardCheck, error) {
	checker, ok := s.source(source).(BoardChecker)
	if !ok {
		return BoardCheck{}, fmt.Errorf("%w: %q não usa lista de empresas", ErrUnknownSource, source)
	}
	return checker.Check(ctx, strings.ToLower(strings.TrimSpace(token))), nil
}

// Links públicos dos quadros: boards.greenhouse.io/stone, job-boards.greenhouse.io/stone,
// jobs.lever.co/ciandt.
var boardURL = map[string]*regexp.Regexp{
	"greenhouse": regexp.MustCompile(`^(?:job-)?boards(?:-api)?\.greenhouse\.io$`),
	"lever":      regexp.MustCompile(`^jobs\.lever\.co$`),
}

// FindBoards procura a empresa em todos os ATS. query é o link do quadro de vagas
// (confere só aquele) ou o nome da empresa (tenta os tokens mais prováveis). Devolve
// só o que existe.
func (s *Service) FindBoards(ctx context.Context, query string) []BoardCheck {
	type try struct {
		checker BoardChecker
		token   string
	}
	var tries []try
	if source, token, ok := parseBoardURL(query); ok {
		if c, ok := s.source(source).(BoardChecker); ok {
			tries = append(tries, try{c, token})
		}
	} else {
		for _, src := range s.Sources {
			if c, ok := src.(BoardChecker); ok {
				for _, token := range tokenCandidates(query) {
					tries = append(tries, try{c, token})
				}
			}
		}
	}

	results := make([]BoardCheck, len(tries))
	var wg sync.WaitGroup
	for i, t := range tries {
		wg.Go(func() { results[i] = t.checker.Check(ctx, t.token) })
	}
	wg.Wait()

	found := []BoardCheck{}
	for _, r := range results {
		if r.Found {
			found = append(found, r)
		}
	}
	return found
}

func parseBoardURL(raw string) (source, token string, ok bool) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", false
	}
	for name, host := range boardURL {
		if host.MatchString(strings.ToLower(u.Hostname())) {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			// boards-api.greenhouse.io/v1/boards/<token>/jobs
			if len(parts) >= 3 && parts[0] == "v1" && parts[1] == "boards" {
				parts = parts[2:]
			}
			if t := strings.ToLower(parts[0]); boardTokenPattern.MatchString(t) {
				return name, t, true
			}
		}
	}
	return "", "", false
}

var (
	boardTokenPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,79}$`)
	nonAlnum          = regexp.MustCompile(`[^a-z0-9]+`)
)

// Palavras que não costumam estar no token: "Banco Inter S.A." → inter.
var nameNoise = map[string]bool{
	"sa": true, "s": true, "a": true, "ltda": true, "me": true, "inc": true, "co": true,
	"corp": true, "company": true, "tecnologia": true, "technology": true, "technologies": true,
	"brasil": true, "brazil": true, "br": true, "grupo": true, "group": true, "carreiras": true,
}

// tokenCandidates: "Banco Inter S.A." → bancointer, banco-inter, banco, inter. O
// primeiro e o último nome sozinhos entram porque muitas empresas usam só um deles
// ("Stone Co." → stone, "Banco Inter" → inter).
func tokenCandidates(name string) []string {
	var words []string
	for _, w := range strings.Fields(nonAlnum.ReplaceAllString(foldAccents(strings.ToLower(name)), " ")) {
		if !nameNoise[w] {
			words = append(words, w)
		}
	}
	if len(words) == 0 {
		return nil
	}
	cands := []string{strings.Join(words, ""), strings.Join(words, "-")}
	if len(words) > 1 {
		cands = append(cands, words[0], words[len(words)-1])
	}
	var out []string
	for _, c := range cands {
		if len(c) >= 2 && boardTokenPattern.MatchString(c) && !contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}
