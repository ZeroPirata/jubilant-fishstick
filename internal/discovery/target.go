package discovery

import (
	"slices"
	"sort"
	"strings"
)

// Target é o "Alvo" do Descobrir: a função, as tecnologias principais, o nível e o
// modo de trabalho que a pessoa procura. Fica nas preferências do Descobrir e vale
// para a lista, os avisos, a Seleção com IA e as buscas geradas no LinkedIn.
type Target struct {
	Roles          []string `json:"roles"`
	Technologies   []string `json:"technologies"`
	Seniorities    []string `json:"seniorities"`
	WorkplaceTypes []string `json:"workplace_types"`
}

// ParsedTarget é o que ParseTarget entendeu de um texto livre, e o que não entendeu.
type ParsedTarget struct {
	Target
	// Unknown: palavras que não viraram nada (a tela mostra "não entendi: …").
	Unknown []string `json:"unknown"`
}

// targetPhrase: um termo do texto e o que ele quer dizer.
type targetPhrase struct {
	text  string // em minúsculas e sem acento
	kind  string // role, tech, level, workplace
	value string
}

// Palavras que só ligam as outras ("vaga de desenvolvedor back-end em Go"): não são
// "não entendi".
var targetFiller = map[string]bool{
	"de": true, "da": true, "do": true, "das": true, "dos": true, "em": true, "e": true, "ou": true, "com": true,
	"para": true, "pra": true, "a": true, "o": true, "as": true, "os": true, "no": true, "na": true, "um": true,
	"uma": true, "vaga": true, "vagas": true, "desenvolvedor": true, "desenvolvedora": true, "developer": true,
	"dev": true, "engenheiro": true, "engenheira": true, "engineer": true, "programador": true, "programadora": true,
	"software": true, "analista": true, "and": true, "or": true, "with": true, "in": true, "of": true, "nivel": true,
	"trabalho": true, "modelo": true, "quero": true, "procuro": true,
}

// Termos de nível e modo de trabalho que a pessoa escreve (os marcadores de vaga têm
// abreviações que no texto livre confundem: "pl", "ii").
var (
	targetLevelWords = map[string][]string{
		LevelIntern:  {"estagio", "estagiario", "estagiaria", "intern", "internship", "trainee"},
		LevelJunior:  {"junior", "jr", "entry level"},
		LevelMid:     {"pleno", "plena", "mid", "mid-level", "mid level"},
		LevelSenior:  {"senior", "sr"},
		LevelExpert:  {"especialista", "staff", "principal", "tech lead", "lead"},
		LevelManager: {"gestao", "gerente", "manager", "coordenador", "coordenadora", "head"},
	}
	targetWorkplaceWords = map[string][]string{
		"remote": {"remoto", "remota", "remote", "home office", "anywhere"},
		"hybrid": {"hibrido", "hibrida", "hybrid"},
		"onsite": {"presencial", "onsite", "on-site", "escritorio"},
		Abroad:   {"exterior", "internacional", "abroad", "gringa"},
	}
)

// targetPhrases: todos os termos reconhecidos, do mais longo para o mais curto
// ("react native" antes de "react", "back end" antes de "end").
var targetPhrases = func() []targetPhrase {
	var out []targetPhrase
	add := func(kind, value string, terms ...string) {
		for _, t := range terms {
			out = append(out, targetPhrase{text: foldAccents(strings.ToLower(t)), kind: kind, value: value})
		}
	}
	for _, r := range Roles {
		add("role", r.Key, r.Markers...)
	}
	for _, t := range Technologies {
		add("tech", t.Name, t.Terms...)
		add("tech", t.Name, t.Name)
	}
	for level, words := range targetLevelWords {
		add("level", level, words...)
	}
	for wt, words := range targetWorkplaceWords {
		add("workplace", wt, words...)
	}
	sort.SliceStable(out, func(a, b int) bool { return len(out[a].text) > len(out[b].text) })
	return out
}()

// ParseTarget entende um texto livre como "golang backend remoto pleno": Go, Back-end,
// remoto, pleno. Aqui "go" sozinho é Go (a pessoa escreveu procurando a linguagem; na
// vaga o mesmo termo exige contexto). O que sobra volta em Unknown.
func ParseTarget(text string) ParsedTarget {
	p := ParsedTarget{Target: Target{Roles: []string{}, Technologies: []string{}, Seniorities: []string{}, WorkplaceTypes: []string{}}, Unknown: []string{}}
	rest := " " + foldAccents(strings.ToLower(strings.Join(strings.Fields(text), " "))) + " "
	found := map[string]map[string]bool{"role": {}, "tech": {}, "level": {}, "workplace": {}}
	for _, ph := range targetPhrases {
		if ph.text == "" || !containsWord(rest, ph.text) {
			continue
		}
		found[ph.kind][ph.value] = true
		rest = removeWord(rest, ph.text)
	}
	// Na ordem dos catálogos, não na do texto: a mesma busca sempre fica igual.
	for _, r := range Roles {
		if found["role"][r.Key] {
			p.Roles = append(p.Roles, r.Key)
		}
	}
	for _, t := range Technologies {
		if found["tech"][t.Name] {
			p.Technologies = append(p.Technologies, t.Name)
		}
	}
	for _, l := range SeniorityLevels {
		if found["level"][l] {
			p.Seniorities = append(p.Seniorities, l)
		}
	}
	for _, wt := range LocationModes {
		if found["workplace"][wt] {
			p.WorkplaceTypes = append(p.WorkplaceTypes, wt)
		}
	}
	for _, w := range strings.FieldsFunc(rest, func(r rune) bool { return !isWordRune(r) && r != '.' && r != '-' }) {
		w = strings.Trim(w, ".-")
		if w != "" && !targetFiller[w] && !slices.Contains(p.Unknown, w) {
			p.Unknown = append(p.Unknown, w)
		}
	}
	return p
}

// removeWord tira do texto todas as ocorrências do termo como palavra inteira.
func removeWord(text, word string) string {
	for from := 0; from < len(text); {
		i := strings.Index(text[from:], word)
		if i < 0 {
			break
		}
		start, end := from+i, from+i+len(word)
		if boundaryBefore(text, start) && boundaryAfter(text, end) {
			text = text[:start] + " " + text[end:]
			from = start + 1
			continue
		}
		from = start + 1
	}
	return text
}

// RoleLabels: os nomes das funções, para a tela.
func RoleLabels(keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if r, ok := RoleByKey(k); ok {
			out = append(out, r.Label)
		}
	}
	return out
}
