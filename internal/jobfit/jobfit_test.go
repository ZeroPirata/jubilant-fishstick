package jobfit

import (
	"hackton-treino/internal/db"
	"testing"
)

// TestTechMatch cobre os casos de match e os falsos positivos corrigidos.
// Cada sub-teste tem um label descritivo para facilitar o diagnóstico de falhas.
func TestTechMatch(t *testing.T) {
	cases := []struct {
		label  string
		item   string
		filtro string
		want   bool
	}{
		// --- matches esperados ---
		{label: "exact match simples", item: "go", filtro: "go", want: true},
		{label: "exact match multi-palavra", item: "postgresql", filtro: "postgresql", want: true},
		// "postgres" não bate em "postgresql": next char é 'q' (alphanumeric, sem boundary)
		// para isso o usuário deve criar alias "postgres" → "postgresql"
		{label: "postgres sem alias nao bate em postgresql", item: "postgresql", filtro: "postgres", want: false},
		{label: "prefix antes de separador ponto", item: "node.js", filtro: "node", want: true},
		{label: "prefix multi-token", item: "event-driven architecture", filtro: "event-driven", want: true},
		{label: "token do meio via barra", item: "gitlab ci/cd", filtro: "ci", want: true},
		{label: "primeiro token", item: "gitlab ci/cd", filtro: "gitlab", want: true},
		{label: "segundo token apos ponto", item: "node.js", filtro: "js", want: true},
		{label: "token em item com hifen", item: "event-driven", filtro: "driven", want: true},

		// --- falsos positivos que o código antigo cometia ---
		{label: "go nao bate em django (sufixo)", item: "django", filtro: "go", want: false},
		{label: "go nao bate em gorilla", item: "gorilla", filtro: "go", want: false},
		{label: "go nao bate em mongodb", item: "mongodb", filtro: "go", want: false},
		{label: "go nao bate em cargo", item: "cargo", filtro: "go", want: false},

		// --- sem match ---
		{label: "tecnologias diferentes", item: "python", filtro: "go", want: false},
		{label: "ferramentas diferentes", item: "kubernetes", filtro: "docker", want: false},

		// aliases são normalizados antes de chegar ao techMatch;
		// sem alias, "golang" não deve bater em "go"
		{label: "golang sem alias nao bate em go", item: "go", filtro: "golang", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := TechMatch(tc.item, tc.filtro)
			if got != tc.want {
				t.Errorf("TechMatch(%q, %q) = %v, want %v", tc.item, tc.filtro, got, tc.want)
			}
		})
	}
}

