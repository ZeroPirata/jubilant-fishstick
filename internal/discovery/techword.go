package discovery

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Termos de tecnologia que também são palavras comuns em inglês: "go-to company",
// "react quickly", "SWIFT transfer", "spring 2026". Só contam com contexto de
// tecnologia em volta (ver techContext).
var ambiguousTerms = map[string]bool{
	"go": true, "react": true, "swift": true, "rust": true, "spring": true,
	"spark": true, "node": true, "rails": true, "dart": true, "flask": true,
}

// Palavras antes do termo que indicam a tecnologia: "desenvolvedor Go", "Go developer".
// As preposições ("experience in Go") só valem com o termo em maiúscula.
var (
	techLeadWords = map[string]bool{
		"desenvolvedor": true, "desenvolvedora": true, "developer": true, "developers": true,
		"dev": true, "devs": true, "engenheiro": true, "engenheira": true, "engineer": true,
		"engineers": true, "programador": true, "programadora": true, "backend": true,
		"back-end": true, "linguagem": true, "linguagens": true, "language": true, "languages": true,
	}
	techPrepositions = map[string]bool{
		"em": true, "in": true, "with": true, "com": true, "using": true, "usando": true,
	}
	techFollowWords = map[string]bool{
		"developer": true, "developers": true, "dev": true, "devs": true, "engineer": true,
		"engineers": true, "lang": true, "language": true, "programming": true,
		"boot": true, "native": true, "js": true, "framework": true, "backend": true, "back-end": true,
	}
	// Separadores de lista: "Python, Go e Java", "Go/Rust", "Go (Golang)".
	listWords = map[string]bool{"e": true, "and": true, "or": true, "ou": true, "&": true}
)

// listTechTerms são os termos do catálogo de uma palavra só: vizinho de lista de um
// deles ("Python, Go", "Go/Rust"), o termo ambíguo conta.
var listTechTerms = func() map[string]bool {
	m := map[string]bool{}
	for _, t := range Technologies {
		for _, term := range t.Terms {
			if !strings.Contains(term, " ") {
				m[term] = true
			}
		}
	}
	return m
}()

// Palavras que, depois do termo com maiúscula, mostram que é o verbo e não a
// tecnologia: "Go Above and Beyond", "Go To Market", "Go Live".
var nonTechFollowers = map[string]bool{
	"to": true, "for": true, "above": true, "beyond": true, "ahead": true, "live": true,
	"through": true, "back": true, "out": true, "on": true, "get": true, "the": true,
	"a": true, "an": true, "into": true, "further": true, "far": true, "deeper": true,
	"home": true, "fast": true, "green": true, "big": true,
}

// nonTechNames: termo + palavra seguinte que formam o nome de outra coisa. Vagas de
// vendas citam o CRM "Go High Level" ao lado de HubSpot e Zoho.
var nonTechNames = map[string]bool{
	"go high": true, "go highlevel": true,
}

// containsTerm é o containsWord que, para termo ambíguo, exige contexto de
// tecnologia. lower é o texto em minúsculas; orig, o original, serve para ver se o
// termo veio com maiúscula.
func containsTerm(lower, orig, term string) bool {
	if !ambiguousTerms[term] {
		return containsWord(lower, term)
	}
	if len(orig) != len(lower) { // minúscula mudou o tamanho: sem como alinhar
		orig = lower
	}
	for from := 0; from < len(lower); {
		i := strings.Index(lower[from:], term)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(term)
		if boundaryBefore(lower, start) && boundaryAfter(lower, end) && techContext(lower, orig, start, end) {
			return true
		}
		from = start + 1
	}
	return false
}

