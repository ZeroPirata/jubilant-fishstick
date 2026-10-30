package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/discovery"
	"hackton-treino/internal/middleware"
	discoveryRepo "hackton-treino/internal/repository/discovery"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Varreduras listadas nas métricas.
const discoveryAdminRuns = 30

const (
	maxCatalogTerms = 100
	maxBoards       = 100
)

// Token do quadro de vagas da empresa no ATS (o pedaço da URL pública).
var boardToken = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,79}$`)

// DiscoveryAdminHandler controla a busca de vagas (painel admin: configuração e
// "rodar agora") e alimenta a aba "Busca de vagas" do painel de métricas.
type DiscoveryAdminHandler struct {
	*BaseHandler
	Service *discovery.Service
	Repo    discoveryRepo.Repository
}

func NewDiscoveryAdminHandler(logger *zap.Logger, conn *pgxpool.Pool, svc *discovery.Service) *DiscoveryAdminHandler {
	return &DiscoveryAdminHandler{
		BaseHandler: NewBaseHandler(logger),
		Service:     svc,
		Repo:        discoveryRepo.New(conn),
	}
}

type DiscoverySettingsBody struct {
	Enabled         bool     `json:"enabled"`
	IntervalMinutes int      `json:"interval_minutes"`
	CatalogTerms    []string `json:"catalog_terms"`
	DisabledSources []string `json:"disabled_sources"`
	// Empresas por fonte de ATS: {"greenhouse": [{"token": "stone", "company": "Stone"}]}.
	Boards map[string][]discovery.Board `json:"boards"`
}

type discoverySourceStatus struct {
	Name           string     `json:"name"`
	Kind           string     `json:"kind"`
	Registered     bool       `json:"registered"` // false: saiu do código, sobraram as vagas
	Enabled        bool       `json:"enabled"`
	JobsTotal      int32      `json:"jobs_total"`
	JobsActive     int32      `json:"jobs_active"`
	LastRunAt      *time.Time `json:"last_run_at"`
	LastSuccessAt  *time.Time `json:"last_success_at"`
	KeywordsOK     int32      `json:"keywords_ok"`
	KeywordsFailed int32      `json:"keywords_failed"`
	JobsSaved      int32      `json:"jobs_saved"`
	JobsNew        int32      `json:"jobs_new"`
	LastError      string     `json:"last_error"`
}

type discoveryRunSource struct {
	Source         string `json:"source"`
	Kind           string `json:"kind"`
	KeywordsOK     int32  `json:"keywords_ok"`
	KeywordsFailed int32  `json:"keywords_failed"`
	JobsSaved      int32  `json:"jobs_saved"`
	JobsNew        int32  `json:"jobs_new"`
	LastError      string `json:"last_error"`
	DurationMs     int32  `json:"duration_ms"`
}

type discoveryRunView struct {
	ID          string               `json:"id"`
	Trigger     string               `json:"trigger"`
	Status      string               `json:"status"`
	Keywords    int32                `json:"keywords"`
	Partial     bool                 `json:"partial"`
	Selected    []string             `json:"selected_sources"`
	JobsSaved   int32                `json:"jobs_saved"`
	JobsNew     int32                `json:"jobs_new"`
	JobsRemoved int32                `json:"jobs_removed"`
	StartedAt   time.Time            `json:"started_at"`
	FinishedAt  *time.Time           `json:"finished_at"`
	Sources     []discoveryRunSource `json:"sources"`
}

type discoverySourceToggle struct {
	discovery.SourceInfo
	Enabled bool `json:"enabled"`
}

type discoverySettingsView struct {
	Settings  DiscoverySettingsBody   `json:"settings"`
	Sources   []discoverySourceToggle `json:"sources"`
	UpdatedAt *time.Time              `json:"updated_at"`
}

type discoveryMetricsView struct {
	Enabled         bool                     `json:"enabled"`
	IntervalMinutes int                      `json:"interval_minutes"`
	Scheduler       discovery.SchedulerState `json:"scheduler"`
	Sources         []discoverySourceStatus  `json:"sources"`
	Runs            []discoveryRunView       `json:"runs"`
}

// GetSettings é o que o painel admin edita: liga/desliga, intervalo, termos e quais
// sites usar. Métricas ficam em Metrics.
func (h *DiscoveryAdminHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, errR := h.Repo.SelectSettings(r.Context())
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	infos := h.Service.SourceInfos()
	sources := make([]discoverySourceToggle, len(infos))
	for i, info := range infos {
		sources[i] = discoverySourceToggle{SourceInfo: info, Enabled: !slices.Contains(settings.DisabledSources, info.Name)}
	}
	boards, err := discovery.ParseBoards(settings.Boards)
	if err != nil {
		h.Logger.Warn("Descoberta: empresas dos ATS ilegíveis", zap.Error(err))
	}
	writeJSON(w, http.StatusOK, discoverySettingsView{
		Settings: DiscoverySettingsBody{
			Enabled:         settings.Enabled,
			IntervalMinutes: int(settings.IntervalMinutes),
			CatalogTerms:    nonNil(settings.CatalogTerms),
			DisabledSources: nonNil(settings.DisabledSources),
			Boards:          boards,
		},
		Sources:   sources,
		UpdatedAt: timePtr(settings.UpdatedAt),
	})
}

// Metrics é a aba "Busca de vagas" do painel de métricas: próxima varredura, o
// andamento da que está rodando, a situação de cada site e as últimas varreduras.
func (h *DiscoveryAdminHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	settings, errR := h.Repo.SelectSettings(ctx)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	stats, errR := h.Repo.CatalogStats(ctx, pgtype.Timestamptz{Time: time.Now().Add(-discovery.ActiveFor), Valid: true})
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	lastResults, errR := h.Repo.LastSourceResults(ctx)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	runs, errR := h.Repo.ListRuns(ctx, discoveryAdminRuns)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	ids := make([]pgtype.UUID, len(runs))
	for i, run := range runs {
		ids[i] = run.ID
	}
	runSources, errR := h.Repo.ListRunSources(ctx, ids)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

	writeJSON(w, http.StatusOK, discoveryMetricsView{
		Enabled:         settings.Enabled,
		IntervalMinutes: int(settings.IntervalMinutes),
		Scheduler:       h.Service.Scheduler(),
		Sources:         buildSourceStatus(h.Service.SourceInfos(), settings.DisabledSources, stats, lastResults),
		Runs:            buildRunViews(runs, runSources),
	})
}

// buildSourceStatus junta as fontes registradas no código com as que só existem no
// histórico (removidas do código, mas ainda com vagas no banco).
func buildSourceStatus(infos []discovery.SourceInfo, disabled []string, stats []db.DiscoveryCatalogStatsRow, last []db.DiscoveryLastSourceResultsRow) []discoverySourceStatus {
	byName := map[string]*discoverySourceStatus{}
	kinds := map[string]string{}
	for _, info := range infos {
		kinds[info.Name] = info.Kind
	}
	var order []string
	get := func(name string) *discoverySourceStatus {
		if s, ok := byName[name]; ok {
			return s
		}
		kind, reg := kinds[name]
		s := &discoverySourceStatus{Name: name, Kind: kind, Registered: reg, Enabled: reg && !slices.Contains(disabled, name)}
		byName[name] = s
		order = append(order, name)
		return s
	}
	for _, info := range infos {
		get(info.Name)
	}
	for _, st := range stats {
		s := get(st.Source)
		s.JobsTotal, s.JobsActive = st.Total, st.Active
	}
	for _, l := range last {
		s := get(l.Source)
		s.LastRunAt = timePtr(l.StartedAt)
		s.LastSuccessAt = timePtr(l.LastSuccessAt)
		s.KeywordsOK, s.KeywordsFailed, s.JobsSaved, s.JobsNew = l.KeywordsOk, l.KeywordsFailed, l.JobsSaved, l.JobsNew
		if s.Kind == "" {
			s.Kind = l.Kind
		}
		s.LastError = l.LastError
	}

	out := make([]discoverySourceStatus, len(order))
	for i, name := range order {
		out[i] = *byName[name]
	}
	return out
}

func buildRunViews(runs []db.DiscoveryListRunsRow, sources []db.DiscoveryListRunSourcesRow) []discoveryRunView {
	byRun := map[string][]discoveryRunSource{}
	for _, s := range sources {
		key := s.RunID.String()
		byRun[key] = append(byRun[key], discoveryRunSource{
			Source:         s.Source,
			Kind:           s.Kind,
			KeywordsOK:     s.KeywordsOk,
			KeywordsFailed: s.KeywordsFailed,
			JobsSaved:      s.JobsSaved,
			JobsNew:        s.JobsNew,
			LastError:      s.LastError,
			DurationMs:     s.DurationMs,
		})
	}
	out := make([]discoveryRunView, len(runs))
	for i, run := range runs {
		out[i] = discoveryRunView{
			ID:          run.ID.String(),
			Trigger:     run.Trigger,
			Status:      run.Status,
			Keywords:    run.Keywords,
			Partial:     run.Partial,
			Selected:    nonNil(run.Sources),
			JobsSaved:   run.JobsSaved,
			JobsNew:     run.JobsNew,
			JobsRemoved: run.JobsRemoved,
			StartedAt:   run.StartedAt.Time,
			FinishedAt:  timePtr(run.FinishedAt),
			Sources:     byRun[run.ID.String()],
		}
		if out[i].Sources == nil {
			out[i].Sources = []discoveryRunSource{}
		}
	}
	return out
}

func (h *DiscoveryAdminHandler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	var body DiscoverySettingsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}
	clean, err := cleanDiscoverySettings(body, h.Service.SourceInfos())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	boards, err := json.Marshal(clean.Boards)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao salvar as empresas")
		return
	}
	if errR := h.Repo.UpdateSettings(r.Context(), db.DiscoveryUpdateSettingsParams{
		Boards:          boards,
		Enabled:         clean.Enabled,
		IntervalMinutes: int32(clean.IntervalMinutes),
		CatalogTerms:    clean.CatalogTerms,
		DisabledSources: clean.DisabledSources,
		UpdatedBy:       userID,
	}); errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	h.Service.SettingsChanged()
	h.Logger.Info("Descoberta: configuração alterada",
		zap.String("por", userID.String()),
		zap.Bool("ativa", clean.Enabled),
		zap.Int("intervalo_min", clean.IntervalMinutes),
		zap.Strings("fontes_desligadas", clean.DisabledSources),
		zap.Int("termos", len(clean.CatalogTerms)),
	)
	writeJSON(w, http.StatusOK, clean)
}

func cleanDiscoverySettings(b DiscoverySettingsBody, infos []discovery.SourceInfo) (DiscoverySettingsBody, error) {
	out := DiscoverySettingsBody{Enabled: b.Enabled, CatalogTerms: []string{}, DisabledSources: []string{}, Boards: map[string][]discovery.Board{}}
	registered := make([]string, len(infos))
	for i, info := range infos {
		registered[i] = info.Name
	}

	interval := time.Duration(b.IntervalMinutes) * time.Minute
	if interval < discovery.MinInterval || interval > discovery.MaxInterval {
		return out, fmt.Errorf("o intervalo precisa ficar entre %d minutos e %d dias",
			int(discovery.MinInterval.Minutes()), int(discovery.MaxInterval.Hours()/24))
	}
	out.IntervalMinutes = b.IntervalMinutes

	for _, src := range b.DisabledSources {
		if !slices.Contains(registered, src) {
			return out, fmt.Errorf("site desconhecido: %q", src)
		}
		if !slices.Contains(out.DisabledSources, src) {
			out.DisabledSources = append(out.DisabledSources, src)
		}
	}

	for _, t := range b.CatalogTerms {
		t = strings.ToLower(strings.Join(strings.Fields(t), " "))
		if t == "" || slices.Contains(out.CatalogTerms, t) {
			continue
		}
		if utf8.RuneCountInString(t) > 60 {
			return out, fmt.Errorf("termo muito longo: %q", t)
		}
		out.CatalogTerms = append(out.CatalogTerms, t)
	}
	if len(out.CatalogTerms) > maxCatalogTerms {
		return out, fmt.Errorf("no máximo %d termos no catálogo", maxCatalogTerms)
	}

	for _, info := range infos {
		if !info.UsesBoards {
			continue
		}
		boards, err := cleanBoards(info.Name, b.Boards[info.Name])
		if err != nil {
			return out, err
		}
		out.Boards[info.Name] = boards
	}
	for name := range b.Boards {
		if !slices.ContainsFunc(infos, func(i discovery.SourceInfo) bool { return i.Name == name && i.UsesBoards }) {
			return out, fmt.Errorf("o site %q não usa lista de empresas", name)
		}
	}
	return out, nil
}

// cleanBoards valida as empresas de um ATS: token como na URL pública, sem repetir.
// Sem nome, a empresa aparece pelo token.
func cleanBoards(source string, in []discovery.Board) ([]discovery.Board, error) {
	out := []discovery.Board{}
	seen := map[string]bool{}
	for _, b := range in {
		token := strings.ToLower(strings.TrimSpace(b.Token))
		if token == "" || seen[token] {
			continue
		}
		if !boardToken.MatchString(token) {
			return nil, fmt.Errorf("%s: token inválido %q (use o identificador da URL do quadro de vagas)", source, b.Token)
		}
		company := strings.Join(strings.Fields(b.Company), " ")
		if company == "" {
			company = token
		}
		if utf8.RuneCountInString(company) > 80 {
			return nil, fmt.Errorf("%s: nome de empresa muito longo: %q", source, company)
		}
		seen[token] = true
		out = append(out, discovery.Board{Token: token, Company: company})
	}
	if len(out) > maxBoards {
		return nil, fmt.Errorf("%s: no máximo %d empresas", source, maxBoards)
	}
	return out, nil
}

// DiscoveryRunBody escolhe o que rodar agora. Vazio (ou sem corpo) = varredura completa.
type DiscoveryRunBody struct {
	Sources     []string `json:"sources"`
	CatalogOnly bool     `json:"catalog_only"`
}

// RunNow dispara uma varredura agora, fora do horário: completa, ou só com os sites
// escolhidos e/ou só com os termos do catálogo. A parcial não muda o agendamento.
func (h *DiscoveryAdminHandler) RunNow(w http.ResponseWriter, r *http.Request) {
	var body DiscoveryRunBody
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
			return
		}
	}
	err := h.Service.RunNow(discovery.RunOptions{Sources: body.Sources, CatalogOnly: body.CatalogOnly})
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, h.Service.Scheduler())
	case errors.Is(err, discovery.ErrAlreadyRunning):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, discovery.ErrUnknownSource):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		h.Logger.Error("Descoberta: erro ao disparar varredura", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "erro ao disparar a varredura")
	}
}

func timePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

// DiscoveryBoardCheckBody: query (nome da empresa ou link do quadro de vagas) procura
// em todos os ATS; source + token confere uma empresa já cadastrada.
type DiscoveryBoardCheckBody struct {
	Query  string `json:"query"`
	Source string `json:"source"`
	Token  string `json:"token"`
}

type discoveryBoardCheckView struct {
	Results []discovery.BoardCheck `json:"results"`
}

// boardCheckTimeout: a Greenhouse de empresas grandes devolve milhares de vagas.
const boardCheckTimeout = 30 * time.Second

// CheckBoards responde "essa empresa está no Greenhouse/Lever? com quantas vagas que
// servem?" sem gravar nada. Usado para adicionar empresas e para conferir as cadastradas.
func (h *DiscoveryAdminHandler) CheckBoards(w http.ResponseWriter, r *http.Request) {
	var body DiscoveryBoardCheckBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), boardCheckTimeout)
	defer cancel()

	query := strings.TrimSpace(body.Query)
	switch {
	case query != "":
		if utf8.RuneCountInString(query) > 200 {
			writeError(w, http.StatusBadRequest, "busca muito longa")
			return
		}
		writeJSON(w, http.StatusOK, discoveryBoardCheckView{Results: h.Service.FindBoards(ctx, query)})
	case body.Source != "" && boardToken.MatchString(strings.ToLower(strings.TrimSpace(body.Token))):
		res, err := h.Service.CheckBoard(ctx, body.Source, body.Token)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, discoveryBoardCheckView{Results: []discovery.BoardCheck{res}})
	default:
		writeError(w, http.StatusBadRequest, "informe o nome ou link da empresa, ou o site e o token")
	}
}
