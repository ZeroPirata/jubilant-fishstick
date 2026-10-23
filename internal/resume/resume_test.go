package resume

import (
	"strings"
	"testing"
	"time"
)

func ym(s string) time.Time {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestYearsOfExperience(t *testing.T) {
	now := ym("2026-10")
	cases := []struct {
		label string
		exps  []SourceExperience
		want  int
	}{
		{"sem experiência", nil, 0},
		// Caso real do perfil de teste: 16 + 14 + 11 meses = 41 → 3 anos (a IA dizia 2).
		{"três cargos em sequência", []SourceExperience{
			{Start: ym("2022-11"), End: ym("2024-03")},
			{Start: ym("2024-09"), End: ym("2025-11")},
			{Start: ym("2025-11"), Current: true},
		}, 3},
		{"lacuna entre empregos não conta", []SourceExperience{
			{Start: ym("2020-01"), End: ym("2021-01")},
			{Start: ym("2024-01"), End: ym("2025-01")},
		}, 2},
		{"cargos simultâneos não contam em dobro", []SourceExperience{
			{Start: ym("2023-01"), End: ym("2025-01")},
			{Start: ym("2024-01"), End: ym("2024-06")},
		}, 2},
		{"menos de 1 ano", []SourceExperience{{Start: ym("2026-01"), Current: true}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := YearsOfExperience(tc.exps, now); got != tc.want {
				t.Errorf("YearsOfExperience = %d, want %d", got, tc.want)
			}
		})
	}
}

func llmOut() LLMOutput {
	var out LLMOutput
	out.Resumo = "Engenheiro Backend com 3 anos em Go."
	out.Experiencias = append(out.Experiencias, struct {
		ID      int      `json:"id"`
		Bullets []string `json:"bullets"`
	}{0, []string{
		"Desenvolveu microsserviços em Go com Fiber para motores de cálculo.",
		"Otimizou a latência em 30% com Redis.", // número inventado
		"Implementou validação CRC-16 nos pacotes UDP.",
	}})
	out.Habilidades = append(out.Habilidades, struct {
		Categoria string   `json:"categoria"`
		Itens     []string `json:"itens"`
	}{"Linguagens", []string{"Go", "Java", "Python", "go"}})
	out.Projetos = append(out.Projetos,
		struct {
			ID        int    `json:"id"`
			Descricao string `json:"descricao"`
		}{1, "Gerador de currículos com Go."},
		struct {
			ID        int    `json:"id"`
			Descricao string `json:"descricao"`
		}{99, "projeto que não existe"},
	)
	return out
}

func testSource() Source {
	return Source{
		Lang:   "pt-br",
		Header: Header{Name: "Fulano", Email: "f@x.com"},
		Experiences: []SourceExperience{{
			Company: "liax", Role: "Dev Pleno", Start: ym("2025-11"), Current: true,
			Stack:        []string{"Golang", "Redis", "Fiber"},
			Achievements: []string{"Desenvolvi microsserviços com Fiber", "validação com CRC-16"},
		}},
		Projects: []SourceProject{
			{Name: "Portfolio", Description: "site"},
			{Name: "JobPilot", Description: "gera currículos", URL: "https://github.com/x/jobpilot"},
		},
		Skills:  []string{"Python"},
		Aliases: map[string]string{"golang": "go"},
	}
}

func TestAssemble_FatosVemDoBanco(t *testing.T) {
	doc, _ := Assemble(testSource(), llmOut())

	e := doc.Experiences[0]
	if e.Company != "liax" || e.Role != "Dev Pleno" || e.Start != "2025-11" || !e.Current || e.End != "" {
		t.Errorf("experiência = %+v, want dados do banco", e)
	}
	if doc.Header.Name != "Fulano" {
		t.Errorf("header = %+v", doc.Header)
	}
}

func TestAssemble_RemoveMetricaInventada(t *testing.T) {
	doc, warnings := Assemble(testSource(), llmOut())

	bullets := doc.Experiences[0].Bullets
	if len(bullets) != 2 {
		t.Fatalf("bullets = %v, want 2 (o de 30%% removido)", bullets)
	}
	for _, b := range bullets {
		if strings.Contains(b, "30%") {
			t.Errorf("bullet com métrica inventada não foi removido: %q", b)
		}
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "30%") {
		t.Errorf("warnings deveriam citar o bullet removido: %v", warnings)
	}
}

func TestAssemble_SemBulletsUsaConquistas(t *testing.T) {
	out := llmOut()
	out.Experiencias = nil
	doc, _ := Assemble(testSource(), out)

	if got := doc.Experiences[0].Bullets; len(got) != 2 || got[0] != "Desenvolvi microsserviços com Fiber" {
		t.Errorf("bullets = %v, want as conquistas cadastradas", got)
	}
}

func TestAssemble_HabilidadesSoDoPerfil(t *testing.T) {
	doc, warnings := Assemble(testSource(), llmOut())

	// "Go" vale por causa do alias golang→go na stack; "Java" não existe no perfil;
	// "go" repetido é deduplicado.
	got := doc.Skills[0].Items
	if strings.Join(got, ",") != "Go,Python" {
		t.Errorf("habilidades = %v, want [Go Python]", got)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "Java") {
		t.Errorf("warnings deveriam citar Java: %v", warnings)
	}
}

// Tecnologia citada só na descrição de um projeto também vale (perfil de primeiro emprego).
// Mas "Go" não pode casar dentro de "Google".
func TestAssemble_HabilidadeCitadaEmProjeto(t *testing.T) {
	src := testSource()
	src.Experiences = nil
	src.Skills = nil
	src.Projects = []SourceProject{{Name: "Bot", Description: "Bot em TypeScript com C# e integração Google Sheets"}}
	out := llmOut()
	out.Habilidades[0].Itens = []string{"TypeScript", "C#", "Go"}

	doc, _ := Assemble(src, out)
	if got := strings.Join(doc.Skills[0].Items, ","); got != "TypeScript,C#" {
		t.Errorf("habilidades = %v, want [TypeScript C#]", got)
	}
}

func TestAssemble_HabilidadeCitadaNasConquistas(t *testing.T) {
	src := testSource()
	src.Experiences[0].Achievements = []string{"Projetei o sistema em Hexagonal Architecture", "Filas com AWS SQS"}
	out := llmOut()
	out.Habilidades[0].Itens = []string{"Hexagonal Architecture", "AWS", "Kubernetes"}

	doc, _ := Assemble(src, out)
	if got := strings.Join(doc.Skills[0].Items, ","); got != "Hexagonal Architecture,AWS" {
		t.Errorf("habilidades = %v, want [Hexagonal Architecture AWS]", got)
	}
}

func TestAssemble_ProjetosPorID(t *testing.T) {
	doc, _ := Assemble(testSource(), llmOut())

	if len(doc.Projects) != 1 {
		t.Fatalf("projetos = %+v, want só o id 1 (99 não existe)", doc.Projects)
	}
	p := doc.Projects[0]
	if p.Name != "JobPilot" || p.URL != "https://github.com/x/jobpilot" || p.Description != "Gerador de currículos com Go." {
		t.Errorf("projeto = %+v", p)
	}
}

func TestText(t *testing.T) {
	doc, _ := Assemble(testSource(), llmOut())
	txt := Text(doc)

	for _, want := range []string{"Fulano", "f@x.com", "RESUMO PROFISSIONAL", "liax — Dev Pleno", "Nov/2025 - Presente", "- Desenvolveu microsserviços", "Linguagens: Go, Python", "JobPilot"} {
		if !strings.Contains(txt, want) {
			t.Errorf("texto não contém %q:\n%s", want, txt)
		}
	}
}

func TestFormatMonth(t *testing.T) {
	cases := map[string]string{"2025-11": "Nov/2025", "2024-02": "Fev/2024", "": "", "2024-13": "", "lixo": ""}
	for in, want := range cases {
		if got := FormatMonth(in, "pt-br"); got != want {
			t.Errorf("FormatMonth(%q) = %q, want %q", in, got, want)
		}
	}
	if got := FormatMonth("2024-02", "en"); got != "Feb/2024" {
		t.Errorf("FormatMonth en = %q, want Feb/2024", got)
	}
}

func TestSummaryViolations(t *testing.T) {
	now := ym("2026-10")
	junior := Source{Lang: "en"} // sem experiência e sem formação
	cases := []struct {
		label   string
		src     Source
		summary string
		want    int
	}{
		// Caso real do lab: a IA escreveu as duas coisas.
		{"0 anos, experiência e formação inventada", junior, "Backend Engineer with 0 years of experience and a degree in Computer Science. Built X.", 3},
		{"resumo limpo", junior, "Developer with hands-on projects in Go. Built an AI resume generator.", 0},
		{"formação existe", Source{Lang: "en", Education: []Education{{Course: "ADS"}}}, "Developer with a degree in ADS.", 0},
		{"com experiência pode citar anos", Source{Experiences: []SourceExperience{{Start: ym("2022-01"), Current: true}}, Education: []Education{{}}}, "Engenheiro com 4 anos em Go.", 0},
		{"pt: formação inventada", Source{Lang: "pt-br"}, "Desenvolvedor com formação em Ciência da Computação.", 1},
		// Caso real do lab: objetivo certo na linha 1, mas "possui experiência" na linha 2.
		{"pt: possui experiência sem experiência", Source{Lang: "pt-br", Education: []Education{{}}},
			"Desenvolvedor em busca da primeira oportunidade como Backend Engineer. Possui experiência em sistemas distribuídos.", 1},
		{"pt: objetivo limpo", Source{Lang: "pt-br", Education: []Education{{}}},
			"Desenvolvedor em busca da primeira oportunidade como Backend Engineer, com projetos práticos em Go. Construí um gerador de currículos.", 0},
		{"en: experience in", Source{Lang: "en", Education: []Education{{}}}, "Developer seeking a first opportunity. Has experience in Go.", 1},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := SummaryViolations(tc.summary, tc.src, now); len(got) != tc.want {
				t.Errorf("violações = %v, want %d", got, tc.want)
			}
		})
	}
}

