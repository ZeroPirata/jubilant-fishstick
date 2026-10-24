package profileimport

import (
	"context"
	"strings"
	"testing"
	"time"
)

var (
	refNow      = time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	testAliases = map[string]string{"golang": "go", "google cloud platform": "gcp", "next.js": "nextjs"}
)

func TestCanonKey(t *testing.T) {
	same := [][2]string{
		{"Go", "Golang"}, {"Go (Golang)", "go"}, {"Google Cloud Platform (GCP)", "GCP"},
		{"Next.js", "NextJS"}, {"HTML5", "html"}, {"CSS3", "CSS"},
	}
	for _, p := range same {
		if CanonKey(p[0], testAliases) != CanonKey(p[1], testAliases) {
			t.Errorf("%q e %q deveriam ser a mesma habilidade", p[0], p[1])
		}
	}
	if CanonKey("Java", testAliases) == CanonKey("JavaScript", testAliases) {
		t.Error("Java não é JavaScript")
	}
}

func TestNormMonth(t *testing.T) {
	cases := map[string]string{
		"2024-03":    "2024-03",
		"2024-3":     "2024-03",
		"2024-03-15": "2024-03",
		"03/2024":    "2024-03",
		"2021":       "2021-01",
		"Present":    "",
		"atual":      "",
		"2024-13":    "",
		"":           "",
	}
	for in, want := range cases {
		if got := NormMonth(in); got != want {
			t.Errorf("NormMonth(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSkillKey(t *testing.T) {
	if SkillKey("Node.js") != SkillKey("nodejs") || SkillKey("Next JS") != SkillKey("next.js") {
		t.Error("variações de grafia deveriam ser a mesma habilidade")
	}
	if SkillKey("C++") == SkillKey("C") || SkillKey("C#") == SkillKey("C") {
		t.Error("C++ e C# não são C")
	}
}

func TestNormalize_NivelPorTempoSemDuplicarPromocoes(t *testing.T) {
	d := Draft{
		Experiences: []Experience{
			// Promoção na mesma empresa: períodos se sobrepõem, não podem somar duas vezes.
			{CompanyName: "Acme", JobRole: "Dev II", StartDate: "2025-01", IsCurrentJob: true, TechStack: []string{"Go"}},
			{CompanyName: "Acme", JobRole: "Dev I", StartDate: "2024-01", EndDate: "2025-03", TechStack: []string{"Go"}},
			{CompanyName: "Beta", JobRole: "Estágio", StartDate: "2020-01", EndDate: "2020-12", Description: "APIs em Python e um pouco de go para scripts"},
		},
		Skills: []Skill{
			{SkillName: "Go"},
			{SkillName: "Python"},
			{SkillName: "React", ProficiencyLevel: "expert", Explicit: true},
			{SkillName: "Docker", ProficiencyLevel: "advanced"}, // nível sem explicit: ignorado
			{SkillName: "Golang"}, // sinônimo: duplicada
		},
	}
	d.Normalize(refNow, testAliases)

	got := map[string]Skill{}
	for _, s := range d.Skills {
		got[s.SkillName] = s
	}
	if len(d.Skills) != 4 {
		t.Fatalf("esperava 4 habilidades sem duplicata, veio %d: %+v", len(d.Skills), d.Skills)
	}
	// Go: jan/2024 até out/2026 = 34 meses (a sobreposição não conta duas vezes).
	if g := got["Go"]; g.Months != 34 || g.ProficiencyLevel != "intermediate" || len(g.UsedIn) != 1 {
		t.Errorf("Go = %+v", g)
	}
	// "go" minúsculo no texto não é a linguagem Go; Python está escrito na descrição.
	if p := got["Python"]; p.Months != 12 || p.ProficiencyLevel != "intermediate" {
		t.Errorf("Python = %+v", p)
	}
	if r := got["React"]; r.ProficiencyLevel != "expert" || !r.Explicit {
		t.Errorf("nível explícito deveria ser mantido: %+v", r)
	}
	if dk := got["Docker"]; dk.ProficiencyLevel != "basic" || dk.Explicit {
		t.Errorf("sem uso nem nível escrito deveria ser basic sugerido: %+v", dk)
	}
	if d.Skills[0].SkillName != "Go" {
		t.Errorf("mais usada deveria vir primeiro, veio %q", d.Skills[0].SkillName)
	}
}

func TestNormalize_LimpaItensEURLs(t *testing.T) {
	d := Draft{
		Source: "qualquer",
		Links:  Links{LinkedinURL: "www.linkedin.com/in/fulano", GithubURL: "javascript:alert(1)", PortfolioURL: "meu site"},
		Experiences: []Experience{
			{CompanyName: "", JobRole: "Dev"},
			{CompanyName: "Acme", JobRole: "Dev", StartDate: "2023", EndDate: "2024-02", IsCurrentJob: true,
				Achievements: []string{"● Fez X", "Fez X", "  "}},
		},
	}
	d.Normalize(refNow, testAliases)

	if d.Source != "curriculo" {
		t.Errorf("source = %q", d.Source)
	}
	if d.Links.LinkedinURL != "https://www.linkedin.com/in/fulano" || d.Links.GithubURL != "" || d.Links.PortfolioURL != "" {
		t.Errorf("links = %+v", d.Links)
	}
	if len(d.Experiences) != 1 {
		t.Fatalf("experiência sem empresa deveria sair: %+v", d.Experiences)
	}
	e := d.Experiences[0]
	if e.StartDate != "2023-01" || e.IsCurrentJob || len(e.Achievements) != 1 || e.Achievements[0] != "Fez X" {
		t.Errorf("experiência = %+v", e)
	}
}

func TestApplyRequest_Validate(t *testing.T) {
	ok := ApplyRequest{
		Experiences: []Experience{{CompanyName: "Acme", JobRole: "Dev", StartDate: "2024-01"}},
		Skills:      []Skill{{SkillName: "Go", ProficiencyLevel: "advanced"}},
	}
	if err := ok.Validate(); err != nil {
		t.Fatalf("válido deu erro: %v", err)
	}

	bad := []ApplyRequest{
		{Experiences: []Experience{{CompanyName: "Acme", JobRole: "Dev"}}},
		{Experiences: []Experience{{CompanyName: "Acme", JobRole: "Dev", StartDate: "2024-05", EndDate: "2024-01"}}},
		{Skills: []Skill{{SkillName: "Go", ProficiencyLevel: "mestre"}}},
		{Projects: []Project{{ProjectName: "X"}}},
		{Certificates: []Certificate{{IssuingOrganization: "AWS"}}},
	}
	for i, r := range bad {
		if err := r.Validate(); err == nil {
			t.Errorf("caso %d deveria falhar", i)
		}
	}
}

func TestExtractText(t *testing.T) {
	txt := strings.Repeat("Desenvolvedor Go com experiência em APIs. ", 10) + "​\nPage 1 of 3\n\n\n\nFim"
	got, err := ExtractText(context.Background(), []byte(txt))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "​") || strings.Contains(got, "Page 1 of 3") || strings.Contains(got, "\n\n\n") {
		t.Errorf("texto não foi limpo: %q", got)
	}

	if _, err := ExtractText(context.Background(), []byte("curto")); err != ErrNoText {
		t.Errorf("texto curto: err = %v", err)
	}
	if _, err := ExtractText(context.Background(), []byte{0x89, 'P', 'N', 'G', 0, 0}); err != ErrUnsupported {
		t.Errorf("imagem: err = %v", err)
	}
}

func TestDecode_RejeitaJSONVazio(t *testing.T) {
	if _, err := Decode(`{"profile":{},"experiences":[]}`); err == nil {
		t.Error("JSON sem dados deveria falhar para o fallback tentar outro modelo")
	}
	if _, err := Decode(`{"profile":{"full_name":"Fulano"}}`); err != nil {
		t.Errorf("JSON com nome deveria passar: %v", err)
	}
}

func TestNormalize_IdiomasCodigoNivelEDuplicados(t *testing.T) {
	d := Draft{Profile: Profile{FullName: "X"}, Languages: []Language{
		{Language: "Inglês", Proficiency: "advanced"},
		{Language: "English", Proficiency: "fluent"}, // mesmo idioma: fica o primeiro
		{Language: "Espanhol", Proficiency: "conversação"},
		{Language: " ", Proficiency: "basic"},
	}}
	d.Normalize(time.Now(), nil)
	if len(d.Languages) != 2 {
		t.Fatalf("idiomas = %+v", d.Languages)
	}
	if d.Languages[0] != (Language{Language: "en", Proficiency: "advanced"}) {
		t.Errorf("0 = %+v", d.Languages[0])
	}
	if d.Languages[1] != (Language{Language: "es", Proficiency: "intermediate"}) {
		t.Errorf("nível desconhecido vira intermediate: %+v", d.Languages[1])
	}
}

func TestLanguagesFromText_LinhaDentroDeHabilidades(t *testing.T) {
	text := "HABILIDADES\nFrameworks: React, Next.js\nIdiomas: Português (Nativo), Inglês (C1 - Avançado), Japonês (Básico).\nInglês é usado no dia a dia." +
		"\nSpanish – B1\nFrench: fluent"
	got := LanguagesFromText(text)
	want := []Language{{"pt", "native", ""}, {"en", "advanced", ""}, {"ja", "basic", ""}, {"es", "intermediate", ""}, {"fr", "fluent", ""}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestMergeLanguages_SoAcrescentaOQueFaltou(t *testing.T) {
	d := Draft{Languages: []Language{{Language: "en", Proficiency: "fluent"}}}
	d.MergeLanguages([]Language{{Language: "en", Proficiency: "basic"}, {Language: "ja", Proficiency: "basic"}})
	if len(d.Languages) != 2 || d.Languages[0].Proficiency != "fluent" || d.Languages[1].Language != "ja" {
		t.Errorf("idiomas = %+v", d.Languages)
	}
}

func TestNormalize_IdiomaNaoEntraComoHabilidade(t *testing.T) {
	d := Draft{Profile: Profile{FullName: "X"}, Skills: []Skill{{SkillName: "Inglês"}, {SkillName: "Go"}, {SkillName: "English"}}}
	d.Normalize(time.Now(), nil)
	if len(d.Skills) != 1 || d.Skills[0].SkillName != "Go" {
		t.Errorf("skills = %+v", d.Skills)
	}
}
