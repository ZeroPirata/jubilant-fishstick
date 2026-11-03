// Package jobfit calcula a compatibilidade entre a stack de uma vaga e as habilidades
// do candidato. É o cálculo único usado pelo worker (qualidade gravada na vaga) e pela
// listagem de vagas (detalhes da aba Qualidade), para a tela e a nota nunca divergirem.
package jobfit

import (
	"hackton-treino/internal/db"
	"hackton-treino/internal/discovery"
	"math"
	"strings"
)

const (
	StatusStrong  = "strong"  // tem a skill em nível avançado ou expert
	StatusPartial = "partial" // tem, mas em nível básico ou intermediário
	StatusMissing = "missing"
)

// Peso de cada nível na cobertura: saber o básico de React não vale o mesmo que ser
// expert em Go. Nível vazio (dado antigo) conta como forte.
var levelWeight = map[db.SkillLevel]float64{
	db.SkillLevelExpert:       1,
	db.SkillLevelAdvanced:     1,
	db.SkillLevelIntermediate: 0.75,
	db.SkillLevelBasic:        0.4,
	"":                        1,
}

// Limiares da qualidade sobre a cobertura ponderada.
const (
	highFrom = 70
	midFrom  = 30
)

// minItemsForHigh: com menos tecnologias que isso a vaga não fica com compatibilidade
// alta, mesmo cobrindo 100%. Uma vaga que só cita "Go" diz pouco sobre o que ela pede
// (e às vezes nem é de desenvolvimento): não deve passar na frente nem gerar alerta.
const minItemsForHigh = 3

type Item struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	// Skill e Level: a skill do perfil que cobre o item (a de maior nível).
	Skill string `json:"skill,omitempty"`
	Level string `json:"level,omitempty"`
}

type Fit struct {
	// CoveragePct: cobertura da stack ponderada pelo nível (0–100).
	CoveragePct int           `json:"coverage_pct"`
	Quality     db.JobQuality `json:"quality"`
	Items       []Item        `json:"items"`
	Strong      int           `json:"strong"`
	Partial     int           `json:"partial"`
	Missing     int           `json:"missing"`
}

type candidate struct {
	names []string // nome da skill e as partes dele
	tags  []string
	skill db.UserSkill
}

// Evaluate compara cada item da stack (já normalizado pelos aliases no scrape) com o
// nome da skill e as partes dele ("Go (Golang)" → "go", "golang"); só sem nenhuma
// skill pelo nome é que as tags contam. Assim "MySQL" intermediário vale mais que a
// tag "mysql" da skill "SQL" avançado. Sem stack ou sem skills não há o que medir:
// qualidade média, sem itens.
func Evaluate(stack []string, skills []db.UserSkill, aliases map[string]string) Fit {
	return NewEvaluator(skills, aliases).Evaluate(stack)
}

// Evaluator avalia várias stacks contra as mesmas skills (o ranking do Descobrir
// avalia milhares de vagas). Guarda o resultado de cada item: as vagas repetem as
// mesmas tecnologias. Não é seguro para uso concorrente.
type Evaluator struct {
	cands   []candidate
	aliases map[string]string
	memo    map[string]itemResult
}

type itemResult struct {
	best  float64
	skill string
	level string
}

func NewEvaluator(skills []db.UserSkill, aliases map[string]string) *Evaluator {
	e := &Evaluator{aliases: aliases, memo: map[string]itemResult{}, cands: make([]candidate, 0, len(skills))}
	for _, s := range skills {
		c := candidate{skill: s}
		for _, t := range discovery.NewSkill(s.SkillName).Terms {
			c.names = append(c.names, canonical(t, aliases))
		}
		for _, tag := range s.Tags {
			if t := canonical(tag, aliases); t != "" {
				c.tags = append(c.tags, t)
			}
		}
		e.cands = append(e.cands, c)
	}
	return e
}

func (e *Evaluator) Evaluate(stack []string) Fit {
	fit := Fit{Quality: db.JobQualityMid, Items: []Item{}}
	if len(stack) == 0 || len(e.cands) == 0 {
		return fit
	}

	var score float64
	for _, item := range stack {
		norm := canonical(item, e.aliases)
		if norm == "" {
			continue
		}
		res := e.item(norm)
		it := Item{Name: item, Status: StatusMissing, Skill: res.skill, Level: res.level}
		switch {
		case res.best >= 1:
			it.Status = StatusStrong
			fit.Strong++
		case res.best > 0:
			it.Status = StatusPartial
			fit.Partial++
		default:
			fit.Missing++
		}
		if res.best > 0 {
			score += res.best
		}
		fit.Items = append(fit.Items, it)
	}
	if len(fit.Items) == 0 {
		return fit
	}

	fit.CoveragePct = int(math.Round(score * 100 / float64(len(fit.Items))))
	switch {
	case fit.CoveragePct >= highFrom && len(fit.Items) >= minItemsForHigh:
		fit.Quality = db.JobQualityHigh
	case fit.CoveragePct >= midFrom:
		fit.Quality = db.JobQualityMid
	default:
		fit.Quality = db.JobQualityLow
	}
	return fit
}

// item acha a skill de maior nível que cobre o item: pelo nome primeiro, pelas tags
// só se nenhum nome cobrir.
func (e *Evaluator) item(norm string) itemResult {
	if r, ok := e.memo[norm]; ok {
		return r
	}
	r := itemResult{best: -1}
	for _, byTag := range []bool{false, true} {
		for _, c := range e.cands {
			terms := c.names
			if byTag {
				terms = c.tags
			}
			w := levelWeight[c.skill.ProficiencyLevel]
			if w <= r.best || !matchesAny(norm, terms) {
				continue
			}
			r.best = w
			r.skill, r.level = c.skill.SkillName, string(c.skill.ProficiencyLevel)
		}
		if r.best >= 0 {
			break
		}
	}
	e.memo[norm] = r
	return r
}

func canonical(s string, aliases map[string]string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if c, ok := aliases[s]; ok {
		return c
	}
	return s
}

func matchesAny(item string, terms []string) bool {
	for _, t := range terms {
		if t != "" && TechMatch(item, t) {
			return true
		}
	}
	return false
}

// TechMatch verifica se o item da stack contém o termo com fronteira de palavra:
// "go" não bate em "django" nem "gorilla".
//
//  1. igual: "go" == "go"
//  2. prefixo seguido de separador: "node.js" + "node"
//  3. token igual: "gitlab ci/cd" + "ci"
func TechMatch(item, term string) bool {
	if item == term {
		return true
	}
	if strings.HasPrefix(item, term) {
		rest := item[len(term):]
		if rest == "" || rest[0] == ' ' || rest[0] == '.' || rest[0] == '-' || rest[0] == '/' || rest[0] == '_' {
			return true
		}
	}
	for _, token := range strings.FieldsFunc(item, func(r rune) bool {
		return r == ' ' || r == '.' || r == '-' || r == '/' || r == '_'
	}) {
		if token == term {
			return true
		}
	}
	return false
}
