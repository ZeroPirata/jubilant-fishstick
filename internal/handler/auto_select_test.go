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

func TestAutoDedup(t *testing.T) {
	d := autoDedup{}
	for _, tc := range []struct {
		title, company string
		dup            bool
	}{
		{"Senior Backend Software Engineer", "MEP | Wellhub (Gympass)", false},
		{"Senior Backend Software Engineer", "MEP | Wellhub", true},
		{"Analista de SRE Sr (vaga remota)", "Méliuz", false},
		{"Analista de SRE Sr (vaga remota)", "Carreiras Méliuz", true},
		{"Senior Backend Software Engineer", "Visa", false},
		{"Data Engineer", "", false},
		{"Data Engineer", "", true},
	} {
		if got := d.seen(tc.title, tc.company); got != tc.dup {
			t.Errorf("%s | %s: repetida=%v, esperado %v", tc.title, tc.company, got, tc.dup)
		}
	}
}

func TestParseAutoVerdicts_Fatos(t *testing.T) {
	got, err := parseAutoVerdicts(`{"avaliacoes":[
		{"vaga":1,"anos_pedidos":"5+","modelo":"Híbrido","faltando":["Ruby on Rails","Vue.js"],"nota":70,"candidatar":true,"motivo":"x"},
		{"vaga":2,"anos_pedidos":null,"modelo":"","faltando":"nenhum","nota":80,"candidatar":true,"motivo":"y"}]}`, 2)
	if err != nil {
		t.Fatal(err)
	}
	if v := got[1]; v.YearsAsked != 5 || v.Workplace != "hybrid" || len(v.Missing) != 2 || !v.Missing[0].Central {
		t.Errorf("vaga 1: %+v", v)
	}
	if v := got[2]; v.YearsAsked != 0 || v.Workplace != "" || len(v.Missing) != 0 {
		t.Errorf("vaga 2: %+v", v)
	}
}

