package discovery

import "strings"

// Palavras curtas muito comuns em cada idioma e raras no outro. "a", "e" e "do"
// ficaram de fora: existem nos dois.
var (
	englishWords = map[string]bool{
		"the": true, "and": true, "with": true, "you": true, "your": true, "we": true,
		"our": true, "for": true, "of": true, "to": true, "is": true, "are": true,
		"will": true, "in": true, "on": true, "experience": true, "team": true, "work": true,
	}
	portugueseWords = map[string]bool{
		"de": true, "que": true, "com": true, "para": true, "voce": true, "os": true,
		"as": true, "uma": true, "um": true, "em": true, "nao": true, "na": true,
		"no": true, "da": true, "das": true, "dos": true, "experiencia": true, "vaga": true,
	}
)

// GuessLanguage estima o idioma da vaga ("en" ou "pt-br") contando palavras comuns de
// cada idioma. É só o padrão do seletor de idioma: o usuário escolhe. Na dúvida (texto
// curto ou misturado), português.
func GuessLanguage(text string) string {
	var en, pt int
	for _, w := range strings.FieldsFunc(foldAccents(strings.ToLower(text)), func(r rune) bool { return !isWordRune(r) }) {
		if englishWords[w] {
			en++
		}
		if portugueseWords[w] {
			pt++
		}
	}
	if en >= 3 && en > pt*2 {
		return "en"
	}
	return "pt-br"
}