func TestStripViolatingSentences(t *testing.T) {
	got := StripViolatingSentences("Backend Engineer with 0 years of experience. Built an AI resume generator in Go. Holds a degree in CS.", Source{Lang: "en"}, ym("2026-10"))
	if got != "Built an AI resume generator in Go." {
		t.Errorf("got %q", got)
	}
}

// Caso real do lab: sem experiência, a IA escreveu "extensive experience" e citou
// AWS/Azure (stack da vaga, ausentes do perfil).
func TestSummaryViolations_TecnologiaDaVagaEExperienciaInflada(t *testing.T) {
	src := Source{
		Lang:     "en",
		Skills:   []string{"Go", "TypeScript"},
		JobStack: []string{"go", "aws", "azure", "kubernetes"},
		Aliases:  map[string]string{"golang": "go"},
	}
	summary := "Backend Engineer with extensive experience in microservices. Proficient in Golang and TypeScript. Skilled in AWS, Azure and GCP."

	got := SummaryViolations(summary, src, ym("2026-10"))
	joined := strings.Join(got, " | ")
	if len(got) != 2 || !strings.Contains(joined, "extensive experience") || !strings.Contains(joined, "aws, azure") {
		t.Fatalf("violações = %v", got)
	}
	if strings.Contains(joined, "kubernetes") {
		t.Errorf("kubernetes não está no resumo, não deveria ser citado: %v", got)
	}

	stripped := StripViolatingSentences(summary, src, ym("2026-10"))
	if stripped != "Proficient in Golang and TypeScript." {
		t.Errorf("stripped = %q", stripped)
	}
}

