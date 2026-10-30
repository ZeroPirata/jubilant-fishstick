package resume

import (
	"strings"
	"testing"
)

func TestRoleTranslationOK(t *testing.T) {
	cases := []struct {
		orig, tr string
		want     bool
	}{
		{"Desenvolvedor Pleno II", "Mid-level Software Developer II", true},
		{"Desenvolvedor Pleno II", "Software Developer II", true},
		{"Desenvolvedor Pleno II", "Software Developer", true}, // perder o nível intermediário não promove
		{"Desenvolvedor Backend", "Backend Developer", true},
		{"Estagiário de Desenvolvimento", "Software Development Intern", true},
		{"Desenvolvedor Pleno II", "Senior Software Developer", false},
		{"Desenvolvedor Pleno II", "Software Developer III", false},
		{"Desenvolvedor Backend", "Backend Tech Lead", false},
		{"Desenvolvedor Backend", "Backend Engineering Manager", false},
		{"Arquiteto de Software", "Software Architect", true},
		{"Desenvolvedor Backend", "Software Architect", false},
		{"Estagiário de Desenvolvimento", "Software Developer", false}, // esconde o estágio
		{"Desenvolvedor Júnior", "Junior Developer", true},
		{"Desenvolvedor Júnior", "Developer", false},
		{"Tech Lead", "Tech Lead", true},
		{"Desenvolvedor", "", false},
	}
	for _, c := range cases {
		if got := roleTranslationOK(c.orig, c.tr); got != c.want {
			t.Errorf("roleTranslationOK(%q, %q) = %v, want %v", c.orig, c.tr, got, c.want)
		}
	}
}

func TestSkillTranslationOK(t *testing.T) {
	cases := []struct {
		orig, tr string
		want     bool
	}{
		{"Design de Sistemas", "System Design", true},
		{"Arquitetura Hexagonal", "Hexagonal Architecture", true},
		{"Otimização de Performance (BD)", "Database Performance Optimization", true},
		{"Otimização de Performance (BD)", "Performance Optimization (DB)", true},
		{"UDP / CRC-16", "UDP / CRC-16", true},
		{"Concorrência / Goroutines", "Concurrency / Goroutines", true},
		{"Mensageria Assíncrona", "Asynchronous Messaging (Kafka)", false},
		{"Mensageria Assíncrona", "Asynchronous Messaging with AWS SQS", false},
		{"Padrões de Projeto", "Design Patterns", true},
		{"Monitoramento e Observabilidade", "Monitoring and Observability", true},
		{"Banco de Dados", "Databases, SQL", false},
		{"HTTP", "HTTP/2", false},
	}
	for _, c := range cases {
		if got := skillTranslationOK(c.orig, c.tr); got != c.want {
			t.Errorf("skillTranslationOK(%q, %q) = %v, want %v", c.orig, c.tr, got, c.want)
		}
	}
}

func enSource() Source {
	src := testSource()
	src.Lang = "en"
	src.Experiences[0].Role = "Desenvolvedor Pleno II"
	src.Skills = append(src.Skills, "Design de Sistemas", "Arquitetura Hexagonal")
	return src
}

func TestAssemble_TraduzCargoESkillsNoIngles(t *testing.T) {
	out := llmOut()
	out.Experiencias[0].Cargo = "Mid-level Software Developer II"
	out.Habilidades = []LLMSkillGroup{{"Concepts & Architecture", []string{"Design de Sistemas", "Hexagonal Architecture", "Python"}}}
	out.Traducoes = map[string]string{"Design de Sistemas": "System Design", "Arquitetura Hexagonal": "Hexagonal Architecture"}

	doc, warnings := Assemble(enSource(), out)

	if got := doc.Experiences[0].Role; got != "Mid-level Software Developer II" {
		t.Errorf("cargo = %q, want a tradução", got)
	}
	got := strings.Join(doc.Skills[0].Items, ", ")
	if got != "System Design, Hexagonal Architecture, Python" {
		t.Errorf("skills = %q (avisos: %v)", got, warnings)
	}
}

func TestAssemble_RecusaPromocaoNaTraducao(t *testing.T) {
	out := llmOut()
	out.Experiencias[0].Cargo = "Senior Software Engineer"
	out.Habilidades = []LLMSkillGroup{{"Concepts", []string{"Design de Sistemas"}}}
	out.Traducoes = map[string]string{"Design de Sistemas": "Distributed System Design with Kafka (AWS MSK)"}

	doc, warnings := Assemble(enSource(), out)

	if got := doc.Experiences[0].Role; got != "Desenvolvedor Pleno II" {
		t.Errorf("cargo = %q, want o original", got)
	}
	if got := doc.Skills[0].Items; len(got) != 1 || got[0] != "Design de Sistemas" {
		t.Errorf("skills = %v, want o nome original", got)
	}
	joined := strings.Join(warnings, "\n")
	for _, want := range []string{"tradução de cargo recusada", "tradução de habilidade recusada"} {
		if !strings.Contains(joined, want) {
			t.Errorf("avisos = %v, want %q", warnings, want)
		}
	}
}

func TestAssemble_PortuguesIgnoraTraducao(t *testing.T) {
	out := llmOut()
	out.Experiencias[0].Cargo = "Mid-level Software Developer"
	out.Traducoes = map[string]string{"Python": "Pythonzinho"}

	doc, _ := Assemble(testSource(), out)

	if got := doc.Experiences[0].Role; got != "Dev Pleno" {
		t.Errorf("cargo = %q, want o do perfil no currículo em português", got)
	}
}
