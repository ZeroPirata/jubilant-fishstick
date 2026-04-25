package worker

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"hackton-treino/internal/db"
	"hackton-treino/internal/services"

	"github.com/jackc/pgx/v5/pgtype"
)

// --- helpers ---

func makeJob(company, title string, stack, requirements []string) *db.Job {
	return &db.Job{
		CompanyName:  pgtype.Text{String: company, Valid: company != ""},
		JobTitle:     pgtype.Text{String: title, Valid: title != ""},
		TechStack:    stack,
		Requirements: requirements,
	}
}

func makeJobWithDescription(company, title, description string) *db.Job {
	return &db.Job{
		CompanyName: pgtype.Text{String: company, Valid: true},
		JobTitle:    pgtype.Text{String: title, Valid: true},
		Description: pgtype.Text{String: description, Valid: true},
		// Requirements vazio → buildUserPrompt deve usar Description
	}
}

func pgDate(year int, month time.Month) pgtype.Date {
	return pgtype.Date{Time: time.Date(year, month, 1, 0, 0, 0, 0, time.UTC), Valid: true}
}

func emptyMatch() *services.MatchResult {
	return &services.MatchResult{}
}

// parsePrompt deserializa o JSON do prompt para inspecionar campos
func parsePrompt(t *testing.T, raw string) userPrompt {
	t.Helper()
	var p userPrompt
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("parsePrompt: %v\njson: %s", err, raw)
	}
	return p
}

// newWorkerForTest cria um Worker mínimo para testar métodos puros.
// Campos de infra (DB, Redis, Logger) ficam nil — buildUserPrompt não os usa.
func newWorkerForTest() *Worker {
	return &Worker{}
}

// =============================================================================
// Campos da vaga
// =============================================================================

func TestBuildUserPrompt_DadosDaVagaNoJSON(t *testing.T) {
	w := newWorkerForTest()
	job := makeJob("Acme Corp", "Engenheiro Go", []string{"Go", "PostgreSQL"}, []string{"3 anos de experiência"})

	raw, err := w.buildUserPrompt(job, emptyMatch())
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}

	p := parsePrompt(t, raw)
	if p.Vaga.Empresa != "Acme Corp" {
		t.Errorf("Empresa = %q, want %q", p.Vaga.Empresa, "Acme Corp")
	}
	if p.Vaga.Titulo != "Engenheiro Go" {
		t.Errorf("Titulo = %q, want %q", p.Vaga.Titulo, "Engenheiro Go")
	}
	if len(p.Vaga.Stack) != 2 || p.Vaga.Stack[0] != "Go" {
		t.Errorf("Stack = %v, want [Go PostgreSQL]", p.Vaga.Stack)
	}
}

func TestBuildUserPrompt_UsaDescricaoQuandoRequisitoVazio(t *testing.T) {
	// Quando Requirements está vazio, a descrição bruta vai no campo Descricao da vaga.
	// Isso garante que o LLM tenha contexto mesmo quando o scraper não extraiu requisitos.
	w := newWorkerForTest()
	job := makeJobWithDescription("Empresa X", "Dev Backend", "Vaga de backend em Go com foco em microserviços")

	raw, err := w.buildUserPrompt(job, emptyMatch())
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}

	p := parsePrompt(t, raw)
	if p.Vaga.Descricao == "" {
		t.Error("Descricao deveria estar preenchida quando Requirements é vazio")
	}
	if len(p.Vaga.Requisitos) != 0 {
		t.Errorf("Requisitos deveria estar vazio, got %v", p.Vaga.Requisitos)
	}
}

func TestBuildUserPrompt_NaoUsaDescricaoQuandoRequisitosPresentes(t *testing.T) {
	// Quando Requirements está preenchido, Descricao NÃO vai no prompt
	// (evita duplicar contexto e inflacionar o token count)
	w := newWorkerForTest()
	job := makeJob("Empresa X", "Dev Backend", []string{"Go"}, []string{"3 anos"})
	job.Description = pgtype.Text{String: "descrição longa que não deveria ir", Valid: true}

	raw, err := w.buildUserPrompt(job, emptyMatch())
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}

	p := parsePrompt(t, raw)
	if p.Vaga.Descricao != "" {
		t.Errorf("Descricao deveria estar vazia quando Requirements está preenchido, got %q", p.Vaga.Descricao)
	}
}

// =============================================================================
// Limites de truncamento
// =============================================================================

func TestBuildUserPrompt_TruncaExperiencias(t *testing.T) {
	// maxExperiencias = 5: enviar mais de 5 ao LLM desperdicia tokens sem ganho.
	w := newWorkerForTest()

	exps := make([]db.UserExperience, 8)
	for i := range exps {
		exps[i] = db.UserExperience{CompanyName: "Empresa", JobRole: "Dev"}
	}

	raw, err := w.buildUserPrompt(makeJob("X", "Y", nil, nil), &services.MatchResult{Experiencias: exps})
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}

	p := parsePrompt(t, raw)
	if len(p.Experiencias) != 5 {
		t.Errorf("Experiencias: len = %d, want 5 (maxExperiencias)", len(p.Experiencias))
	}
}

func TestBuildUserPrompt_TruncaProjetos(t *testing.T) {
	// maxProjetos = 6
	w := newWorkerForTest()

	projs := make([]db.UserProject, 10)
	for i := range projs {
		projs[i] = db.UserProject{ProjectName: "Projeto"}
	}

	raw, err := w.buildUserPrompt(makeJob("X", "Y", nil, nil), &services.MatchResult{Projetos: projs})
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}

	p := parsePrompt(t, raw)
	if len(p.Projetos) != 6 {
		t.Errorf("Projetos: len = %d, want 6 (maxProjetos)", len(p.Projetos))
	}
}

