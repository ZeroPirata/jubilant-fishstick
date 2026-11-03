package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/discovery"
	"hackton-treino/internal/jobfit"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/repository/aliases"
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

// Teto de segurança das candidatas por requisição. Elas vêm sem a descrição (linhas
// leves) e todas entram no ranking e nas contagens dos filtros; o teto só existe para
// um perfil que case com o catálogo inteiro não pesar demais.
const discoveryRankLimit = 20000

type DiscoveryHandler struct {
	*BaseHandler
	Service *discovery.Service
	Repo    discoveryRepo.Repository
	Users   users.Repository
	Aliases aliases.Repository
}

func NewDiscoveryHandler(logger *zap.Logger, conn *pgxpool.Pool, svc *discovery.Service) *DiscoveryHandler {
	return &DiscoveryHandler{
		BaseHandler: NewBaseHandler(logger),
		Service:     svc,
		Repo:        discoveryRepo.New(conn),
		Users:       users.New(conn),
		Aliases:     aliases.New(conn),
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
	// ao gerar o currículo. Calculado na varredura.
	Language string `json:"language,omitempty"`
	// Fit: compatibilidade da stack da vaga com o perfil (o mesmo JobFit da aba Vagas),
	// que ordena a lista. Só vai na resposta para as vagas da página.
	Fit *jobfit.Fit `json:"fit,omitempty"`
	// AlsoOn: a mesma vaga (empresa e título iguais) publicada em outros sites. A
	// lista mostra uma só, com os links das outras, em vez de repetir.
	AlsoOn []JobLink `json:"also_on,omitempty"`
	// Dismissed: o usuário dispensou a vaga (só aparece em ?dismissed=1).
	Dismissed bool `json:"dismissed,omitempty"`
	techs     []string
	fit       jobfit.Fit
	firstSeen time.Time
}

type JobLink struct {
	Source string `json:"source"`
	Url    string `json:"url"`
}

// ListJobs devolve as vagas do catálogo que citam skills em que o usuário é forte
// (avançado ou expert), das mais compatíveis (JobFit) para as menos. Quem ainda não
// marcou nenhuma skill como avançada/expert vê o casamento com todas as skills.
//
// O texto das vagas não é lido aqui: a varredura grava as tecnologias de cada vaga
// (techs, índice GIN) e as skills fora do catálogo usam a busca de texto do Postgres
// (search_vec, índice GIN). Vêm só linhas leves; a descrição, só das vagas da página.
//
// Filtros: o local de interesse e o nível salvos (/discovery/preferences) e, na query string,
// ?source= (plataformas) e ?skill= (a vaga precisa citar todas), ambos repetíveis.
// As vagas dispensadas ficam de fora; ?dismissed=1 lista só elas (para restaurar).
func (h *DiscoveryHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	ctx := r.Context()
	cursor, size := getPaginationParams(r)

	view := hideDismissed
	if r.URL.Query().Get("dismissed") == "1" {
		view = onlyDismissed
	}
	rk, errR := h.rankedJobs(ctx, userID, view)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	if rk.jobs == nil {
		writeDiscoveryList(w, discoveryResult{Jobs: []DiscoveredJob{}, Facets: emptyFacets()}, size, cursor, rk)
		return
	}

	q := r.URL.Query()
	f := rk.filter
	f.Sources, f.Skills = q["source"], q["skill"]
	result := filterDiscovered(rk.jobs, f)
	if errR := h.fillPage(ctx, userID, result.Jobs, cursor, size); errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	writeDiscoveryList(w, result, size, cursor, rk)
}

// dismissView: o que fazer com as vagas que o usuário dispensou.
type dismissView int

const (
	hideDismissed dismissView = iota
	onlyDismissed
)

// ranked: as vagas candidatas do usuário já ranqueadas, e o filtro salvo (local e
// nível) para aplicar nelas. jobs nil = o perfil não tem skills nem tecnologias de
// interesse para casar.
type ranked struct {
	jobs       []DiscoveredJob
	filter     discoveryFilter
	onlyStrong bool
	// dismissed: quantas candidatas o usuário dispensou (a tela mostra "N dispensadas").
	dismissed int
}

// rankedJobs faz a parte comum da lista do Descobrir e dos alertas: candidatas do
// catálogo, casamento com as skills e as tecnologias de interesse, e o ranking.
func (h *DiscoveryHandler) rankedJobs(ctx context.Context, userID pgtype.UUID, view dismissView) (ranked, *repository.RepositoryError) {
	var out ranked
	skills, errR := h.Users.QuerySelectAllSkills(ctx, userID.String())
	if errR != nil {
		return out, errR
	}
	prefs, errR := h.Repo.SelectPreferences(ctx, userID)
	if errR != nil {
		return out, errR
	}
	out.filter = discoveryFilter{
		Location:  discovery.LocationPrefs{WorkplaceTypes: prefs.WorkplaceTypes, Cities: prefs.Cities},
		Seniority: discovery.SeniorityPrefs{Levels: prefs.Seniorities, IncludeUnknown: prefs.SeniorityUnknown},
	}

	matchSkills, onlyStrong := strongSkills(skills)
	out.onlyStrong = onlyStrong
	interest := prefs.Technologies
	if len(matchSkills) == 0 && len(interest) == 0 {
		return out, nil
	}
	plan := discovery.PlanSkills(matchSkills)
	activeSince := pgtype.Timestamptz{Time: time.Now().Add(-discovery.ActiveFor), Valid: true}

	hitRows, errR := h.Repo.SkillHits(ctx, db.DiscoverySkillHitsParams{Terms: plan.Terms, ActiveSince: activeSince})
	if errR != nil {
		return out, errR
	}
	hits := map[[16]byte][]string{}
	for _, hr := range hitRows {
		hits[hr.ID.Bytes] = append(hits[hr.ID.Bytes], hr.Term)
	}

	// Com tecnologias de interesse, elas decidem quais vagas entram (mesmo sem skill do
	// perfil); as skills só ordenam. Sem elas, entram as que citam as skills.
	params := db.DiscoveryListCandidatesParams{ActiveSince: activeSince, Techs: interest, ExtraIds: []pgtype.UUID{}, MaxCount: discoveryRankLimit}
	if len(interest) == 0 {
		params.Techs = plan.AllTechs
		for _, hr := range hitRows {
			params.ExtraIds = append(params.ExtraIds, hr.ID)
		}
	}
	rows, errR := h.Repo.ListCandidates(ctx, params)
	if errR != nil {
		return out, errR
	}

	aliasMap, errA := h.Aliases.QuerySelectAllStackAliases(ctx)
	if errA != nil {
		aliasMap = map[string]string{}
	}
	eval := jobfit.NewEvaluator(skills, aliasMap)
	dismissed, errR := h.Repo.DismissedIDs(ctx, userID)
	if errR != nil {
		return out, errR
	}

	// A mesma vaga costuma ser publicada uma vez por cidade, e às vezes em mais de um
	// site: fica a mais recente (a query vem da mais recente para a mais antiga), com
	// os links dos outros sites em AlsoOn. Nenhuma se perde: só não se repete.
	jobs := make([]DiscoveredJob, 0, len(rows))
	kept := map[string]int{} // chave → índice em jobs; -1 = a primeira não casou
	for _, row := range rows {
		// Sem empresa (issues do GitHub) o título sozinho juntaria vagas diferentes.
		key := strings.ToLower(strings.TrimSpace(row.Title) + "|" + strings.TrimSpace(row.Company))
		if row.Company == "" {
			key = row.Url
		}
		if i, ok := kept[key]; ok {
			if i >= 0 {
				jobs[i].addAlsoOn(row.Source, row.Url)
			}
			continue
		}
		kept[key] = -1
		matched := plan.Matched(row.Techs, hits[row.ID.Bytes])
		matchedTechs := intersect(interest, row.Techs)
		if (len(interest) > 0 && len(matchedTechs) == 0) || (len(interest) == 0 && len(matched) == 0) {
			continue
		}
		if isDismissed := dismissed[row.ID.Bytes]; isDismissed != (view == onlyDismissed) {
			if isDismissed {
				out.dismissed++
			}
			continue
		}
		job := toDiscoveredJob(row, matched)
		job.MatchedTechs = matchedTechs
		job.Dismissed = view == onlyDismissed
		job.fit = eval.Evaluate(row.Techs)
		kept[key] = len(jobs)
		jobs = append(jobs, job)
	}
	if view == onlyDismissed {
		out.dismissed = len(jobs)
	}
	rankDiscovered(jobs)
	out.jobs = jobs
	return out, nil
}

// rankDiscovered ordena pela compatibilidade (JobFit): primeiro a faixa (alta, média,
// baixa; sem stack conhecida por último), depois quantas tecnologias da vaga o perfil
// cobre (forte vale o dobro de parcial, para uma vaga que só cita Go não passar na
// frente de uma com Go, PostgreSQL e Docker), a cobertura e as skills citadas.
// Estável: no empate fica a mais recente.
func rankDiscovered(jobs []DiscoveredJob) {
	tier := func(f jobfit.Fit) int {
		if len(f.Items) == 0 {
			return 0
		}
		switch f.Quality {
		case db.JobQualityHigh:
			return 3
		case db.JobQualityMid:
			return 2
		}
		return 1
	}
	sort.SliceStable(jobs, func(a, b int) bool {
		fa, fb := jobs[a].fit, jobs[b].fit
		if ta, tb := tier(fa), tier(fb); ta != tb {
			return ta > tb
		}
		if ca, cb := 2*fa.Strong+fa.Partial, 2*fb.Strong+fb.Partial; ca != cb {
			return ca > cb
		}
		if fa.CoveragePct != fb.CoveragePct {
			return fa.CoveragePct > fb.CoveragePct
		}
		if len(jobs[a].MatchedSkills) != len(jobs[b].MatchedSkills) {
			return len(jobs[a].MatchedSkills) > len(jobs[b].MatchedSkills)
		}
		return len(jobs[a].MatchedTechs) > len(jobs[b].MatchedTechs)
	})
}

// fillPage completa as vagas da página com o que precisa da descrição (trecho) ou da
// pipeline do usuário, e expõe a compatibilidade e a stack restante.
func (h *DiscoveryHandler) fillPage(ctx context.Context, userID pgtype.UUID, jobs []DiscoveredJob, cursor, size int32) *repository.RepositoryError {
	start, end := pageBounds(int32(len(jobs)), cursor, size)
	page := jobs[start:end]
	ids := make([]pgtype.UUID, 0, len(page))
	for _, j := range page {
		var id pgtype.UUID
		if err := id.Scan(j.ID); err == nil {
			ids = append(ids, id)
		}
	}
	details, errR := h.Repo.PageDetails(ctx, db.DiscoveryPageDetailsParams{UserID: userID, Ids: ids})
	if errR != nil {
		return errR
	}
	byID := make(map[string]db.DiscoveryPageDetailsRow, len(details))
	for _, d := range details {
		byID[d.ID.String()] = d
	}
	for i := range page {
		j := &page[i]
		if d, ok := byID[j.ID]; ok {
			j.Snippet = snippet(d.Description, 280)
			if j.Language == "" {
				j.Language = discovery.GuessLanguage(j.Title + "\n" + d.Description)
			}
			if d.PipelineJobID.Valid {
				j.AlreadyAdded = true
				j.PipelineJobID = d.PipelineJobID.String()
				j.PipelineStatus = d.PipelineStatus
			}
		}
		j.Stack = otherStack(*j)
		fit := j.fit
		j.Fit = &fit
	}
	return nil
}

// addAlsoOn guarda o link da mesma vaga em outro site (uma vez por site; outra cidade
// no mesmo site não entra).
func (j *DiscoveredJob) addAlsoOn(source, url string) {
	if source == j.Source || url == "" {
		return
	}
	for _, l := range j.AlsoOn {
		if l.Source == source {
			return
		}
	}
	j.AlsoOn = append(j.AlsoOn, JobLink{Source: source, Url: url})
}

func pageBounds(total, cursor, size int32) (int32, int32) {
	start := min(max(cursor, 0), total)
	return start, min(start+max(size, 1), total)
}

// intersect devolve os itens de want que estão em have, na ordem de want.
func intersect(want, have []string) []string {
	out := []string{}
	for _, w := range want {
		if slices.Contains(have, w) {
			out = append(out, w)
		}
	}
	return out
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

// otherStack devolve as tecnologias citadas na vaga (techs, da varredura) que não são
// as já destacadas (skills do perfil e tecnologias de interesse).
func otherStack(j DiscoveredJob) []string {
	out := []string{}
	skills := make([]discovery.Skill, len(j.MatchedSkills))
	for i, s := range j.MatchedSkills {
		skills[i] = discovery.NewSkill(s)
	}
	for _, t := range j.techs {
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
// casamento caiu para todas as skills), as contagens dos filtros e quantas vagas o
// usuário dispensou.
func writeDiscoveryList(w http.ResponseWriter, res discoveryResult, size, cursor int32, rk ranked) {
	total := int32(len(res.Jobs))
	start, end := pageBounds(total, cursor, size)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":               res.Jobs[start:end],
		"meta":               meta{Size: size, Cursor: cursor, Total: total},
		"only_strong_skills": rk.onlyStrong,
		"facets":             res.Facets,
		"dismissed_count":    rk.dismissed,
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

func toDiscoveredJob(row db.DiscoveryListCandidatesRow, matched []string) DiscoveredJob {
	job := DiscoveredJob{
		ID:            row.ID.String(),
		Source:        row.Source,
		Title:         row.Title,
		Company:       row.Company,
		Location:      row.Location,
		WorkplaceType: row.WorkplaceType,
		Url:           row.Url,
		MatchedSkills: matched,
		MatchedTechs:  []string{},
		International: row.International,
		Seniority:     nonNil(row.Seniority),
		Stack:         []string{},
		Language:      row.Language,
		techs:         row.Techs,
		firstSeen:     row.FirstSeenAt.Time,
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
