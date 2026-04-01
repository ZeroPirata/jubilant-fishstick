package worker

import (
	"hackton-treino/internal/scraper"
	"hackton-treino/internal/util"
	"strings"
)

func (w *Worker) isVagaRelevante(result *scraper.ScraperResult) bool {
	filtrosRaw := w.filtros

	filtros := make([]string, len(filtrosRaw))
	for i, f := range filtrosRaw {
		filtros[i] = util.Normalize(f)
	}

	haystack := append(result.Stack, result.Requirements...)
	for _, item := range haystack {
		itemNormalizado := util.Normalize(item)
		for _, filtro := range filtros {
			if strings.Contains(itemNormalizado, filtro) {
				return true
			}
		}
	}

	return false
}

func (w *Worker) extrairKeywordsDaVaga(result *scraper.ScraperResult) []string {
	filtros := w.filtros

	textoVaga := strings.ToLower(strings.Join(result.Stack, " ") + strings.Join(result.Requirements, " "))

	var keywordsEncontradas []string
	for _, filtro := range filtros {
		filtroLow := strings.ToLower(filtro)
		if strings.Contains(textoVaga, filtroLow) {
			keywordsEncontradas = append(keywordsEncontradas, filtroLow)
		}
	}
	return keywordsEncontradas
}