// TestEvaluate_Qualidade testa os limiares de qualidade e a integração com aliases.
func TestEvaluate_Qualidade(t *testing.T) {
	noAliases := map[string]string{}

	cases := []struct {
		label   string
		stack   []string
		filtros []db.UserSkill
		aliases map[string]string
		want    db.JobQuality
	}{
		// --- edge cases ---
		{
			label:   "stack vazia retorna mid",
			stack:   []string{},
			filtros: []db.UserSkill{{SkillName: "go"}},
			aliases: noAliases,
			want:    db.JobQualityMid,
		},
		{
			label:   "filtros vazios retorna mid",
			stack:   []string{"Go", "PostgreSQL"},
			filtros: []db.UserSkill{},
			aliases: noAliases,
			want:    db.JobQualityMid,
		},

		// --- limiares de qualidade ---
		{
			label: "0% match resulta em low",
			stack: []string{"Java", "Spring", "Maven"},
			filtros: []db.UserSkill{
				{SkillName: "go"}, {SkillName: "postgresql"},
			},
			aliases: noAliases,
			want:    db.JobQualityLow,
		},
		{
			label: "33% match (1/3) resulta em mid",
			stack: []string{"Go", "Java", "Spring"},
			filtros: []db.UserSkill{
				{SkillName: "go"},
			},
			aliases: noAliases,
			want:    db.JobQualityMid,
		},
		{
			label: "100% match resulta em high",
			stack: []string{"Go", "PostgreSQL", "Docker"},
			filtros: []db.UserSkill{
				{SkillName: "go"}, {SkillName: "postgresql"}, {SkillName: "docker"},
			},
			aliases: noAliases,
			want:    db.JobQualityHigh,
		},
		// Vaga que cita só 1 ou 2 tecnologias diz pouco: no máximo média.
		{
			label: "100% com so 2 tecnologias fica mid",
			stack: []string{"Go", "PostgreSQL"},
			filtros: []db.UserSkill{
				{SkillName: "go"}, {SkillName: "postgresql"},
			},
			aliases: noAliases,
			want:    db.JobQualityMid,
		},
		{
			label:   "100% com so 1 tecnologia fica mid",
			stack:   []string{"Go"},
			filtros: []db.UserSkill{{SkillName: "go"}},
			aliases: noAliases,
			want:    db.JobQualityMid,
		},
		{
			label: "70% match (7/10) resulta em high",
			stack: []string{
				"Go", "PostgreSQL", "Redis", "Docker",
				"Kubernetes", "gRPC", "REST", "AWS", "Kafka", "GraphQL",
			},
			filtros: []db.UserSkill{
				{SkillName: "go"}, {SkillName: "postgresql"},
				{SkillName: "redis"}, {SkillName: "docker"},
				{SkillName: "kubernetes"}, {SkillName: "grpc"},
				{SkillName: "rest"},
			},
			aliases: noAliases,
			want:    db.JobQualityHigh,
		},
		{
			label: "limiar exato 30% resulta em mid",
			stack: []string{"Go", "Java", "Python"},
			filtros: []db.UserSkill{
				{SkillName: "go"},
			},
			aliases: noAliases,
			want:    db.JobQualityMid, // 1/3 = 33% >= 30%
		},

		// --- normalização de aliases ---
		// "golang" → "go" via alias bate em item "go"; "postgresql" bate diretamente
		{
			label: "alias normaliza filtro antes do match",
			stack: []string{"Go", "PostgreSQL", "Redis"},
			filtros: []db.UserSkill{
				{SkillName: "golang"}, {SkillName: "postgresql"}, {SkillName: "redis"},
			},
			aliases: map[string]string{"golang": "go"},
			want:    db.JobQualityHigh,
		},
		// "postgres" (sem alias para "postgresql") não bate — 0/2 → low
		{
			label: "postgres sem alias nao bate em postgresql na stack",
			stack: []string{"Go", "PostgreSQL"},
			filtros: []db.UserSkill{
				{SkillName: "postgres"},
			},
			aliases: noAliases,
			want:    db.JobQualityLow,
		},

		// --- regressão: falso positivo do código antigo ---
		// "go" como filtro não deve bater em "Django" na stack
		{
			label:   "filtro go nao bate em Django na stack (regressao)",
			stack:   []string{"Django", "Python", "PostgreSQL"},
			filtros: []db.UserSkill{{SkillName: "go"}},
			aliases: noAliases,
			want:    db.JobQualityLow, // 0/3 = 0%
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := Evaluate(tc.stack, tc.filtros, tc.aliases).Quality
			if got != tc.want {
				t.Errorf("Evaluate().Quality = %q, want %q", got, tc.want)
			}
		})
	}
}

// Vaga real (Sezzle): o nível da skill pesa e cada item diz qual skill o cobre.
func TestEvaluate_PesoPorNivel(t *testing.T) {
	skills := []db.UserSkill{
		{SkillName: "Go (Golang)", ProficiencyLevel: db.SkillLevelExpert, Tags: []string{"golang", "go", "backend"}},
		{SkillName: "React.js", ProficiencyLevel: db.SkillLevelBasic, Tags: []string{"react", "frontend"}},
		{SkillName: "AWS SQS", ProficiencyLevel: db.SkillLevelAdvanced, Tags: []string{"aws", "sqs"}},
		{SkillName: "Amazon S3", ProficiencyLevel: db.SkillLevelIntermediate, Tags: []string{"aws", "s3"}},
		{SkillName: "MySQL", ProficiencyLevel: db.SkillLevelIntermediate},
		// A tag "mysql" de uma skill avançada não passa por cima da skill MySQL.
		{SkillName: "SQL", ProficiencyLevel: db.SkillLevelAdvanced, Tags: []string{"sql", "mysql"}},
	}
	fit := Evaluate([]string{"go", "react", "aws", "mysql", "kubernetes"}, skills, nil)

	want := map[string]Item{
		"go":         {Status: StatusStrong, Skill: "Go (Golang)", Level: "expert"},
		"react":      {Status: StatusPartial, Skill: "React.js", Level: "basic"},
		"aws":        {Status: StatusStrong, Skill: "AWS SQS", Level: "advanced"}, // o maior nível vence
		"mysql":      {Status: StatusPartial, Skill: "MySQL", Level: "intermediate"},
		"kubernetes": {Status: StatusMissing},
	}
	for _, it := range fit.Items {
		w := want[it.Name]
		if it.Status != w.Status || it.Skill != w.Skill || it.Level != w.Level {
			t.Errorf("%s: %+v, esperado %+v", it.Name, it, w)
		}
	}
	// (1 + 0.4 + 1 + 0.75 + 0) / 5 = 63%
	if fit.CoveragePct != 63 || fit.Quality != db.JobQualityMid {
		t.Errorf("cobertura %d%% %s", fit.CoveragePct, fit.Quality)
	}
	if fit.Strong != 2 || fit.Partial != 2 || fit.Missing != 1 {
		t.Errorf("contagens %d/%d/%d", fit.Strong, fit.Partial, fit.Missing)
	}
}
