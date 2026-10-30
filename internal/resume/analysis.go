package resume

import (
	"strings"
	"unicode/utf8"
)

// Recomendações aceitas na análise de aderência.
const (
	RecommendApply        = "candidatar"
	RecommendWithCaveats  = "candidatar_com_ressalvas"
	RecommendLowAlignment = "baixa_aderencia"
)

// FitAnalysis é o que a IA diz sobre candidato × vaga, escrito para o candidato decidir
// se vale se candidatar. Fica na vaga (jobs.fit_analysis), não no currículo.
type FitAnalysis struct {
	Resumo       string   `json:"resumo"`
	PontosFortes []string `json:"pontos_fortes"`
	Lacunas      []string `json:"lacunas"`
	Recomendacao string   `json:"recomendacao"`
}

const (
	maxAnalysisItems  = 4
	maxAnalysisItem   = 200
	maxAnalysisResumo = 600
)

// Clean apara o que a IA devolveu: textos longos demais, listas grandes, itens vazios e
// recomendação fora das três aceitas. Devolve nil quando não sobra nada útil.
func (a *FitAnalysis) Clean() *FitAnalysis {
	if a == nil {
		return nil
	}
	out := &FitAnalysis{
		Resumo:       clip(a.Resumo, maxAnalysisResumo),
		PontosFortes: cleanAnalysisList(a.PontosFortes),
		Lacunas:      cleanAnalysisList(a.Lacunas),
	}
	switch r := strings.ToLower(strings.TrimSpace(a.Recomendacao)); r {
	case RecommendApply, RecommendWithCaveats, RecommendLowAlignment:
		out.Recomendacao = r
	}
	if out.Resumo == "" && len(out.PontosFortes) == 0 && len(out.Lacunas) == 0 {
		return nil
	}
	return out
}

func cleanAnalysisList(items []string) []string {
	out := []string{}
	for _, it := range items {
		if it = clip(it, maxAnalysisItem); it != "" {
			out = append(out, it)
		}
		if len(out) == maxAnalysisItems {
			break
		}
	}
	return out
}

func clip(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max-1]) + "…"
}

// VerifyStrengths remove dos pontos fortes os que citam tecnologia fora do perfil (a
// IA às vezes "completa" a lista com o que a vaga pede: "Docker e Kubernetes" sem
// Kubernetes no perfil). techs são grupos de nomes de uma mesma tecnologia ("golang",
// "go"); a stack da vaga entra junto. Devolve os pontos removidos.
func (a *FitAnalysis) VerifyStrengths(src Source, techs [][]string) (dropped []string) {
	if a == nil || len(a.PontosFortes) == 0 {
		return nil
	}
	for _, t := range src.JobStack {
		techs = append(techs, []string{t})
	}
	idx := newProfileIndex(src)
	kept := a.PontosFortes[:0]
strengths:
	for _, p := range a.PontosFortes {
		for _, group := range techs {
			if !mentionsAny(p, group) {
				continue
			}
			known := false
			for _, term := range group {
				if idx.has(term) {
					known = true
					break
				}
			}
			if !known {
				dropped = append(dropped, p)
				continue strengths
			}
		}
		kept = append(kept, p)
	}
	a.PontosFortes = kept
	return dropped
}

func mentionsAny(text string, terms []string) bool {
	for _, t := range terms {
		if strings.TrimSpace(t) != "" && wordRegexp(t).MatchString(text) {
			return true
		}
	}
	return false
}
