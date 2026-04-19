package worker

import (
	"hackton-treino/internal/db"
	"hackton-treino/internal/scraper"
	"unicode/utf8"
	"hackton-treino/internal/util"
	"strings"
)

// calcularQualidade retorna low/mid/high baseado na proporção de itens do Stack
// da vaga cobertos pelos filtros do usuário.
//
// Usa apenas Stack (tecnologias + domínios técnicos) como denominador — Requirements
// são soft skills (anos de experiência, idioma) que nunca batem com filtros técnicos
// e inflariam o denominador artificialmente.
//
//	< 30% → low  (não gera currículo)
//	30-69% → mid
//	≥ 70%  → high
func calcularQualidade(result *scraper.ResultScraper, filtros []string, aliases map[string]string) db.JobQuality {
	haystack := result.Stack
	if len(haystack) == 0 {
		return db.JobQualityMid
	}
	if len(filtros) == 0 {
		return db.JobQualityMid
	}

	normalizedFiltros := make([]string, len(filtros))
	for i, f := range filtros {
		f = strings.TrimSpace(strings.ToLower(f))
		if canonical, ok := aliases[f]; ok {
			f = canonical
		}
		normalizedFiltros[i] = f
	}

	matched := 0
	for _, item := range haystack {
		itemNorm := util.Normalize(item)
		for _, filtro := range normalizedFiltros {
			if strings.Contains(itemNorm, filtro) || strings.Contains(filtro, itemNorm) {
				matched++
				break
			}
		}
	}

	ratio := float64(matched) / float64(len(haystack))
	switch {
	case ratio >= 0.70:
		return db.JobQualityHigh
	case ratio >= 0.30:
		return db.JobQualityMid
	default:
		return db.JobQualityLow
	}
}

func firstN(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for j := range s {
		if i == n {
			return s[:j] + "…"
		}
		i++
	}
	return s
}
