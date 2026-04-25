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
			if techMatch(itemNorm, filtro) {
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

// techMatch verifica se item contém filtro com word-boundary awareness.
// Previne falsos positivos como filtro "go" batendo em "django" ou "gorilla".
//
// Regras:
//  1. match exato: "go" == "go"
//  2. prefix com boundary: HasPrefix só vale se o próximo char for separador ou fim.
//     "node.js" + "node" → ok (next='.'), "gorilla" + "go" → não (next='r')
//  3. token exato: split por separadores, cada token comparado diretamente.
//     "gitlab ci/cd" + "ci" → ok via token
func techMatch(item, filtro string) bool {
	if item == filtro {
		return true
	}
	if strings.HasPrefix(item, filtro) {
		rest := item[len(filtro):]
		if rest == "" || rest[0] == ' ' || rest[0] == '.' || rest[0] == '-' || rest[0] == '/' || rest[0] == '_' {
			return true
		}
	}
	for _, token := range strings.FieldsFunc(item, func(r rune) bool {
		return r == ' ' || r == '.' || r == '-' || r == '/' || r == '_'
	}) {
		if token == filtro {
			return true
		}
	}
	return false
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
