package resume

import (
	"strings"
	"testing"
)

func TestFitAnalysisClean(t *testing.T) {
	if (*FitAnalysis)(nil).Clean() != nil || (&FitAnalysis{Resumo: "  "}).Clean() != nil {
		t.Error("análise vazia vira nil")
	}
	got := (&FitAnalysis{
		Resumo:       "  Boa   aderência técnica. ",
		PontosFortes: []string{"Go expert", "", "gRPC", "Redis", "Docker", "Kafka"},
		Lacunas:      []string{strings.Repeat("x", 300)},
		Recomendacao: " Candidatar_com_Ressalvas ",
	}).Clean()
	if got.Resumo != "Boa aderência técnica." {
		t.Errorf("resumo %q", got.Resumo)
	}
	if len(got.PontosFortes) != 4 || got.PontosFortes[1] != "gRPC" {
		t.Errorf("pontos fortes %v", got.PontosFortes)
	}
	if n := len([]rune(got.Lacunas[0])); n != maxAnalysisItem {
		t.Errorf("lacuna com %d caracteres", n)
	}
	if got.Recomendacao != RecommendWithCaveats {
		t.Errorf("recomendação %q", got.Recomendacao)
	}
	if (&FitAnalysis{Resumo: "ok", Recomendacao: "talvez"}).Clean().Recomendacao != "" {
		t.Error("recomendação desconhecida fica vazia")
	}
}

// Caso real: a IA listou Kubernetes como ponto forte sem ele estar no perfil.
func TestVerifyStrengths(t *testing.T) {
	src := Source{
		Skills:      []string{"Go (Golang)", "Docker", "AWS SQS", "Google Cloud Platform"},
		Aliases:     map[string]string{"golang": "go", "gcp": "google cloud platform"},
		Experiences: []SourceExperience{{Company: "X", Stack: []string{"Go", "gRPC"}, Description: "microsserviços em Go"}},
		JobStack:    []string{"go", "react", "kubernetes"},
	}
	a := &FitAnalysis{PontosFortes: []string{
		"Experiência sólida com Go (Golang) em nível expert.",
		"Proficiência em AWS e GCP, e containerização com Docker e Kubernetes.",
		"Ampla experiência com microsserviços.",
	}}
	techs := [][]string{{"golang", "go"}, {"kubernetes", "k8s"}, {"docker"}, {"gcp", "google cloud"}}
	dropped := a.VerifyStrengths(src, techs)
	if len(dropped) != 1 || !strings.Contains(dropped[0], "Kubernetes") {
		t.Errorf("removidos: %v", dropped)
	}
	if len(a.PontosFortes) != 2 {
		t.Errorf("mantidos: %v", a.PontosFortes)
	}
}
