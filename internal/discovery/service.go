package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hackton-treino/internal/db"
	discoveryRepo "hackton-treino/internal/repository/discovery"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	// Páginas por palavra-chave numa fonte de busca. A Gupy devolve 12 por página e no
	// máximo 100 vagas por termo, então 9 páginas cobrem tudo.
	maxPages = 9
	// Pausa entre páginas da mesma fonte de busca, para não martelar o site.
	pageDelay = time.Second
	// Na busca de um usuário, palavra-chave buscada há menos que isso é pulada: dois
	// usuários com "golang", ou o mesmo clicando várias vezes, geram uma busca só.
	// A varredura completa (agendada ou do painel) ignora isso e busca tudo.
	freshFor = time.Hour
	// Tempo máximo de uma busca disparada pelo botão do usuário.
	userRefreshTimeout = 10 * time.Minute
	// Tempo máximo de uma varredura completa.
	fullRunTimeout = 2 * time.Hour
	// Espera mínima antes da primeira varredura depois que o servidor sobe, para um
	// restart com a varredura atrasada não competir com a subida.
	startupGrace = time.Minute

	// Vaga que não aparece numa busca há mais que isso sai da lista (cobre uma
	// varredura de 12h que falhou). Depois de deleteAfter ela é apagada.
	ActiveFor   = 36 * time.Hour
	deleteAfter = 7 * 24 * time.Hour
	// Histórico de varreduras mantido no painel.
	runsRetention = 30 * 24 * time.Hour

	// Limites do intervalo configurável no painel (iguais ao CHECK da tabela).
	MinInterval = 15 * time.Minute
	MaxInterval = 7 * 24 * time.Hour
)

const (
	TriggerSchedule = "schedule"
	TriggerAdmin    = "admin"
	TriggerUser     = "user"
)

var (
	ErrAlreadyRunning = errors.New("já existe uma busca em andamento")
	ErrNoKeywords     = errors.New("escolha ao menos uma área ou tecnologia para buscar")
	ErrDisabled       = errors.New("a busca de vagas está desativada pelo administrador")
	ErrUnknownSource  = errors.New("site desconhecido")
)

type Status struct {
	Running     bool      `json:"running"`
	LastRefresh time.Time `json:"last_refresh"`
}

// Settings é a configuração editável no painel admin (tabela discovery_settings).
type Settings struct {
	Enabled         bool
	Interval        time.Duration
	CatalogTerms    []string
	DisabledSources []string
	// Boards: empresas por fonte de ATS ("greenhouse" → [{stone, Stone}]).
	Boards map[string][]Board
}

// RunOptions é a varredura pedida no painel. Vazio = completa: todos os sites
// ligados, catálogo mais as palavras-chave dos usuários.
type RunOptions struct {
	// Sources escolhe os sites; um site desligado na configuração roda se for escolhido.
	Sources []string
	// CatalogOnly busca só os termos do catálogo, sem as palavras-chave dos usuários.
	CatalogOnly bool
}

func (o RunOptions) partial() bool { return len(o.Sources) > 0 || o.CatalogOnly }

// SchedulerState é o que o painel de métricas mostra do agendador.
type SchedulerState struct {
	Running   bool      `json:"running"`
	NextRunAt time.Time `json:"next_run_at"`
	// Andamento da varredura completa em curso; nil quando não há nenhuma.
	Progress *Progress `json:"progress"`
	// Buscas disparadas por usuários ("Buscar novas vagas") rodando agora.
	UserRuns int `json:"user_runs"`
}

// Progress é o andamento de uma varredura do agendador ou do painel. Os sites rodam
// em paralelo, então cada um tem o seu andamento.
type Progress struct {
	Trigger   string           `json:"trigger"`
	Partial   bool             `json:"partial"`
	StartedAt time.Time        `json:"started_at"`
	Keywords  int              `json:"keywords"`
	Sources   []SourceProgress `json:"sources"`
	JobsSaved int              `json:"jobs_saved"`
	JobsNew   int              `json:"jobs_new"`
	Failed    int              `json:"failed"`
	LastError string           `json:"last_error"`
}

