package worker

import (
	"hackton-treino/internal/db"
	"hackton-treino/internal/scraper"
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
			got := techMatch(tc.item, tc.filtro)
			if got != tc.want {
				t.Errorf("techMatch(%q, %q) = %v, want %v", tc.item, tc.filtro, got, tc.want)
			}
		})
	}
}

// TestCalcularQualidade testa os limiares de qualidade e a integração com aliases.
func TestCalcularQualidade(t *testing.T) {
	noAliases := map[string]string{}

	cases := []struct {
		label   string
		stack   []string
		filtros []string
		aliases map[string]string
		want    db.JobQuality
	}{
		// --- edge cases ---
		{
			label:   "stack vazia retorna mid",
			stack:   []string{},
			filtros: []string{"go"},
			aliases: noAliases,
			want:    db.JobQualityMid,
		},
		{
			label:   "filtros vazios retorna mid",
			stack:   []string{"Go", "PostgreSQL"},
			filtros: []string{},
			aliases: noAliases,
			want:    db.JobQualityMid,
		},

		// --- limiares de qualidade ---
		{
			label:   "0% match resulta em low",
			stack:   []string{"Java", "Spring", "Maven"},
			filtros: []string{"go", "postgresql"},
			aliases: noAliases,
			want:    db.JobQualityLow,
		},
		{
			label:   "33% match (1/3) resulta em mid",
			stack:   []string{"Go", "Java", "Spring"},
			filtros: []string{"go"},
			aliases: noAliases,
			want:    db.JobQualityMid,
		},
		{
			label:   "100% match resulta em high",
			stack:   []string{"Go", "PostgreSQL"},
			filtros: []string{"go", "postgresql"},
			aliases: noAliases,
			want:    db.JobQualityHigh,
		},
		{
			label: "70% match (7/10) resulta em high",
			stack: []string{
				"Go", "PostgreSQL", "Redis", "Docker",
				"Kubernetes", "gRPC", "REST", "AWS", "Kafka", "GraphQL",
			},
			filtros: []string{"go", "postgresql", "redis", "docker", "kubernetes", "grpc", "rest"},
			aliases: noAliases,
			want:    db.JobQualityHigh,
		},
		{
			label:   "limiar exato 30% resulta em mid",
			stack:   []string{"Go", "Java", "Python"},
			filtros: []string{"go"},
			aliases: noAliases,
			want:    db.JobQualityMid, // 1/3 = 33% >= 30%
		},

		// --- normalização de aliases ---
		// "golang" → "go" via alias bate em item "go"; "postgresql" bate diretamente
		{
			label:   "alias normaliza filtro antes do match",
			stack:   []string{"Go", "PostgreSQL"},
			filtros: []string{"golang", "postgresql"},
			aliases: map[string]string{"golang": "go"},
			want:    db.JobQualityHigh,
		},
		// "postgres" (sem alias para "postgresql") não bate — 0/2 → low
		{
			label:   "postgres sem alias nao bate em postgresql na stack",
			stack:   []string{"Go", "PostgreSQL"},
			filtros: []string{"postgres"},
			aliases: noAliases,
			want:    db.JobQualityLow,
		},

		// --- regressão: falso positivo do código antigo ---
		// "go" como filtro não deve bater em "Django" na stack
		{
			label:   "filtro go nao bate em Django na stack (regressao)",
			stack:   []string{"Django", "Python", "PostgreSQL"},
			filtros: []string{"go"},
			aliases: noAliases,
			want:    db.JobQualityLow, // 0/3 = 0%
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			result := &scraper.ResultScraper{
				NLScraperResult: scraper.NLScraperResult{Stack: tc.stack},
			}
			got := calcularQualidade(result, tc.filtros, tc.aliases)
			if got != tc.want {
				t.Errorf("calcularQualidade() = %q, want %q", got, tc.want)
			}
		})
	}
}