func techContext(lower, orig string, start, end int) bool {
	// Hífen colado é expressão: "go-to", "go-live", "on-the-go", "no-go".
	if (start > 0 && lower[start-1] == '-') || (end < len(lower) && lower[end] == '-') {
		return false
	}
	// "GO" todo em maiúscula é a sigla de Goiás ("Goiânia/GO"), não a linguagem.
	capital := unicode.IsUpper(rune(orig[start])) && !(end-start <= 2 && strings.ToUpper(orig[start:end]) == orig[start:end])

	prev, prevAt, prevSep := wordBefore(lower, start)
	next, nextAt, nextSep := wordAfter(lower, end)
	// Nome de produto que começa com o termo: "Go High Level" (CRM de vendas).
	if !nextSep && nonTechNames[lower[start:end]+" "+next] {
		return false
	}
	switch {
	case !prevSep && techLeadWords[prev]:
		return true
	case !prevSep && capital && techPrepositions[prev]:
		return true
	case !nextSep && techFollowWords[next]:
		return true
	case aloneOnLine(lower, start, end):
		return true
	// "Usamos Go.", "stack em Go e Postgres": no meio da frase, com maiúscula, é o nome
	// da tecnologia. No começo da frase ou entre aspas ("the 'Go to' company") não dá
	// para saber.
	case capital && !nonTechFollowers[next] && midSentence(orig, start):
		return true
	}
	// Vizinho de lista: separador (ou "e"/"and") e, do outro lado, uma tecnologia.
	// "e/ou" junta separador e palavra: anda até achar uma palavra que não é de lista.
	for i := 0; i < 3 && (prevSep || listWords[prev]); i++ {
		if prev, prevAt, prevSep = wordBefore(lower, prevAt); listTechTerms[prev] {
			return true
		}
	}
	for i := 0; i < 3 && (nextSep || listWords[next]); i++ {
		if next, nextAt, nextSep = wordAfter(lower, nextAt); listTechTerms[next] {
			return true
		}
	}
	return false
}

// aloneOnLine: o termo é o único item da linha ("- Go", tag "react").
func aloneOnLine(s string, start, end int) bool {
	for i := start - 1; i >= 0 && s[i] != '\n'; i-- {
		if !strings.ContainsRune(" \t-*•·>", rune(s[i])) {
			return false
		}
	}
	for i := end; i < len(s) && s[i] != '\n'; i++ {
		if !strings.ContainsRune(" \t.;,:!", rune(s[i])) {
			return false
		}
	}
	return true
}

// midSentence: antes do termo vem texto da mesma frase, não começo de frase, linha
// ou aspas.
func midSentence(s string, start int) bool {
	for i := start - 1; i >= 0; i-- {
		switch c := s[i]; c {
		case ' ', '\t':
			continue
		// Depois de separador ("Anápolis /Go") só vale a regra de lista. Parêntese
		// passa: "(React)", "(Spark)"; a sigla "(GO)" já caiu na regra da maiúscula.
		case '.', '!', '?', '\n', '"', '\'', '-', '*', '/', '|':
			return false
		default:
			// Aspas curvas (‘ “) terminam em 0x98/0x9c depois de 0xe2 0x80.
			if c >= 0x80 && i >= 2 && s[i-2] == 0xe2 && s[i-1] == 0x80 && (c == 0x98 || c == 0x9c || c == 0xa2) {
				return false
			}
			return true
		}
	}
	return false
}

func isListSep(r rune) bool {
	return r == ',' || r == '/' || r == '|' || r == ';' || r == '(' || r == ')' || r == '·' || r == '•'
}

// isTermRune: letras, dígitos e o que aparece dentro de nome de tecnologia (node.js,
// c#, c++, back-end, ci/cd não: a barra separa).
func isTermRune(r rune) bool {
	return isWordRune(r) || r == '.' || r == '-'
}

// wordBefore devolve a palavra que termina antes de i (pulando espaços) e onde ela
// começa; se antes vier um separador de lista, devolve "", a posição dele e true.
func wordBefore(s string, i int) (string, int, bool) {
	for i > 0 {
		r, n := utf8.DecodeLastRuneInString(s[:i])
		if !unicode.IsSpace(r) {
			break
		}
		i -= n
	}
	if i == 0 {
		return "", 0, false
	}
	if r, n := utf8.DecodeLastRuneInString(s[:i]); isListSep(r) {
		return "", i - n, true
	}
	end := i
	for i > 0 {
		r, n := utf8.DecodeLastRuneInString(s[:i])
		if !isTermRune(r) {
			break
		}
		i -= n
	}
	return strings.Trim(s[i:end], ".-"), i, false
}

// wordAfter é o espelho de wordBefore: devolve onde a palavra (ou o separador) termina.
func wordAfter(s string, i int) (string, int, bool) {
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !unicode.IsSpace(r) {
			break
		}
		i += n
	}
	if i >= len(s) {
		return "", len(s), false
	}
	if r, n := utf8.DecodeRuneInString(s[i:]); isListSep(r) {
		return "", i + n, true
	}
	start := i
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !isTermRune(r) {
			break
		}
		i += n
	}
	return strings.Trim(s[start:i], ".-"), i, false
}
