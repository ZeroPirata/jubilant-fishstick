package discovery

import (
	"slices"
	"testing"
)

func TestMatchTechs(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		{"Vaga para dev Golang com Kubernetes (k8s)", []string{"Go", "Kubernetes"}},
		{"Experiência com Go e gRPC", []string{"Go"}},
		{"Apoiar o go-live e a estratégia go-to-market", nil},
		{"JavaScript e Node.js", []string{"JavaScript", "Node.js"}},
		{"Java 17 e Spring Boot", []string{"Java"}},
		{"Usamos MySQL", nil}, // "sql" não bate dentro de "mysql"
		{"C# e .NET 8", []string{"C#"}},
		// CRM de vendas, não a linguagem.
		{"Experience in Zoho and Go High Level preferred.", nil},
		{"Maintain accurate records in Go High Level.", nil},
		{"Tools: GoHighLevel, HubSpot", nil},
		{"Backend em Go, HubSpot no comercial", []string{"Go"}},
	}
	names := []string{"Go", "Kubernetes", "JavaScript", "Node.js", "Java", "SQL", "C#"}
	for _, tc := range cases {
		if got := MatchTechs(tc.text, names); !slices.Equal(got, tc.want) {
			t.Errorf("MatchTechs(%q) = %v, quer %v", tc.text, got, tc.want)
		}
	}
}

func TestTechnologiesCatalog(t *testing.T) {
	seen := map[string]bool{}
	for _, tc := range Technologies {
		if seen[tc.Name] {
			t.Errorf("tecnologia repetida: %s", tc.Name)
		}
		seen[tc.Name] = true
		if tc.Search == "" || len(tc.Terms) == 0 || tc.Group == "" {
			t.Errorf("tecnologia incompleta: %+v", tc)
		}
		// Cada termo precisa bater no próprio texto (pega termo com maiúscula ou espaço sobrando).
		if got := MatchTechs(tc.Search, []string{tc.Name}); len(got) != 1 {
			t.Errorf("%s: termo de busca %q não casa com a própria tecnologia", tc.Name, tc.Search)
		}
	}
	if got := TechSearchTerms([]string{"Go", "inexistente"}); !slices.Equal(got, []string{"golang"}) {
		t.Errorf("TechSearchTerms: %v", got)
	}
}

// Vagas reais que casavam com Go sem ter nada de programação em Go.
func TestMatchTechs_GoAmbiguo(t *testing.T) {
	notGo := []string{
		"By 60 days: is the reliable go-to for release readiness on their areas",
		"Client Accelerators is the ‘go to’ company for coaches, consultants, experts",
		"Proactive Problem Solving: Go beyond fulfilling requirements",
		"Analista Logística PL - Anápolis/Go",
		"Analista Logística PL - Anápolis /Go",
		"ATENDENTE RESTAURANTE (JARDIM GOIAS - GOIANIA/GO)",
		"Supervisor(a) | Presencial em Jataí- GO",
		"Account Executive - Go High Level (GHL)",
		"We strongly encourage all aspiring applicants to go for it",
		"apoiar o go-live de novos produtos e os próximos go-lives",
		"Own go-to-market strategy across our core products",
		"pipeline depth as you go. You will work closely",
	}
	for _, text := range notGo {
		if got := MatchTechs(text, []string{"Go"}); len(got) > 0 {
			t.Errorf("não é Go: %q", text)
		}
	}
	isGo := []string{
		"Bash, PowerShell, Python e/ou Go;",
		"linguagens como Python, Go, Java, JavaScript",
		"Stack: Go, Next.js e AWS.",
		"Knowledge of the Go programming language",
		"Desejáveis: Desenvolvimento em Go. Spark e processamento distribuído.",
		"Pessoa Desenvolvedora Fullstack Go/Node.js - Pleno",
		"Backend Engineer LATAM Python/Go AWS",
		"Usamos Go.",
		"Desenvolvedor Go Sênior",
		"Tags: go, docker",
		"Experiência com golang",
	}
	for _, text := range isGo {
		if got := MatchTechs(text, []string{"Go"}); len(got) == 0 {
			t.Errorf("é Go: %q", text)
		}
	}
	// Mesma regra para skills do perfil.
	if got := MatchSkills("the reliable go-to for release readiness", []Skill{NewSkill("Go")}); len(got) > 0 {
		t.Errorf("skill Go casou com go-to: %v", got)
	}
}

func TestMatchTechs_OutrosAmbiguos(t *testing.T) {
	cases := map[string]bool{
		"Be able to react quickly to customer needs":     false,
		"Front-end com React e TypeScript":               true,
		"Experience with SWIFT payment messages":         true, // ambíguo de verdade: aceito
		"Hiring for our spring 2027 cohort":              false,
		"Java, Spring Boot e Kafka":                      true,
		"SOFTWARE DEVELOPER ANALYST III (React) - FRAUD": true,
		"- Kubernetes- Node JS- ReactJs":                 true,
	}
	for text, want := range cases {
		got := len(MatchTechs(text, []string{"React", "Swift", "Spring", "Node.js"})) > 0
		if got != want {
			t.Errorf("%q: %v, esperado %v", text, got, want)
		}
	}
}
