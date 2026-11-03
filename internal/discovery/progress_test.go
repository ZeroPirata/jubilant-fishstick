package discovery

import (
	"context"
	"errors"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"
	discoveryRepo "hackton-treino/internal/repository/discovery"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// fakeRepo implementa só o que a varredura usa; o resto do Repository entra via
// interface embutida (nil) e quebraria o teste se fosse chamado. Os sites rodam em
// paralelo, então tudo passa pelo mutex.
type fakeRepo struct {
	discoveryRepo.Repository
	mu       sync.Mutex
	run      db.DiscoveryInsertRunParams
	sources  []db.DiscoveryInsertRunSourceParams
	finish   db.DiscoveryFinishRunParams
	jobs     map[string]bool
	settings db.DiscoverySelectSettingsRow
	userKws  []string
	techs    []string
	cleaned  int
	upserts  []db.DiscoveryUpsertJobParams
}

var runID = pgtype.UUID{Bytes: [16]byte{1}, Valid: true}

func (f *fakeRepo) InsertRun(_ context.Context, p db.DiscoveryInsertRunParams) (pgtype.UUID, *repository.RepositoryError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.run = p
	return runID, nil
}
func (f *fakeRepo) UpsertJob(_ context.Context, p db.DiscoveryUpsertJobParams) (db.DiscoveryUpsertJobRow, *repository.RepositoryError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.jobs == nil {
		f.jobs = map[string]bool{}
	}
	f.upserts = append(f.upserts, p)
	key := p.Source + "|" + p.ExternalID
	inserted := !f.jobs[key]
	f.jobs[key] = true
	return db.DiscoveryUpsertJobRow{ID: pgtype.UUID{Bytes: [16]byte{2}, Valid: true}, Inserted: inserted}, nil
}
func (f *fakeRepo) CleanIndexPending(context.Context) *repository.RepositoryError {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleaned++
	return nil
}

func (f *fakeRepo) LinkKeyword(context.Context, db.DiscoveryLinkKeywordParams) *repository.RepositoryError {
	return nil
}
func (f *fakeRepo) DeleteStale(context.Context, db.DiscoveryDeleteStaleParams) ([]db.DiscoveryDeleteStaleRow, *repository.RepositoryError) {
	return nil, nil
}
func (f *fakeRepo) PruneAlertFeedback(context.Context, time.Time) *repository.RepositoryError {
	return nil
}
func (f *fakeRepo) PruneRuns(context.Context, pgtype.Timestamptz) (int64, *repository.RepositoryError) {
	return 0, nil
}
func (f *fakeRepo) InsertRunSource(_ context.Context, p db.DiscoveryInsertRunSourceParams) *repository.RepositoryError {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sources = append(f.sources, p)
	return nil
}
func (f *fakeRepo) FinishRun(_ context.Context, p db.DiscoveryFinishRunParams) *repository.RepositoryError {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.finish = p
	return nil
}
func (f *fakeRepo) SelectSettings(context.Context) (db.DiscoverySelectSettingsRow, *repository.RepositoryError) {
	return f.settings, nil
}
func (f *fakeRepo) SelectAllKeywords(context.Context) ([]string, *repository.RepositoryError) {
	return f.userKws, nil
}
func (f *fakeRepo) SelectAllTechnologies(context.Context) ([]string, *repository.RepositoryError) {
	return f.techs, nil
}

func (f *fakeRepo) source(name string) (db.DiscoveryInsertRunSourceParams, bool) {
	for _, s := range f.sources {
		if s.Source == name {
			return s, true
		}
	}
	return db.DiscoveryInsertRunSourceParams{}, false
}

// fakeSource devolve uma vaga por busca e falha no termo "quebra". Antes de cada
// busca, guarda o andamento que o painel veria naquele momento.
type fakeSource struct {
	svc  *Service
	seen []SourceProgress
}

func (f *fakeSource) Name() string { return "fake" }
func (f *fakeSource) Search(_ context.Context, keyword string, page int) ([]Listing, bool, error) {
	if p := f.svc.Scheduler().Progress; p != nil {
		f.seen = append(f.seen, p.Sources[0])
	}
	if keyword == "quebra" {
		return nil, false, errors.New("status 500")
	}
	// "go" e "java" trazem a mesma vaga: só a primeira conta como nova.
	return []Listing{{Source: "fake", ExternalID: "x", Title: keyword, URL: "https://x/"}}, false, nil
}

// fakeLister entrega duas vagas, com uma empresa falhando.
type fakeLister struct{ boards []Board }

func (f *fakeLister) Name() string { return "lista" }
func (f *fakeLister) List(_ context.Context, boards []Board, step StepFunc) ListResult {
	f.boards = boards
	step(0, 2, "a")
	step(2, 2, "")
	return ListResult{
		Listings: []Listing{{Source: "lista", ExternalID: "1"}, {Source: "lista", ExternalID: "2"}},
		OK:       1, Failed: 1, LastError: "b: 404",
	}
}

func newTestService(repo *fakeRepo, sources ...Source) *Service {
	svc := &Service{Repo: repo, Logger: zap.NewNop(), fetched: map[string]time.Time{}, userRunning: map[string]bool{}, wake: make(chan struct{}, 1)}
	svc.Sources = sources
	return svc
}

func TestRecord_Andamento(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(repo)
	src := &fakeSource{svc: svc}
	lister := &fakeLister{}
	svc.Sources = []Source{src, lister}

	svc.record(context.Background(), runSpec{
		trigger: TriggerAdmin, keywords: []string{"go", "quebra", "java"},
		sources: svc.Sources, boards: map[string][]Board{"lista": {{Token: "a"}}}, full: true,
	})

	if len(src.seen) != 3 {
		t.Fatalf("esperava andamento visível nas 3 buscas, viu %d", len(src.seen))
	}
	second := src.seen[1]
	if second.Current != "quebra" || second.Kind != KindSearch || second.Page != 1 || second.Done != 1 || second.Total != 3 || second.JobsSaved != 1 {
		t.Errorf("andamento na 2ª busca: %+v", second)
	}
	if third := src.seen[2]; third.Failed != 1 {
		t.Errorf("a falha do 2º termo deveria aparecer na 3ª busca: %+v", third)
	}
	if svc.Scheduler().Progress != nil {
		t.Error("terminada a varredura, o andamento deveria sumir")
	}
	if len(lister.boards) != 1 {
		t.Errorf("o site de lista deveria receber as empresas dele: %+v", lister.boards)
	}

	// fake: 2 gravadas, 1 nova; lista: 2 gravadas, 2 novas.
	if repo.finish.Status != "partial" || repo.finish.JobsSaved != 4 || repo.finish.JobsNew != 3 {
		t.Errorf("varredura fechada como %+v", repo.finish)
	}
	if s, _ := repo.source("fake"); s.Kind != KindSearch || s.KeywordsOk != 2 || s.KeywordsFailed != 1 || s.JobsNew != 1 {
		t.Errorf("resultado do site de busca: %+v", s)
	}
	if s, _ := repo.source("lista"); s.Kind != KindList || s.KeywordsOk != 1 || s.KeywordsFailed != 1 || s.LastError != "b: 404" || s.JobsNew != 2 {
		t.Errorf("resultado do site de lista: %+v", s)
	}
	// A varredura grava o índice do Descobrir e incorpora a lista pendente dos GIN.
	if repo.cleaned != 1 {
		t.Errorf("lista pendente dos índices incorporada %d vezes, want 1", repo.cleaned)
	}
	for _, u := range repo.upserts {
		if u.TechsVersion != TechsVersion || u.Techs == nil {
			t.Errorf("upsert sem índice de tecnologias: %+v", u)
		}
	}
}

func TestRunFull_SitesEscolhidos(t *testing.T) {
	repo := &fakeRepo{
		settings: db.DiscoverySelectSettingsRow{
			Enabled: true, IntervalMinutes: 720, CatalogTerms: []string{"go"},
			DisabledSources: []string{"lista"}, // desligado, mas escolhido no painel
		},
		userKws: []string{"rh"},
		techs:   []string{"Go", "Go"}, // dois usuários com Go: um termo só, "golang"
	}
	svc := newTestService(repo)
	src := &fakeSource{svc: svc}
	svc.Sources = []Source{src, &fakeLister{}}

	svc.runFull(context.Background(), TriggerAdmin, RunOptions{Sources: []string{"lista"}, CatalogOnly: true})

	if !repo.run.Partial || len(repo.run.Sources) != 1 || repo.run.Keywords != 1 {
		t.Errorf("varredura parcial registrada como %+v", repo.run)
	}
	if _, ran := repo.source("fake"); ran || len(src.seen) != 0 {
		t.Error("o site não escolhido não deveria rodar")
	}
	if _, ran := repo.source("lista"); !ran {
		t.Error("o site escolhido deveria rodar mesmo desligado na configuração")
	}

	// Sem escolha: completa, com os termos dos usuários e só os sites ligados.
	repo.sources = nil
	svc.runFull(context.Background(), TriggerSchedule, RunOptions{})
	if repo.run.Partial || repo.run.Keywords != 3 {
		t.Errorf("varredura completa registrada como %+v", repo.run)
	}
	if _, ran := repo.source("lista"); ran {
		t.Error("site desligado não roda na varredura completa")
	}
}

func TestRunNow_SiteDesconhecido(t *testing.T) {
	svc := newTestService(&fakeRepo{}, &fakeLister{})
	if err := svc.RunNow(RunOptions{Sources: []string{"linkedin"}}); err == nil {
		t.Error("site que não existe deveria dar erro")
	}
}

func TestSave_IgnoraVagaVelha(t *testing.T) {
	repo := &fakeRepo{}
	svc := &Service{Repo: repo, Logger: zap.NewNop()}
	old := Listing{Source: "x", URL: "https://x/1", Title: "Dev Go", PublishedAt: time.Now().Add(-MaxJobAge - time.Hour)}
	if saved, _ := svc.save(context.Background(), old, ""); saved {
		t.Fatal("vaga publicada há mais de MaxJobAge foi gravada")
	}
	if len(repo.upserts) != 0 {
		t.Fatalf("upserts = %d, quero 0", len(repo.upserts))
	}
}