// Caso real do benchmark (llama-3.2-11b): bullets com fatos de outra empresa.
func TestAssemble_TecnologiaDeOutraExperiencia(t *testing.T) {
	src := Source{
		Lang:    "pt-br",
		Aliases: map[string]string{"golang": "go", "google cloud platform": "gcp"},
		Experiences: []SourceExperience{
			{Company: "liax", Start: ym("2025-11"), Current: true,
				Stack:        []string{"Golang", "Fiber API", "AWS SQS", "Redis"},
				Achievements: []string{"Desenvolvi microsserviços em Go com Fiber", "mensageria com AWS SQS"}},
			{Company: "iez! Telecom", Start: ym("2024-09"), End: ym("2025-11"),
				Stack:        []string{"Golang", "gRPC", "Protobuf", "Envoy", "Google Cloud Platform"},
				Achievements: []string{"sistema de assinaturas em Hexagonal Architecture", "dados de rede L7 com gRPC"}},
			{Company: "tecsus", Start: ym("2022-11"), End: ym("2024-03"),
				Stack:        []string{"Python", "UDP", "SQS"},
				Achievements: []string{"pacotes via protocolo UDP com validação CRC-16"}},
		},
	}
	var out LLMOutput
	add := func(id int, bullets ...string) {
		out.Experiencias = append(out.Experiencias, struct {
			ID      int      `json:"id"`
			Bullets []string `json:"bullets"`
		}{id, bullets})
	}
	add(0,
		"Desenvolveu microsserviços em Go com Fiber para motores de cálculo.",
		"Desenvolveu sistemas com gRPC e Protobuf e service proxy via Envoy.", // é da iez
		"Implantou em GCP com alta disponibilidade.",                          // alias de Google Cloud Platform, da iez
	)
	add(1,
		"Projetou o sistema de assinaturas em Hexagonal Architecture com CronJobs em Go.", // Go está nas duas: ok
		"Desenvolveu serviço de pacotes via UDP para parsing de protocolos.",              // é da tecsus
	)

	doc, warnings := Assemble(src, out)

	if got := doc.Experiences[0].Bullets; len(got) != 1 || !strings.Contains(got[0], "Fiber") {
		t.Errorf("liax: bullets = %v, want só o do Fiber", got)
	}
	if got := doc.Experiences[1].Bullets; len(got) != 1 || !strings.Contains(got[0], "Hexagonal") {
		t.Errorf("iez: bullets = %v, want só o da Hexagonal Architecture", got)
	}
	joined := strings.Join(warnings, "\n")
	for _, want := range []string{"citava gRPC", "citava gcp", "citava UDP"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings deveriam conter %q:\n%s", want, joined)
		}
	}
}

// "Go" com caixa exata: a palavra portuguesa "go" não pode ser confundida com a linguagem,
// e "Go" não pode casar dentro de "Google".
func TestMentions(t *testing.T) {
	cases := []struct {
		text, term string
		want       bool
	}{
		{"microsserviços em Go com Fiber", "Go", true},
		{"integração com Google Sheets", "Go", false},
		{"o go-live foi em março", "Go", false},
		{"filas com AWS SQS", "aws sqs", true},
		{"API em C# e C++", "C#", true},
		{"Kubernetes", "kube", false},
	}
	for _, tc := range cases {
		if got := mentions(tc.text, tc.term); got != tc.want {
			t.Errorf("mentions(%q, %q) = %v, want %v", tc.text, tc.term, got, tc.want)
		}
	}
}
