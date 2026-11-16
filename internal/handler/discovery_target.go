package handler

import (
	"encoding/json"
	"hackton-treino/internal/db"
	"hackton-treino/internal/discovery"
	"hackton-treino/internal/middleware"
	"net/http"
	"slices"
	"sort"
	"time"
	"unicode/utf8"
)

// ── Alvo do Descobrir ──
// O que a pessoa procura numa frase: função, tecnologias principais, nível e modo de
// trabalho. Fica nas preferências do Descobrir (as mesmas colunas da tela antiga, mais
// a função) e vale para a lista, os avisos, a Seleção com IA e as buscas do LinkedIn.
// Ninguém precisa montar isso item a item: o sistema sugere pelo perfil e entende um
// texto livre ("golang backend remoto pleno").

const (
	targetTextMax     = 300
	targetSuggestTech = 2
)

type targetRole struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// targetView: GET /discovery/target.
type targetView struct {
	// Target: o Alvo salvo. Empty: nada salvo ainda (a tela oferece a sugestão).
	Target discovery.Target `json:"target"`
	Cities []string         `json:"cities"`
	Empty  bool             `json:"empty"`
	// Suggestion: o Alvo montado pelo perfil (cargo das experiências, skills
	// avançadas/expert e tempo de carreira).
	Suggestion discovery.Target `json:"suggestion"`
	Roles      []targetRole     `json:"roles"`
}

func targetRoles() []targetRole {
	out := make([]targetRole, 0, len(discovery.Roles))
	for _, r := range discovery.Roles {
		out = append(out, targetRole{Key: r.Key, Label: r.Label})
	}
	return out
}

// GetTarget (GET /discovery/target): o Alvo salvo e a sugestão pelo perfil.
func (h *DiscoveryHandler) GetTarget(w http.ResponseWriter, r *http.Request) {
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
	skills, errR := h.Users.QuerySelectAllSkills(r.Context(), userID.String())
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	exps, errR := h.Users.QuerySelectAllExperiences(r.Context(), userID.String())
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	t := discovery.Target{
		Roles: nonNil(prefs.Roles), Technologies: nonNil(prefs.Technologies),
		Seniorities: nonNil(prefs.Seniorities), WorkplaceTypes: nonNil(prefs.WorkplaceTypes),
	}
	writeJSON(w, http.StatusOK, targetView{
		Target:     t,
		Cities:     nonNil(prefs.Cities),
		Empty:      len(t.Roles)+len(t.Technologies)+len(t.Seniorities)+len(t.WorkplaceTypes)+len(prefs.Cities) == 0,
		Suggestion: suggestTarget(exps, skills, t.WorkplaceTypes, time.Now()),
		Roles:      targetRoles(),
	})
}

// suggestTarget monta o Alvo pelo perfil:
//   - função: a dos cargos das experiências, da mais recente para a mais antiga (até 2);
//   - tecnologias: as linguagens do catálogo cobertas pelas skills avançadas/expert
//     (até 2; "Go (Golang)" expert vira Go); sem linguagem, as ferramentas;
//   - nível: o do tempo de carreira (o mesmo da Seleção com IA);
//   - modo de trabalho: o que já estava salvo.
func suggestTarget(exps []db.UserExperience, skills []db.UserSkill, workplace []string, now time.Time) discovery.Target {
	t := discovery.Target{Roles: []string{}, Technologies: []string{}, Seniorities: []string{}, WorkplaceTypes: nonNil(workplace)}

	sorted := slices.Clone(exps)
	sort.SliceStable(sorted, func(a, b int) bool {
		if sorted[a].IsCurrentJob != sorted[b].IsCurrentJob {
			return sorted[a].IsCurrentJob
		}
		return sorted[a].StartDate.Time.After(sorted[b].StartDate.Time)
	})
	for _, e := range sorted {
		for _, k := range discovery.ClassifyRoles(e.JobRole) {
			if role, _ := discovery.RoleByKey(k); role.Tech && !slices.Contains(t.Roles, k) && len(t.Roles) < 2 {
				t.Roles = append(t.Roles, k)
			}
		}
	}

	strong := slices.Clone(skills)
	strong = slices.DeleteFunc(strong, func(s db.UserSkill) bool {
		return s.ProficiencyLevel != db.SkillLevelExpert && s.ProficiencyLevel != db.SkillLevelAdvanced
	})
	sort.SliceStable(strong, func(a, b int) bool {
		return strong[a].ProficiencyLevel == db.SkillLevelExpert && strong[b].ProficiencyLevel != db.SkillLevelExpert
	})
	var langs, others []string
	for _, s := range strong {
		skill := discovery.NewSkill(s.SkillName)
		for _, tech := range discovery.Technologies {
			if !discovery.TechCoveredBy(tech.Name, skill) || slices.Contains(langs, tech.Name) || slices.Contains(others, tech.Name) {
				continue
			}
			if tech.Group == "Linguagens" {
				langs = append(langs, tech.Name)
			} else {
				others = append(others, tech.Name)
			}
		}
	}
	// Só as linguagens: banco e ferramentas (PostgreSQL, Docker) estão em quase toda
	// vaga e alargariam a lista e a busca do LinkedIn. Sem linguagem forte, as
	// ferramentas mais fortes.
	pick := langs
	if len(pick) == 0 {
		pick = others
	}
	t.Technologies = append(t.Technologies, pick[:min(targetSuggestTech, len(pick))]...)

	if total, ok := autoExperience(exps, now); ok {
		level, _ := autoLevelFit(total.Hours() / 24 / 365.25)
		t.Seniorities = level
	}
	return t
}

// ParseTarget (POST /discovery/target/parse {text}): o que o texto livre quer dizer.
// Não salva: a tela mostra o que entendeu e a pessoa confirma.
func (h *DiscoveryHandler) ParseTarget(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}
	if utf8.RuneCountInString(body.Text) > targetTextMax {
		writeError(w, http.StatusBadRequest, "texto longo demais (até 300 caracteres)")
		return
	}
	writeJSON(w, http.StatusOK, discovery.ParseTarget(body.Text))
}

// SaveTarget (PUT /discovery/target): grava função, tecnologias, nível e modo de
// trabalho. Cidades e "incluir vagas sem nível" ficam como estavam.
func (h *DiscoveryHandler) SaveTarget(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	var body discovery.Target
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}
	saved, errR := h.Repo.SelectPreferences(r.Context(), userID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	unknown := saved.SeniorityUnknown
	// Primeira vez (nada salvo): vaga sem nível entra, como no resto do Descobrir.
	if len(saved.Seniorities) == 0 && len(saved.Cities) == 0 && len(saved.Technologies) == 0 && len(saved.WorkplaceTypes) == 0 && len(saved.Roles) == 0 {
		unknown = true
	}
	prefs, errR := h.savePreferences(r.Context(), userID, DiscoveryPreferences{
		WorkplaceTypes: nonNil(body.WorkplaceTypes), Cities: nonNil(saved.Cities), Technologies: nonNil(body.Technologies),
		Seniorities: nonNil(body.Seniorities), SeniorityUnknown: &unknown, Roles: nonNil(body.Roles),
	})
	if errR != nil {
		writeError(w, errR.StatusCode, errR.Message)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}