func TestBuildUserPrompt_TruncaExcelentes(t *testing.T) {
	// maxExcelentes = 1: só o melhor exemplo vai no prompt para não inflar o contexto
	w := newWorkerForTest()

	excelentes := [][]byte{
		[]byte(`{"curriculo":"exemplo 1"}`),
		[]byte(`{"curriculo":"exemplo 2"}`),
		[]byte(`{"curriculo":"exemplo 3"}`),
	}

	raw, err := w.buildUserPrompt(makeJob("X", "Y", nil, nil), &services.MatchResult{Excelentes: excelentes})
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}

	p := parsePrompt(t, raw)
	if len(p.Feedback.ExemplosExcelentes) != 1 {
		t.Errorf("ExemplosExcelentes: len = %d, want 1 (maxExcelentes)", len(p.Feedback.ExemplosExcelentes))
	}
}

func TestBuildUserPrompt_TruncaConquistasNaExperiencia(t *testing.T) {
	// Cada experiência limita a 4 conquistas para não sobrecarregar o prompt
	w := newWorkerForTest()

	exp := db.UserExperience{
		CompanyName:  "Empresa",
		JobRole:      "Dev",
		Achievements: []string{"c1", "c2", "c3", "c4", "c5", "c6"},
	}

	raw, err := w.buildUserPrompt(makeJob("X", "Y", nil, nil), &services.MatchResult{Experiencias: []db.UserExperience{exp}})
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}

	p := parsePrompt(t, raw)
	if len(p.Experiencias[0].Conquistas) != 4 {
		t.Errorf("Conquistas: len = %d, want 4", len(p.Experiencias[0].Conquistas))
	}
}

func TestBuildUserPrompt_AbaixoDoLimiteNaoTrunca(t *testing.T) {
	// Verificação de que dados dentro dos limites passam integralmente
	w := newWorkerForTest()

	exps := make([]db.UserExperience, 3) // < maxExperiencias (5)
	for i := range exps {
		exps[i] = db.UserExperience{CompanyName: "Empresa", JobRole: "Dev"}
	}

	raw, err := w.buildUserPrompt(makeJob("X", "Y", nil, nil), &services.MatchResult{Experiencias: exps})
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}

	p := parsePrompt(t, raw)
	if len(p.Experiencias) != 3 {
		t.Errorf("Experiencias: len = %d, want 3 (sem truncamento)", len(p.Experiencias))
	}
}

// =============================================================================
// Formatação de datas
// =============================================================================

func TestBuildUserPrompt_FormatoDeDatas(t *testing.T) {
	// Datas devem aparecer como "YYYY-MM" — formato que o LLM entende facilmente.
	// Se mudar para "YYYY-M" ou outro formato, o teste pega imediatamente.
	w := newWorkerForTest()

	exp := db.UserExperience{
		CompanyName: "iex! Telecom",
		JobRole:     "Engenheiro Go",
		StartDate:   pgDate(2024, time.September),
		EndDate:     pgDate(2025, time.November),
	}

	raw, err := w.buildUserPrompt(makeJob("X", "Y", nil, nil), &services.MatchResult{Experiencias: []db.UserExperience{exp}})
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}

	p := parsePrompt(t, raw)
	e := p.Experiencias[0]
	if e.DataInicio != "2024-09" {
		t.Errorf("DataInicio = %q, want %q", e.DataInicio, "2024-09")
	}
	if e.DataFim != "2025-11" {
		t.Errorf("DataFim = %q, want %q", e.DataFim, "2025-11")
	}
}

func TestBuildUserPrompt_DataInvalidaNaoPopulaCampo(t *testing.T) {
	// pgtype.Date com Valid=false não deve gerar campo de data no prompt
	w := newWorkerForTest()

	exp := db.UserExperience{
		CompanyName: "Empresa",
		JobRole:     "Dev",
		IsCurrentJob: true,
		// StartDate e EndDate com Valid=false (zero value de pgtype.Date)
	}

	raw, err := w.buildUserPrompt(makeJob("X", "Y", nil, nil), &services.MatchResult{Experiencias: []db.UserExperience{exp}})
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}

	p := parsePrompt(t, raw)
	e := p.Experiencias[0]
	if e.DataInicio != "" {
		t.Errorf("DataInicio deveria ser vazio para data inválida, got %q", e.DataInicio)
	}
	if e.DataFim != "" {
		t.Errorf("DataFim deveria ser vazio para data inválida, got %q", e.DataFim)
	}
}

// =============================================================================
// Estrutura geral do JSON
// =============================================================================

func TestBuildUserPrompt_JSONValido(t *testing.T) {
	w := newWorkerForTest()
	raw, err := w.buildUserPrompt(makeJob("X", "Y", nil, nil), emptyMatch())
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}
	if !json.Valid([]byte(raw)) {
		t.Errorf("resultado não é JSON válido: %s", raw)
	}
}

func TestBuildUserPrompt_FeedbacksRepassados(t *testing.T) {
	// Feedbacks anteriores devem ir integralmente para o prompt
	w := newWorkerForTest()
	match := &services.MatchResult{
		Feedbacks: []string{"melhore a seção de experiência", "inclua mais métricas"},
	}

	raw, err := w.buildUserPrompt(makeJob("X", "Y", nil, nil), match)
	if err != nil {
		t.Fatalf("buildUserPrompt: %v", err)
	}

	if !strings.Contains(raw, "melhore a seção de experiência") {
		t.Error("feedback anterior não encontrado no prompt")
	}
}
