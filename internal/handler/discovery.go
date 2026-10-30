package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/discovery"
	"hackton-treino/internal/middleware"
	discoveryRepo "hackton-treino/internal/repository/discovery"
	"hackton-treino/internal/repository/users"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Vagas do catálogo carregadas por requisição para ordenar pelas skills. A ordenação
// precisa ver todas antes de paginar; acima disso ficam só as mais recentes.
const discoveryRankLimit = 2000

type DiscoveryHandler struct {
	*BaseHandler
	Service *discovery.Service
	Repo    discoveryRepo.Repository
	Users   users.Repository
}

func NewDiscoveryHandler(logger *zap.Logger, conn *pgxpool.Pool, svc *discovery.Service) *DiscoveryHandler {
	return &DiscoveryHandler{
		BaseHandler: NewBaseHandler(logger),
		Service:     svc,
		Repo:        discoveryRepo.New(conn),
		Users:       users.New(conn),
	}
}

type DiscoveredJob struct {
	ID            string     `json:"id"`
	Source        string     `json:"source"`
	Title         string     `json:"title"`
	Company       string     `json:"company"`
	Location      string     `json:"location"`
	WorkplaceType string     `json:"workplace_type"`
	Url           string     `json:"url"`
	Snippet       string     `json:"snippet"`
	PublishedAt   *time.Time `json:"published_at"`
	MatchedSkills []string   `json:"matched_skills"`
	// MatchedTechs: tecnologias de interesse que a vaga cita.
	MatchedTechs []string `json:"matched_techs"`
	AlreadyAdded bool     `json:"already_added"`
	// PipelineJobID e PipelineStatus: a vaga na pipeline (Vagas), quando já foi mandada.
	PipelineJobID  string `json:"pipeline_job_id,omitempty"`
	PipelineStatus string `json:"pipeline_status,omitempty"`
	International  bool   `json:"international"`
	// Seniority: níveis da vaga; vazio quando ela não informa.
	Seniority []string `json:"seniority"`
	// Stack: as outras tecnologias do catálogo que a vaga cita, além das do usuário
	// ("C#, Java, Go ou Rust" com Go no perfil traz C#, Java e Rust). Só é calculada
	// para a página devolvida.
	Stack []string `json:"stack"`
	// Language: idioma estimado da vaga ("en" ou "pt-br"), padrão do seletor de idioma
	// ao gerar o currículo. Só é calculado para a página devolvida.
	Language string `json:"language,omitempty"`
	text     string
}

