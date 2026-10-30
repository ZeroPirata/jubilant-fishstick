package discovery

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Skill é uma skill do perfil com os termos que a representam no texto de uma vaga.
type Skill struct {
	Name  string
	Terms []string
}

var parenthetical = regexp.MustCompile(`\(([^)]*)\)`)

// NewSkill monta os termos de busca de uma skill. O nome raramente aparece igual na
// vaga ("Go (Golang)", "Concorrência / Goroutines"), então entram também o nome sem
// o parêntese, o que está dentro dele e as partes separadas por "/". Tags ficam de
// fora: mesmo a primeira costuma ser ampla demais ("aws" em "AWS SQS").
func NewSkill(name string) Skill {
	candidates := []string{name, parenthetical.ReplaceAllString(name, "")}
	for _, m := range parenthetical.FindAllStringSubmatch(name, -1) {
		candidates = append(candidates, m[1])
	}
	for _, c := range append([]string(nil), candidates...) {
		if strings.Contains(c, "/") {
			candidates = append(candidates, strings.Split(c, "/")...)
		}
	}

	skill := Skill{Name: name}
	seen := map[string]bool{}
	for _, c := range candidates {
		t := strings.ToLower(strings.Join(strings.Fields(c), " "))
		if t != "" && !seen[t] {
			seen[t] = true
			skill.Terms = append(skill.Terms, t)
		}
	}
	return skill
}

// MatchSkills devolve o nome das skills com algum termo no texto da vaga, como
// palavra inteira: "go" não bate em "google" nem "c" em "c#". Termo que também é
// palavra comum ("go-to") só conta com contexto de tecnologia (containsTerm).
func MatchSkills(text string, skills []Skill) []string {
	orig, text := text, strings.ToLower(text)
	var matched []string
	seen := map[string]bool{}
	for _, s := range skills {
		key := strings.ToLower(strings.TrimSpace(s.Name))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		for _, term := range s.Terms {
			if containsTerm(text, orig, term) {
				matched = append(matched, s.Name)
				break
			}
		}
	}
	return matched
}

// RegexPattern é o filtro para o ~* do Postgres: qualquer termo cercado por algo que
// não é letra/dígito. Pega um pouco a mais que MatchSkills (o "c" de "c#"), nunca menos.
// Um regex só com alternância é bem mais rápido que um ~* ANY com um regex por termo.
func RegexPattern(skills []Skill) string {
	var terms []string
	for _, s := range skills {
		for _, term := range s.Terms {
			terms = append(terms, regexp.QuoteMeta(term))
		}
	}
	return `(^|[^[:alnum:]])(` + strings.Join(terms, "|") + `)([^[:alnum:]]|$)`
}

func containsWord(text, word string) bool {
	for from := 0; from < len(text); {
		i := strings.Index(text[from:], word)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(word)
		if boundaryBefore(text, start) && boundaryAfter(text, end) {
			return true
		}
		from = start + 1
	}
	return false
}

// '+' e '#' contam como parte da palavra (c++, c#); '.' depois da palavra não
// ("node" bate em "node.js", e em "go." no fim da frase).
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '#'
}

func boundaryBefore(text string, i int) bool {
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(text[:i])
	return !isWordRune(r) && r != '.'
}

func boundaryAfter(text string, i int) bool {
	if i >= len(text) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(text[i:])
	return !isWordRune(r)
}
