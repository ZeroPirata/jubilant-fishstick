package handler

import (
	"hackton-treino/internal/db"
	"hackton-treino/internal/jobfit"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestParseAutoVerdicts(t *testing.T) {
	got, err := parseAutoVerdicts(`{"avaliacoes":[
		{"vaga":1,"nota":82,"candidatar":true,"motivo":"Stack principal bate."},
		{"vaga":"2","nota":"45.6","candidatar":"false","motivo":"Pede 8 anos."},
		{"vaga":9,"nota":90,"candidatar":true,"motivo":"fora da lista"},
		{"vaga":3,"nota":140,"candidatar":"sim","motivo":""}]}`, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("esperava 3 avaliações, veio %d: %+v", len(got), got)
	}
	if v := got[1]; v.Score != 82 || !v.Apply {
		t.Errorf("vaga 1: %+v", v)
	}
	if v := got[2]; v.Score != 46 || v.Apply {
		t.Errorf("vaga 2: %+v", v)
	}
	if v := got[3]; v.Score != 100 || !v.Apply {
		t.Errorf("vaga 3 (nota limitada a 100): %+v", v)
	}
	if _, err := parseAutoVerdicts(`{"avaliacoes":[]}`, 3); err == nil {
		t.Error("resposta sem avaliação deveria ser erro (o próximo modelo tenta)")
	}
}

func TestAutoChoose(t *testing.T) {
	v := func(score int, apply bool) *autoVerdict { return &autoVerdict{Score: score, Apply: apply} }
	job := func(pct int) DiscoveredJob { return DiscoveredJob{fit: jobfit.Fit{CoveragePct: pct}} }
	cands := []autoCandidate{
		{Job: job(50), Verdict: v(70, true)},
		{Job: job(90), Verdict: v(95, false)}, // nota alta, mas a IA disse não
		{Job: job(80), Verdict: nil},          // sem resposta
		{Job: job(60), Verdict: v(55, true)},  // abaixo do mínimo
		{Job: job(90), Verdict: v(70, true)},  // empata com a 0, ganha pela stack
		{Job: job(40), Verdict: v(88, true)},
	}
	got := autoChoose(cands, 60, 2)
	if len(got) != 2 || got[0] != 5 || got[1] != 4 {
		t.Fatalf("escolha = %v, esperava [5 4]", got)
	}
}

func TestAutoProfileTextTotalExperience(t *testing.T) {
	now := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	d := func(y int, m time.Month) pgtype.Date {
		return pgtype.Date{Time: time.Date(y, m, 1, 0, 0, 0, 0, time.UTC), Valid: true}
	}
	exps := []db.UserExperience{
		{JobRole: "Backend", CompanyName: "A", IsCurrentJob: true, StartDate: d(2024, 11), TechStack: []string{"Go"}},
		// Freela em paralelo: não pode somar duas vezes.
		{JobRole: "Freela", CompanyName: "B", StartDate: d(2025, 1), EndDate: d(2025, 6)},
		{JobRole: "Estágio", CompanyName: "C", StartDate: d(2023, 11), EndDate: d(2024, 11)},
	}
	skills := []db.UserSkill{{SkillName: "Go", ProficiencyLevel: db.SkillLevelExpert}}
	txt := autoProfileText(exps, skills, discoveryFilter{}, now)
	if !strings.Contains(txt, "Tempo total de experiência profissional: 3 anos") {
		t.Errorf("tempo total errado:\n%s", txt)
	}
	if !strings.Contains(txt, "- expert: Go") || !strings.Contains(txt, "stack: Go") {
		t.Errorf("skills/stack ausentes:\n%s", txt)
	}
}

func TestValidateAutoSelectSettings(t *testing.T) {
	ok := AutoSelectSettings{Enabled: true, PerRun: 5, PerDay: 10, Candidates: 20, MinScore: 60, MinQuality: "mid", Mode: "full"}
	if err := validateAutoSelectSettings(ok); err != nil {
		t.Fatal(err)
	}
	bad := ok
	bad.PerRun = 12
	if validateAutoSelectSettings(bad) == nil {
		t.Error("por clique acima do diário deveria falhar")
	}
	bad = ok
	bad.Candidates = 4
	if validateAutoSelectSettings(bad) == nil {
		t.Error("ler menos vagas do que escolhe deveria falhar")
	}
}
