package handler

import (
	"hackton-treino/internal/db"
	"hackton-treino/internal/discovery"
	"hackton-treino/internal/jobfit"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// O caso do "golang backend": com o Alvo Back-end + Go, a vaga de back-end com Go no
// título vem primeiro; depois "Desenvolvedor Go", "Backend Developer" e full stack; uma
// vaga que só cita Go no texto, por último, mesmo cobrindo mais a stack.
func TestRankDiscoveredTarget(t *testing.T) {
	high := jobfit.Fit{Quality: db.JobQualityHigh, CoveragePct: 90, Strong: 4, Items: make([]jobfit.Item, 4)}
	low := jobfit.Fit{Quality: db.JobQualityLow, CoveragePct: 20, Strong: 1, Items: make([]jobfit.Item, 4)}
	jobs := []DiscoveredJob{
		{ID: "so-cita", fit: high, RoleFit: discovery.RoleUnknown},
		{ID: "backend", fit: low, RoleFit: discovery.RoleMatch},
		{ID: "dev-go", fit: low, RoleFit: discovery.RoleUnknown, TitleTechs: []string{"Go"}},
		{ID: "backend-go", fit: low, RoleFit: discovery.RoleMatch, TitleTechs: []string{"Go"}},
		{ID: "fullstack", fit: high, RoleFit: discovery.RoleNeighbor},
	}
	rankDiscovered(jobs)
	var got []string
	for _, j := range jobs {
		got = append(got, j.ID)
	}
	names(t, got, "backend-go", "dev-go", "backend", "fullstack", "so-cita")
}

func TestFilterDiscoveredOtherRole(t *testing.T) {
	jobs := []DiscoveredJob{
		{ID: "1", RoleFit: discovery.RoleMatch},
		{ID: "2", RoleFit: discovery.RoleOther},
		{ID: "3", RoleFit: discovery.RoleUnknown},
		{ID: "4", RoleFit: discovery.RoleNeighbor},
	}
	f := discoveryFilter{Roles: []string{"backend"}}
	res := filterDiscovered(jobs, f)
	if len(res.Jobs) != 3 || res.OtherRole != 1 {
		t.Errorf("com Alvo: %d vagas, %d escondidas", len(res.Jobs), res.OtherRole)
	}
	f.AllRoles = true
	if res := filterDiscovered(jobs, f); len(res.Jobs) != 4 || res.OtherRole != 0 {
		t.Errorf("?all_roles=1: %d vagas, %d escondidas", len(res.Jobs), res.OtherRole)
	}
	if res := filterDiscovered(jobs, discoveryFilter{}); len(res.Jobs) != 4 {
		t.Errorf("sem Alvo: %d vagas", len(res.Jobs))
	}
}

func date(y int, m time.Month) pgtype.Date {
	return pgtype.Date{Time: time.Date(y, m, 1, 0, 0, 0, 0, time.UTC), Valid: true}
}

func TestSuggestTarget(t *testing.T) {
	now := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	exps := []db.UserExperience{
		{JobRole: "Estagiário de Suporte", StartDate: date(2021, 1), EndDate: date(2021, 12)},
		{JobRole: "Desenvolvedor Back-end", StartDate: date(2022, 1), IsCurrentJob: true},
	}
	skills := []db.UserSkill{
		{SkillName: "Docker", ProficiencyLevel: db.SkillLevelAdvanced},
		{SkillName: "Go (Golang)", ProficiencyLevel: db.SkillLevelExpert},
		{SkillName: "PostgreSQL", ProficiencyLevel: db.SkillLevelExpert},
		{SkillName: "Java", ProficiencyLevel: db.SkillLevelBasic},
	}
	got := suggestTarget(exps, skills, []string{"remote"}, now)
	// Suporte não é função de desenvolvimento: não entra. Só a linguagem vira
	// tecnologia do Alvo: PostgreSQL e Docker alargariam a lista e a busca.
	want := discovery.Target{Roles: []string{"backend"}, Technologies: []string{"Go"},
		Seniorities: []string{"mid"}, WorkplaceTypes: []string{"remote"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sugestão = %+v\nquer %+v", got, want)
	}

	empty := suggestTarget(nil, nil, nil, now)
	if empty.Roles == nil || empty.Technologies == nil || empty.Seniorities == nil || empty.WorkplaceTypes == nil {
		t.Errorf("perfil vazio devolveu lista nula: %+v", empty)
	}
}

func TestCleanPreferencesRoles(t *testing.T) {
	p, err := cleanPreferences(DiscoveryPreferences{Roles: []string{"backend", "backend", "data"}})
	if err != nil || !reflect.DeepEqual(p.Roles, []string{"backend", "data"}) {
		t.Errorf("funções = %v (%v)", p.Roles, err)
	}
	if _, err := cleanPreferences(DiscoveryPreferences{Roles: []string{"astronauta"}}); err == nil {
		t.Error("aceitou função desconhecida")
	}
}

// Sem linguagem entre as skills fortes: as duas primeiras ferramentas.
func TestSuggestTargetWithoutLanguage(t *testing.T) {
	skills := []db.UserSkill{
		{SkillName: "Docker", ProficiencyLevel: db.SkillLevelAdvanced},
		{SkillName: "Kubernetes", ProficiencyLevel: db.SkillLevelExpert},
		{SkillName: "Terraform", ProficiencyLevel: db.SkillLevelAdvanced},
	}
	got := suggestTarget(nil, skills, nil, time.Now())
	if len(got.Technologies) != 2 || got.Technologies[0] != "Kubernetes" {
		t.Errorf("tecnologias = %v", got.Technologies)
	}
}
