package resume

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

const maxProjects = 3

// YearsOfExperience soma o tempo de cada cargo, sem contar duas vezes períodos
// sobrepostos nem os intervalos entre empregos, e arredonda para baixo.
func YearsOfExperience(exps []SourceExperience, now time.Time) int {
	type span struct{ start, end time.Time }
	spans := make([]span, 0, len(exps))
	for _, e := range exps {
		end := e.End
		if e.Current || end.IsZero() {
			end = now
		}
		if e.Start.IsZero() || !end.After(e.Start) {
			continue
		}
		spans = append(spans, span{e.Start, end})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start.Before(spans[j].start) })

	months := 0
	var cur *span
	flush := func() {
		if cur != nil {
			months += monthsBetween(cur.start, cur.end)
		}
	}
	for i := range spans {
		s := spans[i]
		if cur != nil && !s.start.After(cur.end) {
			if s.end.After(cur.end) {
				cur.end = s.end
			}
			continue
		}
		flush()
		cur = &s
	}
	flush()
	return months / 12
}

func monthsBetween(a, b time.Time) int {
	m := (b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
	if m < 0 {
		return 0
	}
	return m
}

// Assemble junta o texto da IA com os fatos do banco. Também aplica no código as
// regras anti-alucinação que o prompt sozinho não garante. Os avisos listam o que
// foi descartado, para log.
func Assemble(src Source, out LLMOutput) (Doc, []string) {
	var warnings []string
	doc := Doc{
		Version:        DocVersion,
		Lang:           src.Lang,
		Header:         src.Header,
		Summary:        strings.TrimSpace(out.Resumo),
		Education:      src.Education,
		Certifications: src.Certifications,
	}

	bulletsByID := map[int][]string{}
	for _, e := range out.Experiencias {
		bulletsByID[e.ID] = e.Bullets
	}
	evidences := make([]experienceEvidence, len(src.Experiences))
	for i, e := range src.Experiences {
		evidences[i] = src.evidenceFor(e)
	}
	for i, e := range src.Experiences {
		bullets, dropped := src.verifiedBullets(bulletsByID[i], i, evidences)
		warnings = append(warnings, dropped...)
		if len(bullets) == 0 {
			// A IA não escreveu (ou só inventou): usa as conquistas cadastradas.
			bullets = cleanList(e.Achievements)
		}
		exp := Experience{
			Company: e.Company, Role: e.Role,
			Start: MonthYear(e.Start), Current: e.Current, Bullets: bullets,
		}
		if !e.Current {
			exp.End = MonthYear(e.End)
		}
		doc.Experiences = append(doc.Experiences, exp)
	}

	idx := newProfileIndex(src)
	for _, g := range out.Habilidades {
		var items []string
		seen := map[string]bool{}
		for _, it := range g.Itens {
			it = strings.TrimSpace(it)
			c := src.canonical(it)
			if it == "" || seen[c] {
				continue
			}
			if !idx.has(it) {
				warnings = append(warnings, "habilidade fora do perfil removida: "+it)
				continue
			}
			seen[c] = true
			items = append(items, it)
		}
		if len(items) > 0 {
			doc.Skills = append(doc.Skills, SkillGroup{Category: strings.TrimSpace(g.Categoria), Items: items})
		}
	}

	used := map[int]bool{}
	for _, p := range out.Projetos {
		if p.ID < 0 || p.ID >= len(src.Projects) || used[p.ID] || len(doc.Projects) == maxProjects {
			continue
		}
		used[p.ID] = true
		sp := src.Projects[p.ID]
		desc := strings.TrimSpace(p.Descricao)
		if desc == "" {
			desc = sp.Description
		}
		doc.Projects = append(doc.Projects, Project{Name: sp.Name, URL: sp.URL, Description: desc})
	}

	return doc, warnings
}

// profileIndex responde se uma tecnologia aparece no perfil: em habilidades, na stack
// das experiências (com aliases, "Golang" = "Go") ou escrita em conquistas, descrições
// e projetos ("Hexagonal Architecture" numa conquista, "AWS" em "AWS SQS").
type profileIndex struct {
	src     Source
	known   map[string]bool
	written string
}

func newProfileIndex(src Source) profileIndex {
	idx := profileIndex{src: src, known: map[string]bool{}}
	var text strings.Builder
	for _, s := range src.Skills {
		idx.known[src.canonical(s)] = true
	}
	for _, e := range src.Experiences {
		for _, s := range e.Stack {
			idx.known[src.canonical(s)] = true
		}
		text.WriteString(e.Description + " " + strings.Join(e.Achievements, " ") + " " + strings.Join(e.Stack, " ") + " ")
	}
	for _, p := range src.Projects {
		text.WriteString(p.Name + " " + p.Description + " ")
	}
	idx.written = text.String()
	return idx
}

func (idx profileIndex) has(item string) bool {
	return idx.known[idx.src.canonical(item)] || wordRegexp(item).MatchString(idx.written)
}

// wordRegexp casa o termo como palavra inteira, sem diferenciar maiúsculas:
// "Go" não casa dentro de "Google"; "C#" e "C++" funcionam.
func wordRegexp(term string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(^|[^\pL\pN+#])` + regexp.QuoteMeta(strings.TrimSpace(term)) + `($|[^\pL\pN+#])`)
}

func (s Source) canonical(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if c, ok := s.Aliases[n]; ok {
		return c
	}
	return n
}

var reNumber = regexp.MustCompile(`\b\d+(?:[.,]\d+)?\b`)

// experienceEvidence é tudo o que o perfil diz sobre uma experiência.
type experienceEvidence struct {
	text    string          // empresa, cargo, descrição, stack e conquistas
	techs   map[string]bool // stack em nome canônico
	numbers map[string]bool
}

func (s Source) evidenceFor(e SourceExperience) experienceEvidence {
	ev := experienceEvidence{
		text:    strings.Join(append(append([]string{e.Company, e.Role, e.Description}, e.Stack...), e.Achievements...), " "),
		techs:   map[string]bool{},
		numbers: map[string]bool{},
	}
	for _, t := range e.Stack {
		ev.techs[s.canonical(t)] = true
	}
	for _, n := range reNumber.FindAllString(ev.text, -1) {
		ev.numbers[normalizeNumber(n)] = true
	}
	return ev
}

// evidencedIn diz se a tecnologia aparece na experiência (stack, com aliases, ou no texto).
func (s Source) evidencedIn(tech string, ev experienceEvidence) bool {
	return ev.techs[s.canonical(tech)] || mentions(ev.text, tech)
}

// foreignTechs são as tecnologias de OUTRAS experiências que esta não menciona,
// com as variações de nome (aliases) de cada uma: "Google Cloud Platform" também
// procura "GCP" se o alias existir.
func (s Source) foreignTechs(i int, evidences []experienceEvidence) []string {
	variants := map[string][]string{}
	for v, c := range s.Aliases {
		variants[c] = append(variants[c], v)
	}
	seen := map[string]bool{}
	var out []string
	for j, e := range s.Experiences {
		if j == i {
			continue
		}
		for _, t := range e.Stack {
			c := s.canonical(t)
			if seen[c] || s.evidencedIn(t, evidences[i]) {
				continue
			}
			seen[c] = true
			out = append(out, t)
			if c != strings.ToLower(strings.TrimSpace(t)) {
				out = append(out, c)
			}
			out = append(out, variants[c]...)
		}
	}
	return out
}

// verifiedBullets descarta bullets que afirmam o que o perfil não sustenta para
// AQUELA experiência: números (métricas, percentuais) que não aparecem nos dados dela,
// ou tecnologias que só aparecem em outra experiência (fato atribuído à empresa errada).
// Devolve os mantidos e o motivo de cada descarte.
func (s Source) verifiedBullets(bullets []string, i int, evidences []experienceEvidence) (kept, dropped []string) {
	ev := evidences[i]
	foreign := s.foreignTechs(i, evidences)
	company := s.Experiences[i].Company

bullets:
	for _, b := range cleanList(bullets) {
		for _, n := range reNumber.FindAllString(b, -1) {
			if !ev.numbers[normalizeNumber(n)] {
				dropped = append(dropped, "bullet com número inexistente no perfil removido: "+b)
				continue bullets
			}
		}
		for _, t := range foreign {
			if mentions(b, t) {
				dropped = append(dropped, "bullet de "+company+" citava "+t+", que é de outra experiência, removido: "+b)
				continue bullets
			}
		}
		kept = append(kept, b)
	}
	return kept, dropped
}

// mentions procura o termo como palavra inteira. Termos de até 2 letras ("Go", "C", "R")
// exigem a mesma caixa, para não casar com palavras comuns do texto.
func mentions(text, term string) bool {
	term = strings.TrimSpace(term)
	if term == "" {
		return false
	}
	if len([]rune(term)) <= 2 {
		return regexp.MustCompile(`(^|[^\pL\pN+#])` + regexp.QuoteMeta(term) + `($|[^\pL\pN+#])`).MatchString(text)
	}
	return wordRegexp(term).MatchString(text)
}

func normalizeNumber(n string) string {
	return strings.ReplaceAll(n, ",", ".")
}

func cleanList(items []string) []string {
	var out []string
	for _, it := range items {
		it = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(it), "-•*"))
		if it != "" {
			out = append(out, it)
		}
	}
	return out
}