// ListJobs devolve as vagas do catálogo que citam skills em que o usuário é forte
// (avançado ou expert), das que citam mais para as que citam menos. Quem ainda não
// marcou nenhuma skill como avançada/expert vê o casamento com todas as skills.
//
// Filtros: o local de interesse e o nível salvos (/discovery/preferences) e, na query string,
// ?source= (plataformas) e ?skill= (a vaga precisa citar todas), ambos repetíveis.
func (h *DiscoveryHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	cursor, size := getPaginationParams(r)

	skills, errR := h.Users.QuerySelectAllSkills(r.Context(), userID.String())
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	prefs, errR := h.Repo.SelectPreferences(r.Context(), userID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	f := discoveryFilter{
		Location:  discovery.LocationPrefs{WorkplaceTypes: prefs.WorkplaceTypes, Cities: prefs.Cities},
		Seniority: discovery.SeniorityPrefs{Levels: prefs.Seniorities, IncludeUnknown: prefs.SeniorityUnknown},
	}

	matchSkills, onlyStrong := strongSkills(skills)
	techs := prefs.Technologies
	if len(matchSkills) == 0 && len(techs) == 0 {
		writeDiscoveryList(w, discoveryResult{Jobs: []DiscoveredJob{}, Facets: emptyFacets()}, size, cursor, onlyStrong)
		return
	}
	// Com tecnologias escolhidas, elas decidem quais vagas entram (mesmo sem skill do
	// perfil); as skills só ordenam. Sem tecnologias, entram as que citam as skills.
	pattern := discovery.RegexPattern(matchSkills)
	if len(techs) > 0 {
		pattern = discovery.RegexPattern(discovery.TechSkills(techs))
	}

	rows, errR := h.Repo.ListForUser(r.Context(), db.DiscoveryListForUserParams{
		UserID:      userID,
		ActiveSince: pgtype.Timestamptz{Time: time.Now().Add(-discovery.ActiveFor), Valid: true},
		Pattern:     pattern,
		MaxCount:    discoveryRankLimit,
	})
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

	// O regex do banco ainda deixa passar o "c" de "c#"; aqui só fica quem casa por
	// palavra inteira.
	// A mesma vaga costuma ser publicada uma vez por cidade: fica a mais recente.
	jobs := make([]DiscoveredJob, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		// Sem empresa (issues do GitHub) o título sozinho juntaria vagas diferentes.
		key := strings.ToLower(row.Title + "|" + row.Company)
		if row.Company == "" {
			key = row.Url
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		text := row.Title + "\n" + row.Description
		matched := discovery.MatchSkills(text, matchSkills)
		matchedTechs := discovery.MatchTechs(text, techs)
		if (len(techs) > 0 && len(matchedTechs) == 0) || (len(techs) == 0 && len(matched) == 0) {
			continue
		}
		job := toDiscoveredJob(row, matched)
		job.MatchedTechs = nonNil(matchedTechs)
		job.text = text
		jobs = append(jobs, job)
	}
	// Estável: empate mantém a ordem da query (mais recente primeiro).
	sort.SliceStable(jobs, func(a, b int) bool {
		if len(jobs[a].MatchedSkills) != len(jobs[b].MatchedSkills) {
			return len(jobs[a].MatchedSkills) > len(jobs[b].MatchedSkills)
		}
		return len(jobs[a].MatchedTechs) > len(jobs[b].MatchedTechs)
	})

	q := r.URL.Query()
	f.Sources, f.Skills = q["source"], q["skill"]
	result := filterDiscovered(jobs, f)
	writeDiscoveryList(w, result, size, cursor, onlyStrong)
}

type facetCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type discoveryFacets struct {
	Sources []facetCount `json:"sources"`
	Skills  []facetCount `json:"skills"`
	// Levels segue a ordem de discovery.SeniorityLevels; "unknown" conta as vagas sem nível.
	Levels []facetCount `json:"levels"`
}

const unknownLevel = "unknown"

// otherStack devolve as tecnologias citadas na vaga que não são as já destacadas
// (skills do perfil e tecnologias de interesse).
func otherStack(j DiscoveredJob) []string {
	out := []string{}
	if j.text == "" {
		return out
	}
	skills := make([]discovery.Skill, len(j.MatchedSkills))
	for i, s := range j.MatchedSkills {
		skills[i] = discovery.NewSkill(s)
	}
	for _, t := range discovery.MatchTechs(j.text, discovery.TechNames) {
		if slices.Contains(j.MatchedTechs, t) || slices.ContainsFunc(skills, func(s discovery.Skill) bool {
			return discovery.TechCoveredBy(t, s)
		}) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func emptyFacets() discoveryFacets {
	return discoveryFacets{Sources: []facetCount{}, Skills: []facetCount{}, Levels: levelFacets(nil)}
}

type discoveryFilter struct {
	Location  discovery.LocationPrefs
	Seniority discovery.SeniorityPrefs
	Sources   []string
	Skills    []string
}

type discoveryResult struct {
	Jobs   []DiscoveredJob
	Facets discoveryFacets
}

// filterDiscovered aplica local, nível, plataformas e skills. As contagens de cada
// filtro consideram os outros filtros, mas não ele mesmo: a contagem de "gupy" diz
// quantas vagas aparecem ao marcar a gupy com as skills já escolhidas.
func filterDiscovered(jobs []DiscoveredJob, f discoveryFilter) discoveryResult {
	inLocation := make([]DiscoveredJob, 0, len(jobs))
	for _, j := range jobs {
		if f.Location.MatchesLocation(j.WorkplaceType, j.Location, j.International) {
			inLocation = append(inLocation, j)
		}
	}

	sourceOK := func(j DiscoveredJob) bool { return len(f.Sources) == 0 || slices.Contains(f.Sources, j.Source) }
	skillsOK := func(j DiscoveredJob) bool {
		for _, s := range f.Skills {
			if !slices.Contains(j.MatchedSkills, s) {
				return false
			}
		}
		return true
	}

	sourceCount := map[string]int{}
	skillCount := map[string]int{}
	levelCount := map[string]int{}
	final := make([]DiscoveredJob, 0, len(inLocation))
	for _, j := range inLocation {
		src, sk, lv := sourceOK(j), skillsOK(j), f.Seniority.Matches(j.Seniority)
		if sk && lv {
			sourceCount[j.Source]++
		}
		if src && lv {
			for _, s := range j.MatchedSkills {
				skillCount[s]++
			}
		}
		if src && sk {
			for _, l := range j.Seniority {
				levelCount[l]++
			}
			if len(j.Seniority) == 0 {
				levelCount[unknownLevel]++
			}
		}
		if src && sk && lv {
			final = append(final, j)
		}
	}
	return discoveryResult{
		Jobs: final,
		Facets: discoveryFacets{
			Sources: sortedFacets(sourceCount),
			Skills:  sortedFacets(skillCount),
			Levels:  levelFacets(levelCount),
		},
	}
}

// levelFacets lista todos os níveis na ordem, mesmo com 0, e "unknown" no fim.
func levelFacets(counts map[string]int) []facetCount {
	out := make([]facetCount, 0, len(discovery.SeniorityLevels)+1)
	for _, l := range append(slices.Clone(discovery.SeniorityLevels), unknownLevel) {
		out = append(out, facetCount{Name: l, Count: counts[l]})
	}
	return out
}

func sortedFacets(counts map[string]int) []facetCount {
	out := make([]facetCount, 0, len(counts))
	for name, n := range counts {
		out = append(out, facetCount{Name: name, Count: n})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Count != out[b].Count {
			return out[a].Count > out[b].Count
		}
		return out[a].Name < out[b].Name
	})
	return out
}

// writeDiscoveryList é o writeList com only_strong_skills (a tela avisa quando o
// casamento caiu para todas as skills) e as contagens dos filtros.
func writeDiscoveryList(w http.ResponseWriter, res discoveryResult, size, cursor int32, onlyStrong bool) {
	total := int32(len(res.Jobs))
	start := min(max(cursor, 0), total)
	end := min(start+max(size, 1), total)
	for i := start; i < end; i++ {
		res.Jobs[i].Stack = otherStack(res.Jobs[i])
		res.Jobs[i].Language = discovery.GuessLanguage(res.Jobs[i].text)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":               res.Jobs[start:end],
		"meta":               meta{Size: size, Cursor: cursor, Total: total},
		"only_strong_skills": onlyStrong,
		"facets":             res.Facets,
	})
}

type DiscoveryPreferences struct {
	WorkplaceTypes []string `json:"workplace_types"`
	Cities         []string `json:"cities"`
	Technologies   []string `json:"technologies"`
	// Seniorities: níveis aceitos (vazio não filtra). SeniorityUnknown deixa passar as
	// vagas sem nível; ausente no corpo, vale true.
	Seniorities      []string `json:"seniorities"`
	SeniorityUnknown *bool    `json:"seniority_unknown"`
}

const (
	maxPreferenceCities = 10
	maxPreferenceTechs  = 30
)

// Technologies é o catálogo de "Tecnologias de interesse", agrupado como na tela.
func (h *DiscoveryHandler) Technologies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, discovery.Technologies)
}

