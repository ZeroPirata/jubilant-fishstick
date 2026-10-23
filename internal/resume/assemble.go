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
	for i, e := range src.Experiences {
		bullets, dropped := verifiedBullets(bulletsByID[i], e)
		for _, b := range dropped {
			warnings = append(warnings, "bullet com número inexistente no perfil removido: "+b)
		}
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

// verifiedBullets descarta bullets com números (métricas, percentuais, quantidades)
// que não aparecem em nenhum dado daquela experiência: são métricas inventadas.
func verifiedBullets(bullets []string, e SourceExperience) (kept, dropped []string) {
	evidence := strings.Join(append(append([]string{e.Company, e.Role, e.Description}, e.Stack...), e.Achievements...), " ")
	known := map[string]bool{}
	for _, n := range reNumber.FindAllString(evidence, -1) {
		known[normalizeNumber(n)] = true
	}
	for _, b := range cleanList(bullets) {
		ok := true
		for _, n := range reNumber.FindAllString(b, -1) {
			if !known[normalizeNumber(n)] {
				ok = false
				break
			}
		}
		if ok {
			kept = append(kept, b)
		} else {
			dropped = append(dropped, b)
		}
	}
	return kept, dropped
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
