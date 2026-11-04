package discovery

import (
	"hash/fnv"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// O Descobrir não lê mais o texto das vagas a cada requisição. Na varredura, cada vaga
// ganha a lista das tecnologias do catálogo que cita (coluna techs, com índice GIN) e
// o idioma estimado. Skills do perfil fora do catálogo ("APIs REST") vão para a busca
// de texto do Postgres (search_vec), também com índice. Ver repository/migrations/00023.

// techRulesRevision muda quando as regras de casamento mudam sem mudar o catálogo
// (techword.go): força o recálculo de techs de todas as vagas.
const techRulesRevision = 3 // 2: "Go High Level" (CRM) não é Go; 3: observabilidade e infra

// TechsVersion identifica o catálogo e as regras com que techs foi calculada. Vaga
// gravada com outra versão é recalculada por Service.BackfillTechs.
var TechsVersion = func() int64 {
	h := fnv.New64a()
	h.Write([]byte(strconv.Itoa(techRulesRevision)))
	for _, t := range Technologies {
		h.Write([]byte("\x00" + t.Name + "\x01" + strings.Join(t.Terms, "\x02")))
	}
	// Positivo: a coluna é BIGINT e 0 é "nunca calculada".
	return int64(h.Sum64()>>1) | 1
}()

// JobIndex é o que a varredura grava para o Descobrir filtrar e ordenar sem ler o texto.
type JobIndex struct {
	Techs    []string
	Language string
}

// IndexJob calcula as tecnologias citadas (com as regras de contexto: "go-to" não é
// Go) e o idioma da vaga.
func IndexJob(title, description string) JobIndex {
	text := title + "\n" + description
	techs := MatchTechs(text, TechNames)
	if techs == nil {
		techs = []string{}
	}
	return JobIndex{Techs: techs, Language: GuessLanguage(text)}
}

// SkillPlan diz como achar cada skill do perfil no catálogo indexado: pelas
// tecnologias que ela cobre (techs && ...) e, para o que sobra do nome dela, pela
// busca de texto (frases sem símbolos).
type SkillPlan struct {
	skills []Skill
	// techs[i]: tecnologias do catálogo cobertas pela skill i.
	techs [][]string
	// termSkills: termo da busca de texto → índices das skills que ele representa.
	termSkills map[string][]int
	// AllTechs: todas as tecnologias cobertas, para o filtro com índice.
	AllTechs []string
	// Terms: os termos da busca de texto, sem repetição.
	Terms []string
}

// PlanSkills monta o plano. Termos que já são de uma tecnologia coberta não vão para a
// busca de texto. Também ficam fora os termos com símbolos ("c#", "node.js": a busca
// de texto quebra "c#" em "c") e os termos ambíguos ("go"), que só valem com as regras
// de contexto, aplicadas na varredura.
func PlanSkills(skills []Skill) SkillPlan {
	p := SkillPlan{skills: skills, techs: make([][]string, len(skills)), termSkills: map[string][]int{}}
	for i, s := range skills {
		covered := map[string]bool{}
		for _, name := range TechNames {
			if TechCoveredBy(name, s) {
				p.techs[i] = append(p.techs[i], name)
				t, _ := TechByName(name)
				for _, term := range t.Terms {
					covered[term] = true
				}
				if !slices.Contains(p.AllTechs, name) {
					p.AllTechs = append(p.AllTechs, name)
				}
			}
		}
		for _, term := range s.Terms {
			if covered[term] || !textSearchable(term) {
				continue
			}
			if _, ok := p.termSkills[term]; !ok {
				p.Terms = append(p.Terms, term)
			}
			if !slices.Contains(p.termSkills[term], i) {
				p.termSkills[term] = append(p.termSkills[term], i)
			}
		}
	}
	if p.AllTechs == nil {
		p.AllTechs = []string{}
	}
	return p
}

// textSearchable: só letras, dígitos, espaço e hífen, com 2+ caracteres, e não ambíguo.
func textSearchable(term string) bool {
	if len([]rune(term)) < 2 || ambiguousTerms[term] {
		return false
	}
	for _, r := range term {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != ' ' && r != '-' {
			return false
		}
	}
	return true
}

// Matched devolve, na ordem do perfil, as skills que a vaga cita: por uma tecnologia
// coberta em techs ou por um termo achado na busca de texto (hitTerms).
func (p SkillPlan) Matched(techs []string, hitTerms []string) []string {
	hit := make([]bool, len(p.skills))
	for _, term := range hitTerms {
		for _, i := range p.termSkills[term] {
			hit[i] = true
		}
	}
	var out []string
	seen := map[string]bool{}
	for i, s := range p.skills {
		if !hit[i] {
			for _, t := range p.techs[i] {
				if slices.Contains(techs, t) {
					hit[i] = true
					break
				}
			}
		}
		key := strings.ToLower(strings.TrimSpace(s.Name))
		if hit[i] && !seen[key] {
			seen[key] = true
			out = append(out, s.Name)
		}
	}
	return out
}