// SourceProgress: num site de busca cada passo é um termo; num de lista, uma página
// ou empresa (Total 0 enquanto o site não disse quantas são).
type SourceProgress struct {
	Source    string     `json:"source"`
	Kind      string     `json:"kind"`
	Done      int        `json:"done"`
	Total     int        `json:"total"`
	Current   string     `json:"current"`
	Page      int        `json:"page"`
	JobsSaved int        `json:"jobs_saved"`
	JobsNew   int        `json:"jobs_new"`
	Failed    int        `json:"failed"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
}

type sourceResult struct {
	kind      string
	ok        int
	failed    int
	saved     int
	created   int
	lastError string
	duration  time.Duration
}

type Service struct {
	Repo    discoveryRepo.Repository
	Sources []Source
	Logger  *zap.Logger

	// wake acorda o agendador para reler a configuração (o admin salvou algo).
	wake chan struct{}

	mu          sync.Mutex
	fetched     map[string]time.Time // source|keyword → última busca com sucesso
	userRunning map[string]bool
	userLast    map[string]time.Time
	fullRunning bool
	nextRunAt   time.Time
	progress    *Progress
}

func NewService(conn *pgxpool.Pool, logger *zap.Logger) *Service {
	return &Service{
		Repo: discoveryRepo.New(conn),
		Sources: []Source{
			NewGupy(), NewProgramaThor(), NewGitHubVagas(),
			NewGreenhouse(), NewLever(), NewHimalayas(),
		},
		Logger:      logger,
		wake:        make(chan struct{}, 1),
		fetched:     map[string]time.Time{},
		userRunning: map[string]bool{},
		userLast:    map[string]time.Time{},
	}
}

// SourceNames lista as fontes registradas no código (habilitadas ou não).
func (s *Service) SourceNames() []string {
	names := make([]string, len(s.Sources))
	for i, src := range s.Sources {
		names[i] = src.Name()
	}
	return names
}

func (s *Service) source(name string) Source {
	for _, src := range s.Sources {
		if src.Name() == name {
			return src
		}
	}
	return nil
}

func (s *Service) LoadSettings(ctx context.Context) (Settings, error) {
	row, errR := s.Repo.SelectSettings(ctx)
	if errR != nil {
		return Settings{}, errR
	}
	boards, err := ParseBoards(row.Boards)
	if err != nil {
		s.Logger.Warn("Descoberta: empresas dos ATS ilegíveis", zap.Error(err))
	}
	return Settings{
		Enabled:         row.Enabled,
		Interval:        time.Duration(row.IntervalMinutes) * time.Minute,
		CatalogTerms:    row.CatalogTerms,
		DisabledSources: row.DisabledSources,
		Boards:          boards,
	}, nil
}

// ParseBoards lê a coluna boards (JSONB). Vazio vira mapa vazio.
func ParseBoards(raw []byte) (map[string][]Board, error) {
	boards := map[string][]Board{}
	if len(raw) == 0 {
		return boards, nil
	}
	err := json.Unmarshal(raw, &boards)
	return boards, err
}

// SettingsChanged faz o agendador recalcular a próxima varredura na hora.
func (s *Service) SettingsChanged() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) Scheduler() SchedulerState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := SchedulerState{Running: s.fullRunning, NextRunAt: s.nextRunAt}
	if s.progress != nil {
		p := *s.progress
		p.Sources = slices.Clone(p.Sources)
		st.Progress = &p
	}
	for _, running := range s.userRunning {
		if running {
			st.UserRuns++
		}
	}
	return st
}

// track aplica fn no andamento, se esta busca estiver sendo acompanhada (prog != nil).
// i é o índice do site em prog.Sources.
func (s *Service) track(prog *Progress, i int, fn func(p *Progress, sp *SourceProgress)) {
	if prog == nil {
		return
	}
	s.mu.Lock()
	fn(prog, &prog.Sources[i])
	s.mu.Unlock()
}

// Run é o agendador: a próxima varredura é o início da última completa mais o
// intervalo configurado. Mudanças no painel valem na hora (SettingsChanged).
func (s *Service) Run(ctx context.Context) {
	if n, errR := s.Repo.FailInterruptedRuns(ctx); errR == nil && n > 0 {
		s.Logger.Warn("Descoberta: varreduras interrompidas marcadas como erro", zap.Int64("quantidade", n))
	}

	minWait := startupGrace
	for {
		wait, ok := s.nextWait(ctx, minWait)
		minWait = 5 * time.Second

		var timer *time.Timer
		var timerC <-chan time.Time
		if ok {
			timer = time.NewTimer(wait)
			timerC = timer.C
		}

		fire := false
		select {
		case <-ctx.Done():
		case <-s.wake:
		case <-timerC:
			fire = true
		}
		if timer != nil {
			timer.Stop()
		}
		if ctx.Err() != nil {
			return
		}
		if fire {
			s.runFull(ctx, TriggerSchedule, RunOptions{})
		}
	}
}

// nextWait diz quanto falta para a próxima varredura; false quando está desativada.
func (s *Service) nextWait(ctx context.Context, minWait time.Duration) (time.Duration, bool) {
	st, err := s.LoadSettings(ctx)
	if err != nil {
		s.Logger.Error("Descoberta: erro ao ler configuração", zap.Error(err))
		st = Settings{Enabled: true, Interval: 12 * time.Hour}
	}
	if !st.Enabled {
		s.setNextRun(time.Time{})
		return 0, false
	}

	var last time.Time
	if ts, errR := s.Repo.LastFullRun(ctx); errR == nil && ts.Valid {
		last = ts.Time
	}
	wait := max(time.Until(last.Add(st.Interval)), minWait)
	s.setNextRun(time.Now().Add(wait))
	return wait, true
}

func (s *Service) setNextRun(t time.Time) {
	s.mu.Lock()
	s.nextRunAt = t
	s.mu.Unlock()
}

// RunNow dispara uma varredura fora do horário (painel), completa ou só com os sites
// e termos escolhidos.
func (s *Service) RunNow(opts RunOptions) error {
	for _, name := range opts.Sources {
		if s.source(name) == nil {
			return fmt.Errorf("%w: %q", ErrUnknownSource, name)
		}
	}
	s.mu.Lock()
	running := s.fullRunning
	s.mu.Unlock()
	if running {
		return ErrAlreadyRunning
	}
	go s.runFull(context.Background(), TriggerAdmin, opts)
	return nil
}

// runFull busca os termos (catálogo mais as palavras-chave de todos os usuários) nos
// sites de busca e baixa a lista inteira dos sites de lista, todos em paralelo.
// Registra o resultado e apaga as vagas que sumiram.
func (s *Service) runFull(parent context.Context, trigger string, opts RunOptions) {
	s.mu.Lock()
	if s.fullRunning {
		s.mu.Unlock()
		return
	}
	s.fullRunning = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.fullRunning = false
		s.mu.Unlock()
		s.SettingsChanged() // recalcula a próxima a partir desta
	}()

	ctx, cancel := context.WithTimeout(parent, fullRunTimeout)
	defer cancel()

	st, err := s.LoadSettings(ctx)
	if err != nil {
		s.Logger.Error("Descoberta: erro ao ler configuração", zap.Error(err))
		return
	}
	keywords := mergeKeywords(st.CatalogTerms)
	if !opts.CatalogOnly {
		userKeywords, errR := s.Repo.SelectAllKeywords(ctx)
		if errR != nil {
			s.Logger.Error("Descoberta: erro ao listar palavras-chave", zap.String("erro", errR.Message))
		}
		techs, errR := s.Repo.SelectAllTechnologies(ctx)
		if errR != nil {
			s.Logger.Error("Descoberta: erro ao listar tecnologias", zap.String("erro", errR.Message))
		}
		keywords = mergeKeywords(st.CatalogTerms, userKeywords, TechSearchTerms(techs))
	}

	var sources []Source
	for _, src := range s.Sources {
		chosen := slices.Contains(opts.Sources, src.Name())
		if chosen || (len(opts.Sources) == 0 && !slices.Contains(st.DisabledSources, src.Name())) {
			sources = append(sources, src)
		}
	}

	s.record(ctx, runSpec{
		trigger:  trigger,
		partial:  opts.partial(),
		keywords: keywords,
		sources:  sources,
		boards:   st.Boards,
		full:     true,
		selected: opts.Sources,
	})
}

// RefreshUser dispara em segundo plano a busca das palavras-chave de um usuário. Só
// os sites de busca entram: os de lista já trazem tudo na varredura completa.
func (s *Service) RefreshUser(userID pgtype.UUID) error {
	key := userID.String()

	st, err := s.LoadSettings(context.Background())
	if err != nil {
		return err
	}
	if !st.Enabled {
		return ErrDisabled
	}
	areas, errR := s.Repo.SelectUserKeywords(context.Background(), userID)
	if errR != nil {
		return errR
	}
	prefs, errR := s.Repo.SelectPreferences(context.Background(), userID)
	if errR != nil {
		return errR
	}
	keywords := mergeKeywords(areas, TechSearchTerms(prefs.Technologies))
	if len(keywords) == 0 {
		return ErrNoKeywords
	}

	var sources []Source
	for _, src := range s.Sources {
		if _, ok := src.(Searcher); ok && !slices.Contains(st.DisabledSources, src.Name()) {
			sources = append(sources, src)
		}
	}

	s.mu.Lock()
	if s.userRunning[key] {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	s.userRunning[key] = true
	s.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), userRefreshTimeout)
		defer cancel()
		s.record(ctx, runSpec{trigger: TriggerUser, keywords: keywords, sources: sources})

		s.mu.Lock()
		s.userRunning[key] = false
		s.userLast[key] = time.Now()
		s.mu.Unlock()
	}()
	return nil
}

func (s *Service) UserStatus(userID pgtype.UUID) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := userID.String()
	return Status{Running: s.userRunning[key], LastRefresh: s.userLast[key]}
}

type runSpec struct {
	trigger  string
	partial  bool
	keywords []string
	sources  []Source
	boards   map[string][]Board
	// full = varredura do agendador ou do painel: tem andamento, ignora a palavra-chave
	// "fresca" e apaga vagas antigas.
	full     bool
	selected []string
}

// record executa a busca e grava a varredura e o resultado de cada fonte.
func (s *Service) record(ctx context.Context, spec runSpec) {
	start := time.Now()
	// O registro usa um contexto próprio: se a busca estourar o tempo, o resultado
	// ainda precisa ser gravado.
	bg := context.WithoutCancel(ctx)

	runID, errR := s.Repo.InsertRun(bg, db.DiscoveryInsertRunParams{
		Trigger:  spec.trigger,
		Keywords: int32(len(spec.keywords)),
		Partial:  spec.partial,
		Sources:  nonNilStrings(spec.selected),
	})
	if errR != nil {
		s.Logger.Error("Descoberta: erro ao registrar varredura", zap.String("erro", errR.Message))
	}

	// Só a varredura completa tem andamento no painel: a do usuário é curta e pode
	// haver várias ao mesmo tempo.
	var prog *Progress
	if spec.full {
		prog = &Progress{Trigger: spec.trigger, Partial: spec.partial, StartedAt: start, Keywords: len(spec.keywords)}
		for _, src := range spec.sources {
			sp := SourceProgress{Source: src.Name(), Kind: SourceKind(src), StartedAt: start}
			if sp.Kind == KindSearch {
				sp.Total = len(spec.keywords)
			}
			prog.Sources = append(prog.Sources, sp)
		}
		s.mu.Lock()
		s.progress = prog
		s.mu.Unlock()
		defer func() {
			s.mu.Lock()
			s.progress = nil
			s.mu.Unlock()
		}()
	}

	results := s.runSources(ctx, spec, prog)

	var removed int64
	if spec.full {
		cutoff := pgtype.Timestamptz{Time: time.Now().Add(-deleteAfter), Valid: true}
		if removed, errR = s.Repo.DeleteStale(bg, cutoff); errR != nil {
			s.Logger.Warn("Descoberta: erro ao apagar vagas antigas", zap.String("erro", errR.Message))
		}
		runsCutoff := pgtype.Timestamptz{Time: time.Now().Add(-runsRetention), Valid: true}
		if _, errR = s.Repo.PruneRuns(bg, runsCutoff); errR != nil {
			s.Logger.Warn("Descoberta: erro ao apagar histórico antigo", zap.String("erro", errR.Message))
		}
	}

	saved, created, ok, failed := 0, 0, 0, 0
	for name, r := range results {
		saved += r.saved
		created += r.created
		ok += r.ok
		failed += r.failed
		if runID.Valid {
			if errR := s.Repo.InsertRunSource(bg, db.DiscoveryInsertRunSourceParams{
				RunID:          runID,
				Source:         name,
				Kind:           r.kind,
				KeywordsOk:     int32(r.ok),
				KeywordsFailed: int32(r.failed),
				JobsSaved:      int32(r.saved),
				JobsNew:        int32(r.created),
				LastError:      r.lastError,
				DurationMs:     int32(r.duration.Milliseconds()),
			}); errR != nil {
				s.Logger.Warn("Descoberta: erro ao registrar fonte", zap.String("fonte", name), zap.String("erro", errR.Message))
			}
		}
	}

	status := runStatus(ok, failed)
	if runID.Valid {
		if errR := s.Repo.FinishRun(bg, db.DiscoveryFinishRunParams{
			ID:          runID,
			Status:      status,
			JobsSaved:   int32(saved),
			JobsNew:     int32(created),
			JobsRemoved: int32(removed),
		}); errR != nil {
			s.Logger.Error("Descoberta: erro ao fechar varredura", zap.String("erro", errR.Message))
		}
	}

	s.Logger.Info("Descoberta de vagas concluída",
		zap.String("gatilho", spec.trigger),
		zap.Bool("parcial", spec.partial),
		zap.String("status", status),
		zap.Int("palavras_chave", len(spec.keywords)),
		zap.Int("vagas_gravadas", saved),
		zap.Int("vagas_novas", created),
		zap.Int64("vagas_removidas", removed),
		zap.Duration("duracao", time.Since(start)),
	)
}

// runStatus: success sem falhas, error se nada funcionou, partial no meio. Busca
// sem nada a fazer (tudo "fresco") conta como sucesso.
func runStatus(ok, failed int) string {
	switch {
	case failed == 0:
		return "success"
	case ok == 0:
		return "error"
	default:
		return "partial"
	}
}

// runSources roda cada site na sua goroutine: o tempo total é o do site mais lento,
// não a soma. Dentro de um site os pedidos continuam um de cada vez, com pausa.
func (s *Service) runSources(ctx context.Context, spec runSpec, prog *Progress) map[string]*sourceResult {
	results := make(map[string]*sourceResult, len(spec.sources))
	for _, src := range spec.sources {
		results[src.Name()] = &sourceResult{kind: SourceKind(src)}
	}

	var wg sync.WaitGroup
	for i, src := range spec.sources {
		res := results[src.Name()]
		wg.Go(func() {
			start := time.Now()
			switch v := src.(type) {
			case Lister:
				s.runLister(ctx, v, spec.boards[src.Name()], i, res, prog)
			case Searcher:
				s.runSearcher(ctx, v, spec.keywords, spec.full, i, res, prog)
			}
			res.duration = time.Since(start)
			s.track(prog, i, func(_ *Progress, sp *SourceProgress) {
				now := time.Now()
				sp.EndedAt, sp.Current = &now, ""
			})
		})
	}
	wg.Wait()
	return results
}

func (s *Service) runSearcher(ctx context.Context, src Searcher, keywords []string, force bool, i int, res *sourceResult, prog *Progress) {
	for _, kw := range keywords {
		if ctx.Err() != nil {
			res.failed++
			res.lastError = "tempo esgotado: " + ctx.Err().Error()
			return
		}
		s.track(prog, i, func(_ *Progress, sp *SourceProgress) { sp.Current, sp.Page = kw, 0 })
		if !s.claim(src.Name(), kw, force) {
			s.track(prog, i, func(_ *Progress, sp *SourceProgress) { sp.Done++ })
			continue
		}
		err := s.searchKeyword(ctx, src, kw, i, res, prog)
		if err != nil {
			s.release(src.Name(), kw)
			res.failed++
			res.lastError = fmt.Sprintf("%q: %v", kw, err)
			s.track(prog, i, func(p *Progress, sp *SourceProgress) {
				sp.Done++
				sp.Failed++
				p.Failed++
				p.LastError = src.Name() + " · " + res.lastError
			})
			s.Logger.Warn("Descoberta: falha na busca",
				zap.String("fonte", src.Name()), zap.String("keyword", kw), zap.Error(err))
			continue
		}
		res.ok++
		s.track(prog, i, func(_ *Progress, sp *SourceProgress) { sp.Done++ })
	}
}

func (s *Service) searchKeyword(ctx context.Context, src Searcher, keyword string, i int, res *sourceResult, prog *Progress) error {
	for page := 1; page <= maxPages; page++ {
		if page > 1 && !wait(ctx, pageDelay) {
			return ctx.Err()
		}
		s.track(prog, i, func(_ *Progress, sp *SourceProgress) { sp.Page = page })
		listings, hasMore, err := src.Search(ctx, keyword, page)
		if err != nil {
			return err
		}
		s.saveAll(ctx, listings, keyword, i, res, prog)
		if !hasMore {
			break
		}
	}
	return nil
}

func (s *Service) runLister(ctx context.Context, src Lister, boards []Board, i int, res *sourceResult, prog *Progress) {
	lr := src.List(ctx, boards, func(done, total int, current string) {
		s.track(prog, i, func(_ *Progress, sp *SourceProgress) {
			sp.Done, sp.Total, sp.Current = done, total, current
		})
	})
	res.ok, res.failed, res.lastError = lr.OK, lr.Failed, lr.LastError
	if lr.Failed > 0 {
		s.track(prog, i, func(p *Progress, sp *SourceProgress) {
			sp.Failed += lr.Failed
			p.Failed += lr.Failed
			p.LastError = src.Name() + " · " + lr.LastError
		})
		s.Logger.Warn("Descoberta: falha na lista", zap.String("fonte", src.Name()), zap.String("erro", lr.LastError))
	}
	if len(lr.Listings) > 0 {
		s.track(prog, i, func(_ *Progress, sp *SourceProgress) { sp.Current = fmt.Sprintf("gravando %d vagas", len(lr.Listings)) })
	}
	s.saveAll(ctx, lr.Listings, "", i, res, prog)
}

// saveAll grava as vagas. keyword vazia (sites de lista) não liga palavra-chave.
func (s *Service) saveAll(ctx context.Context, listings []Listing, keyword string, i int, res *sourceResult, prog *Progress) {
	for _, l := range listings {
		saved, created := s.save(ctx, l, keyword)
		if !saved {
			continue
		}
		res.saved++
		if created {
			res.created++
		}
		s.track(prog, i, func(p *Progress, sp *SourceProgress) {
			sp.JobsSaved++
			p.JobsSaved++
			if created {
				sp.JobsNew++
				p.JobsNew++
			}
		})
	}
}

// claim marca a palavra-chave como buscada agora; false se ela já foi buscada há
// pouco (ou está sendo buscada por outra chamada). force ignora a janela.
func (s *Service) claim(source, keyword string, force bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := source + "|" + keyword
	if !force && time.Since(s.fetched[key]) < freshFor {
		return false
	}
	s.fetched[key] = time.Now()
	return true
}

func (s *Service) release(source, keyword string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.fetched, source+"|"+keyword)
}

// save grava a vaga (nova ou atualizada) e diz se ela era nova.
func (s *Service) save(ctx context.Context, l Listing, keyword string) (saved, created bool) {
	// O registro continua mesmo com o contexto vencido: a vaga já foi baixada.
	ctx = context.WithoutCancel(ctx)
	row, errR := s.Repo.UpsertJob(ctx, db.DiscoveryUpsertJobParams{
		Source:        l.Source,
		ExternalID:    l.ExternalID,
		Title:         l.Title,
		Company:       l.Company,
		Location:      l.Location,
		WorkplaceType: l.WorkplaceType,
		International: l.International,
		Url:           l.URL,
		Description:   l.Description,
		Seniority:     MergeSeniority(l.Seniority, ClassifySeniority(l.Title)),
		PublishedAt:   toTimestamptz(l.PublishedAt),
		ExpiresAt:     toTimestamptz(l.ExpiresAt),
	})
	if errR != nil {
		s.Logger.Warn("Descoberta: erro ao gravar vaga", zap.String("url", l.URL), zap.String("erro", errR.Message))
		return false, false
	}
	if keyword != "" {
		if errR := s.Repo.LinkKeyword(ctx, db.DiscoveryLinkKeywordParams{JobID: row.ID, Keyword: keyword}); errR != nil {
			s.Logger.Warn("Descoberta: erro ao ligar palavra-chave", zap.String("url", l.URL), zap.String("erro", errR.Message))
		}
	}
	return true, row.Inserted
}

// mergeKeywords junta as listas sem repetir, normalizando como a query faz.
func mergeKeywords(lists ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range lists {
		for _, kw := range list {
			kw = strings.ToLower(strings.TrimSpace(kw))
			if kw != "" && !seen[kw] {
				seen[kw] = true
				out = append(out, kw)
			}
		}
	}
	return out
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func toTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// SourceInfo descreve uma fonte registrada para o painel.
type SourceInfo struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	UsesBoards bool   `json:"uses_boards"`
}

func (s *Service) SourceInfos() []SourceInfo {
	out := make([]SourceInfo, len(s.Sources))
	for i, src := range s.Sources {
		out[i] = SourceInfo{Name: src.Name(), Kind: SourceKind(src), UsesBoards: UsesBoards(src)}
	}
	return out
}