func (h *DiscoveryHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	prefs, errR := h.Repo.SelectPreferences(r.Context(), userID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	writeJSON(w, http.StatusOK, DiscoveryPreferences{
		WorkplaceTypes:   nonNil(prefs.WorkplaceTypes),
		Cities:           nonNil(prefs.Cities),
		Technologies:     nonNil(prefs.Technologies),
		Seniorities:      nonNil(prefs.Seniorities),
		SeniorityUnknown: &prefs.SeniorityUnknown,
	})
}

func (h *DiscoveryHandler) SavePreferences(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	var body DiscoveryPreferences
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}
	prefs, err := cleanPreferences(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errR := h.Repo.UpsertPreferences(r.Context(), db.DiscoveryUpsertPreferencesParams{
		UserID:           userID,
		WorkplaceTypes:   prefs.WorkplaceTypes,
		Cities:           prefs.Cities,
		Technologies:     prefs.Technologies,
		Seniorities:      prefs.Seniorities,
		SeniorityUnknown: *prefs.SeniorityUnknown,
	}); errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func cleanPreferences(p DiscoveryPreferences) (DiscoveryPreferences, error) {
	unknown := p.SeniorityUnknown == nil || *p.SeniorityUnknown
	out := DiscoveryPreferences{
		WorkplaceTypes: []string{}, Cities: []string{}, Technologies: []string{},
		Seniorities: []string{}, SeniorityUnknown: &unknown,
	}
	for _, wt := range p.WorkplaceTypes {
		if !slices.Contains(discovery.LocationModes, wt) {
			return out, fmt.Errorf("modo de trabalho inválido: %q", wt)
		}
		if !slices.Contains(out.WorkplaceTypes, wt) {
			out.WorkplaceTypes = append(out.WorkplaceTypes, wt)
		}
	}
	for _, c := range p.Cities {
		c = strings.Join(strings.Fields(c), " ")
		if c == "" || slices.Contains(out.Cities, c) {
			continue
		}
		if utf8.RuneCountInString(c) > 80 {
			return out, errors.New("nome de cidade muito longo")
		}
		out.Cities = append(out.Cities, c)
	}
	if len(out.Cities) > maxPreferenceCities {
		return out, fmt.Errorf("no máximo %d cidades", maxPreferenceCities)
	}
	for _, t := range p.Technologies {
		if _, ok := discovery.TechByName(t); !ok {
			return out, fmt.Errorf("tecnologia desconhecida: %q", t)
		}
		if !slices.Contains(out.Technologies, t) {
			out.Technologies = append(out.Technologies, t)
		}
	}
	if len(out.Technologies) > maxPreferenceTechs {
		return out, fmt.Errorf("no máximo %d tecnologias", maxPreferenceTechs)
	}
	for _, l := range p.Seniorities {
		if !slices.Contains(discovery.SeniorityLevels, l) {
			return out, fmt.Errorf("nível inválido: %q", l)
		}
		if !slices.Contains(out.Seniorities, l) {
			out.Seniorities = append(out.Seniorities, l)
		}
	}
	return out, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// strongSkills devolve as skills avançadas/expert; sem nenhuma, devolve todas e false.
func strongSkills(skills []db.UserSkill) ([]discovery.Skill, bool) {
	var strong, all []discovery.Skill
	for _, s := range skills {
		skill := discovery.NewSkill(s.SkillName)
		all = append(all, skill)
		if s.ProficiencyLevel == db.SkillLevelAdvanced || s.ProficiencyLevel == db.SkillLevelExpert {
			strong = append(strong, skill)
		}
	}
	if len(strong) > 0 {
		return strong, true
	}
	return all, false
}

// Refresh dispara a busca das palavras-chave do usuário em segundo plano.
func (h *DiscoveryHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}

	err := h.Service.RefreshUser(userID)
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, h.Service.UserStatus(userID))
	case errors.Is(err, discovery.ErrAlreadyRunning):
		writeError(w, http.StatusTooManyRequests, err.Error())
	case errors.Is(err, discovery.ErrNoKeywords), errors.Is(err, discovery.ErrDisabled):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		h.Logger.Error("Descoberta: erro ao iniciar busca", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "erro ao iniciar a busca")
	}
}

func (h *DiscoveryHandler) Status(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	writeJSON(w, http.StatusOK, h.Service.UserStatus(userID))
}

func toDiscoveredJob(row db.DiscoveryListForUserRow, matched []string) DiscoveredJob {
	job := DiscoveredJob{
		ID:            row.ID.String(),
		Source:        row.Source,
		Title:         row.Title,
		Company:       row.Company,
		Location:      row.Location,
		WorkplaceType: row.WorkplaceType,
		Url:           row.Url,
		Snippet:       snippet(row.Description, 280),
		MatchedSkills: matched,
		AlreadyAdded:  row.PipelineJobID.Valid,
		International: row.International,
		Seniority:     nonNil(row.Seniority),
		Stack:         []string{},
	}
	if row.PipelineJobID.Valid {
		job.PipelineJobID = row.PipelineJobID.String()
		job.PipelineStatus = row.PipelineStatus
	}
	if job.MatchedSkills == nil {
		job.MatchedSkills = []string{}
	}
	if row.PublishedAt.Valid {
		t := row.PublishedAt.Time
		job.PublishedAt = &t
	}
	return job
}

func snippet(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}
