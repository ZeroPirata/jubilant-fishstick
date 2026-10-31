package jobfit

import (
	"reflect"
	"testing"

	"hackton-treino/internal/db"
)

func TestKeywords_TresListas(t *testing.T) {
	skills := []db.UserSkill{
		{SkillName: "Go (Golang)", ProficiencyLevel: db.SkillLevelAdvanced},
		{SkillName: "PostgreSQL"},
		{SkillName: "Docker", ProficiencyLevel: db.SkillLevelBasic},
		{SkillName: "Kubernetes"},
	}
	aliases := map[string]string{"k8s": "kubernetes"}
	stack := []string{"Go", "PostgreSQL", "Docker", "Kafka", "Kubernetes"}
	resumeText := "Backend em Golang com PostgreSQL. Deploy em k8s."

	got := Keywords(stack, "", resumeText, skills, aliases)
	if !got.HasResume {
		t.Fatal("HasResume = false com currículo")
	}
	if want := []string{"Go", "PostgreSQL", "Kubernetes"}; !reflect.DeepEqual(got.InResume, want) {
		t.Errorf("InResume = %v, want %v", got.InResume, want)
	}
	if want := []KeywordGap{{Name: "Docker", Skill: "Docker"}}; !reflect.DeepEqual(got.MissingInResume, want) {
		t.Errorf("MissingInResume = %v, want %v", got.MissingInResume, want)
	}
	if want := []string{"Kafka"}; !reflect.DeepEqual(got.NotInProfile, want) {
		t.Errorf("NotInProfile = %v, want %v", got.NotInProfile, want)
	}
}

func TestKeywords_SemCurriculo(t *testing.T) {
	got := Keywords([]string{"Go", "Rust"}, "", "  ", []db.UserSkill{{SkillName: "Go"}}, nil)
	if got.HasResume || len(got.InResume) != 0 {
		t.Fatalf("sem currículo: %+v", got)
	}
	if len(got.MissingInResume) != 1 || got.MissingInResume[0].Name != "Go" {
		t.Errorf("MissingInResume = %v", got.MissingInResume)
	}
	if !reflect.DeepEqual(got.NotInProfile, []string{"Rust"}) {
		t.Errorf("NotInProfile = %v", got.NotInProfile)
	}
}

func TestKeywords_DescricaoSemRepetir(t *testing.T) {
	// "Golang" e "Python" aparecem na descrição: Go já está na stack, Python entra.
	desc := "Buscamos pessoa desenvolvedora com Golang e Python para nosso time."
	got := Keywords([]string{"Go"}, desc, "", nil, nil)
	if want := []string{"Go", "Python"}; !reflect.DeepEqual(got.NotInProfile, want) {
		t.Errorf("termos = %v, want %v", got.NotInProfile, want)
	}
}

func TestKeywords_PalavraInteira(t *testing.T) {
	// "Java" não pode casar dentro de "JavaScript"; "go-live" não é Go.
	got := Keywords([]string{"Java", "Go"}, "", "Front em JavaScript; ajudei no go-live.", []db.UserSkill{{SkillName: "Java"}, {SkillName: "Go"}}, nil)
	if len(got.InResume) != 0 {
		t.Errorf("InResume = %v, want vazio", got.InResume)
	}
}

func TestKeywords_NomeDoCatalogo(t *testing.T) {
	got := Keywords([]string{"postgresql", "golang", "loki"}, "", "", nil, nil)
	if want := []string{"PostgreSQL", "Go", "loki"}; !reflect.DeepEqual(got.NotInProfile, want) {
		t.Errorf("termos = %v, want %v", got.NotInProfile, want)
	}
}
