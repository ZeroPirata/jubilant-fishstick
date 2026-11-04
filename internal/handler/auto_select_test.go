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

func TestParseAutoVerdicts_Eliminatorio(t *testing.T) {
	got, err := parseAutoVerdicts(`{"avaliacoes":[
		{"vaga":1,"nota":75,"eliminatorio":"VTEX obrigatório","candidatar":true,"motivo":"Forte em Go, mas exige VTEX."},
		{"vaga":2,"nota":80,"eliminatorio":"nenhum","candidatar":true,"motivo":"Go é o núcleo."},
		{"vaga":3,"nota":30,"eliminatorio":"Estágio","candidatar":false,"motivo":""}]}`, 3)
	if err != nil {
		t.Fatal(err)
	}
	if v := got[1]; v.Apply || v.Blocker != "VTEX obrigatório" {
		t.Errorf("eliminatório precisa barrar mesmo com candidatar=true: %+v", v)
	}
	if v := got[2]; !v.Apply || v.Blocker != "" {
		t.Errorf(`"nenhum" não é eliminatório: %+v`, v)
	}
	if v := got[3]; v.Reason != "Eliminatório: Estágio" {
		t.Errorf("motivo vazio usa o eliminatório: %+v", v)
	}
}

func TestAutoExperience(t *testing.T) {
	d := func(s string) pgtype.Date {
		tm, _ := time.Parse("2006-01-02", s)
		return pgtype.Date{Time: tm, Valid: true}
	}
	now, _ := time.Parse("2006-01-02", "2026-01-01")
	exps := []db.UserExperience{
		{StartDate: d("2022-01-01"), EndDate: d("2024-01-01")},
		{StartDate: d("2023-01-01"), EndDate: d("2023-06-01")}, // simultâneo: não soma
		{StartDate: d("2025-01-01"), IsCurrentJob: true},
		{}, // sem data
	}
	total, ok := autoExperience(exps, now)
	if years := total.Hours() / 24 / 365.25; !ok || years < 2.95 || years > 3.05 {
		t.Errorf("total = %.2f anos (ok=%v), esperado 3", years, ok)
	}
	if _, ok := autoExperience([]db.UserExperience{{}}, now); ok {
		t.Error("sem datas deveria dar ok=false")
	}
}

func TestAutoLevelRank(t *testing.T) {
	target, stretch := autoLevelFit(3.6) // pleno; júnior e sênior completam
	for _, tc := range []struct {
		levels []string
		want   int
	}{
		{nil, 0},
		{[]string{"mid"}, 0},
		{[]string{"junior", "mid", "senior"}, 0}, // "all levels"
		{[]string{"senior"}, 1},
		{[]string{"intern"}, -1},
		{[]string{"intern", "junior"}, -1}, // estágio não vira "júnior" para quem já trabalha
		{[]string{"expert"}, -1},
		{[]string{"manager"}, -1},
	} {
		if got := autoLevelRank(tc.levels, target, stretch); got != tc.want {
			t.Errorf("%v: %d, esperado %d", tc.levels, got, tc.want)
		}
	}
}

func TestAutoDedupKey(t *testing.T) {
	a := autoDedupKey("Senior Backend Software Engineer", "MEP | Wellhub (Gympass)")
	b := autoDedupKey("Senior Backend Software Engineer", "MEP | Wellhub")
	if a != b {
		t.Errorf("%q != %q", a, b)
	}
	if autoDedupKey("Senior Backend Software Engineer", "Visa") == a {
		t.Error("empresas diferentes não podem colidir")
	}
}