func TestAutoApplyRules(t *testing.T) {
	remote := []string{"remote"}
	for _, tc := range []struct {
		name      string
		v         autoVerdict
		job       DiscoveredJob
		apply     bool
		score     int
		reasonHas string
	}{
		{"ok", autoVerdict{Score: 80, Apply: true, YearsAsked: 3}, DiscoveredJob{}, true, 80, ""},
		{"anos demais", autoVerdict{Score: 70, Apply: true, YearsAsked: 6}, DiscoveredJob{}, false, 70, "pede 6+ anos"},
		{"5 anos para 3,6 passa", autoVerdict{Score: 70, Apply: true, YearsAsked: 5}, DiscoveredJob{}, true, 70, ""},
		{"híbrido sem local no site", autoVerdict{Score: 80, Apply: true, Workplace: "hybrid"}, DiscoveredJob{}, false, 80, "híbrido"},
		{"site diz remoto: vale o site", autoVerdict{Score: 80, Apply: true, Workplace: "hybrid"}, DiscoveredJob{WorkplaceType: "remote"}, true, 80, ""},
		{"dois faltando", autoVerdict{Score: 70, Apply: true, Missing: []autoMissing{{"Ruby on Rails", true}, {"Vue.js", true}}}, DiscoveredJob{}, false, 70, "Ruby on Rails, Vue.js"},
		{"um central limita a nota", autoVerdict{Score: 85, Apply: true, Missing: []autoMissing{{"Machine Learning", true}}}, DiscoveredJob{}, true, 65, ""},
		{"secundários tiram 5 cada", autoVerdict{Score: 80, Apply: true, Missing: []autoMissing{{"HTML5", false}, {"CSS3", false}}}, DiscoveredJob{}, true, 70, ""},
		{"1 central + 2 secundários", autoVerdict{Score: 80, Apply: true, Missing: []autoMissing{{"React", true}, {"Jest", false}, {"Next.js", false}}}, DiscoveredJob{}, false, 80, "React, Jest, Next.js"},
		{"4 secundários", autoVerdict{Score: 80, Apply: true, Missing: []autoMissing{{"a", false}, {"b", false}, {"c", false}, {"d", false}}}, DiscoveredJob{}, false, 80, "a, b, c, d"},
		{"eliminatório", autoVerdict{Score: 80, Apply: true, Blocker: "cidadania"}, DiscoveredJob{}, false, 80, ""},
	} {
		v := tc.v
		autoApplyRules(&v, tc.job, 3.6, remote)
		if v.Apply != tc.apply || v.Score != tc.score || !strings.Contains(v.Reason, tc.reasonHas) {
			t.Errorf("%s: apply=%v score=%d reason=%q", tc.name, v.Apply, v.Score, v.Reason)
		}
	}
	// Carreira desconhecida: a regra de anos não se aplica.
	v := autoVerdict{Score: 70, Apply: true, YearsAsked: 10}
	if autoApplyRules(&v, DiscoveredJob{}, -1, nil); !v.Apply {
		t.Error("sem tempo de carreira não deveria cortar por anos")
	}
	for in, want := range map[string]string{"Remoto": "remote", "home office": "remote", "híbrido": "hybrid", "Hybrid": "hybrid", "presencial": "onsite", "": ""} {
		if got := rawWorkplace(in); got != want {
			t.Errorf("rawWorkplace(%q) = %q, esperado %q", in, got, want)
		}
	}
	stretch := DiscoveredJob{autoStretch: true, Seniority: []string{"senior"}}
	for _, tc := range []struct {
		v     autoVerdict
		apply bool
	}{
		{autoVerdict{Score: 85, Apply: true}, true},
		{autoVerdict{Score: 70, Apply: true}, false},
		{autoVerdict{Score: 90, Apply: true, Missing: []autoMissing{{"Kafka", false}}}, false},
	} {
		v := tc.v
		autoApplyRules(&v, stretch, 3.6, remote)
		if v.Apply != tc.apply {
			t.Errorf("nível vizinho %+v: apply=%v, esperado %v (%s)", tc.v, v.Apply, tc.apply, v.Reason)
		}
	}
	for _, tc := range []struct {
		title, desc string
		want        bool
	}{
		{"[Vaga Afirmativa Mulheres] Software Engineer Sênior", "", true},
		{"Especialista - Vaga Afirmativa para Pretos e Pard", "", true},
		{"Backend Engineer", "Esta é uma vaga exclusiva para mulheres cis e trans.", true},
		{"Backend Engineer", "Processo seletivo afirmativo para pessoas negras.", true},
		{"Backend Engineer", "Encorajamos mulheres, pessoas negras e PcD a se candidatarem.", false},
		{"Backend Engineer", "Esta posição também é destinada a pessoas com deficiência.", false},
		{"Backend Engineer", "A Experian é uma empregadora que promove igualdade de oportunidades e ação afirmativa.", false},
		{"Backend Engineer", "Além das vagas afirmativas, todas as vagas Stone também são destinadas a PcD.", false},
		{"Engenheira de Software Sênior", "Esta vaga é destinada exclusivamente para mulheres.", true},
		{"Freelancer de Áudio", "Vaga exclusiva para pessoas que residem no Brasil.", false},
		{"Técnico de Redes", "Esta é uma vaga exclusiva para pessoas com deficiência (PcD).", true},
		{"Senior Software Engineer", "", false},
	} {
		if got := autoIsAffirmative(tc.title, tc.desc); got != tc.want {
			t.Errorf("autoIsAffirmative(%q, %q) = %v", tc.title, tc.desc, got)
		}
	}
	if !autoIsNonTech("Account Executive, Mid-Market - Brazil") || autoIsNonTech("Backend Engineer") {
		t.Error("autoIsNonTech")
	}
}

func TestRawMissing(t *testing.T) {
	got := rawMissing([]byte(`[{"item":"React","peso":"central"},{"item":"Jest","peso":"secundário"},{"item":"nenhum","peso":"central"}]`))
	if len(got) != 2 || !got[0].Central || got[1].Central {
		t.Errorf("objetos: %+v", got)
	}
	if got := rawMissing([]byte(`"Kafka, Terraform"`)); len(got) != 2 || !got[1].Central {
		t.Errorf("texto: %+v", got)
	}
}

func TestAutoVerifyMissing(t *testing.T) {
	skills := []db.UserSkill{
		{SkillName: "Go (Golang)", ProficiencyLevel: db.SkillLevelExpert},
		{SkillName: "TypeScript", ProficiencyLevel: db.SkillLevelIntermediate},
		{SkillName: "React.js", ProficiencyLevel: db.SkillLevelBasic},
		{SkillName: "AWS SQS", ProficiencyLevel: db.SkillLevelAdvanced, Tags: []string{"aws", "sqs"}},
	}
	eval := jobfit.NewEvaluator(skills, nil)
	v := autoVerdict{Missing: []autoMissing{
		{"Kafka ou SQS", true},                  // tem SQS: sai
		{"TypeScript", false},                   // intermediário: sai
		{"React", true},                         // só básico: fica
		{"Terraform ou OpenTofu", true},         // nenhuma das duas: fica
		{"Golang (or similar)", false},          // tem: sai
		{"frontend avançado (Vue/React)", true}, // React básico, Vue não: fica
	}}
	autoVerifyMissing(&v, eval)
	var names []string
	for _, m := range v.Missing {
		names = append(names, m.Item)
	}
	if strings.Join(names, "|") != "React|Terraform ou OpenTofu|frontend avançado (Vue/React)" {
		t.Errorf("ficaram %v", names)
	}
}
