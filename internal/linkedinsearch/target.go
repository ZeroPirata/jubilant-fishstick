package linkedinsearch

import (
	"hackton-treino/internal/discovery"
	"slices"
	"strings"
	"unicode/utf8"
)

// Generated: uma busca montada pelo Alvo do Descobrir, com o nome que aparece na lista.
type Generated struct {
	Name   string
	Search Search
}

// techLinkedIn: como buscar cada tecnologia no LinkedIn. "Go" sozinho casa com
// qualquer texto em inglês ("go-to", "let's go"); o resto vai pelo nome.
var techLinkedIn = map[string][]string{
	"Go":      {"Golang", "Go developer", "Desenvolvedor Go", "Go engineer"},
	"C#":      {"C#", ".NET"},
	"Node.js": {"Node.js", "NodeJS"},
}

// Níveis do Descobrir → f_E do LinkedIn (2 assistente, 3 júnior, 4 pleno-sênior...).
var levelLinkedIn = map[string][]string{
	discovery.LevelIntern:  {"1"},
	discovery.LevelJunior:  {"2", "3"},
	discovery.LevelMid:     {"4"},
	discovery.LevelSenior:  {"4"},
	discovery.LevelExpert:  {"4", "5"},
	discovery.LevelManager: {"5", "6"},
}

var workplaceLinkedIn = map[string]string{"onsite": "1", "remote": "2", "hybrid": "3"}

// FromTarget monta as buscas do Alvo: a principal (tecnologias E função) e, quando o
// Alvo tem as duas coisas, uma mais larga só com as tecnologias. Nível e modo de
// trabalho vêm do Alvo; o local é o geoId dado (o de uma busca já salva, ou o Brasil).
// Sem tecnologia nem função não há o que buscar: devolve nada.
func FromTarget(t discovery.Target, geoID, geoLabel string) []Generated {
	techs := orGroup(techTerms(t.Technologies, 4))
	roles := orGroup(roleTerms(t.Roles))
	if techs == "" && roles == "" {
		return nil
	}
	base := Search{GeoID: geoID, GeoLabel: geoLabel, Workplace: []string{}, Experience: []string{}, JobTypes: []string{}}
	for _, l := range t.Seniorities {
		base.Experience = append(base.Experience, levelLinkedIn[l]...)
	}
	for _, wt := range t.WorkplaceTypes {
		if v, ok := workplaceLinkedIn[wt]; ok {
			base.Workplace = append(base.Workplace, v)
		}
	}

	techNames := strings.Join(t.Technologies, ", ")
	roleNames := strings.Join(discovery.RoleLabels(t.Roles), ", ")
	// Versões curtas, para quando a completa passa de 200 caracteres: as duas
	// primeiras tecnologias, só com o primeiro termo de cada.
	short := orGroup(techTerms(t.Technologies[:min(2, len(t.Technologies))], 1))
	var out []Generated
	add := func(name string, keywords ...string) {
		s := base
		s.Keywords = firstThatFits(keywords...)
		if cleaned, err := Clean(s); err == nil {
			out = append(out, Generated{Name: truncName(name), Search: cleaned})
		}
	}
	switch {
	case techs != "" && roles != "":
		add(roleNames+" · "+techNames, techs+" AND "+roles, short+" AND "+roles)
		add(techNames+" (qualquer função)", techs, short)
	case techs != "":
		add(techNames, techs, short)
	default:
		add(roleNames, roles)
	}
	return out
}

// firstThatFits: a primeira versão da busca que cabe em 200 caracteres; nenhuma
// cabendo, a última cortada.
func firstThatFits(options ...string) string {
	for _, o := range options {
		if utf8.RuneCountInString(o) <= maxKeywords {
			return o
		}
	}
	last := options[len(options)-1]
	return string([]rune(last)[:maxKeywords])
}

func techTerms(names []string, perTech int) []string {
	var out []string
	for _, n := range names {
		terms := techLinkedIn[n]
		if terms == nil {
			terms = []string{n}
		}
		out = append(out, terms[:min(perTech, len(terms))]...)
	}
	return out
}

func roleTerms(keys []string) []string {
	var out []string
	for _, k := range keys {
		if r, ok := discovery.RoleByKey(k); ok {
			for _, term := range r.LinkedIn {
				if !slices.Contains(out, term) {
					out = append(out, term)
				}
			}
		}
	}
	return out
}

// orGroup: (a OR "b c" OR d). Termo com espaço, hífen, ponto ou # vai entre aspas
// para o LinkedIn não separar.
func orGroup(terms []string) string {
	if len(terms) == 0 {
		return ""
	}
	quoted := make([]string, len(terms))
	for i, t := range terms {
		if strings.ContainsAny(t, " -.#") {
			t = `"` + t + `"`
		}
		quoted[i] = t
	}
	if len(quoted) == 1 {
		return quoted[0]
	}
	return "(" + strings.Join(quoted, " OR ") + ")"
}

func truncName(name string) string {
	if utf8.RuneCountInString(name) <= 80 {
		return name
	}
	return string([]rune(name)[:79]) + "…"
}
