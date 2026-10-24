package profileimport

import (
	"regexp"
	"strings"

	"hackton-treino/internal/resume"
)

// Reserva para quando a IA deixa passar os idiomas (ex.: uma linha "Idiomas: Português
// (Nativo), Inglês (C1 - Avançado)" dentro da seção de habilidades). Só aceita idioma
// seguido de um nível reconhecível, para não pegar "inglês" solto no meio de um texto.
var reLanguageLevel = regexp.MustCompile(`(?i)(?:^|[^\pL])(portugu[eê]s|portuguese|ingl[eê]s|english|espanhol|spanish|franc[eê]s|french|alem[aã]o|german|italiano|italian|japon[eê]s|japanese|mandarim|chin[eê]s|chinese|mandarin|coreano|korean|russo|russian|[aá]rabe|arabic|holand[eê]s|dutch|libras)\s*(?:\(([^)]{1,40})\)|[-–:]\s*([^,;.\n(]{1,40}))`)

// Palavras antes dos códigos do QECR: "C1 - Avançado" vale o que está escrito.
var levelWords = []struct{ level, re string }{
	{"native", `nativ|materna|bil[ií]ngue|bilingual`},
	{"fluent", `fluen|full professional`},
	{"advanced", `avan[cç]ad|advanced|professional working`},
	{"intermediate", `intermedi|limited working|conversa`},
	{"basic", `b[aá]sic|elementar|elementary|iniciante|beginner`},
	{"fluent", `\bc[12]\b`},
	{"advanced", `\bb2\b`},
	{"intermediate", `\bb1\b`},
	{"basic", `\ba[12]\b`},
}

var levelRes = func() []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(levelWords))
	for i, w := range levelWords {
		out[i] = regexp.MustCompile(`(?i)` + w.re)
	}
	return out
}()

func languageLevelFromText(s string) string {
	for i, re := range levelRes {
		if re.MatchString(s) {
			return levelWords[i].level
		}
	}
	return ""
}

// LanguagesFromText acha idiomas com nível escrito no texto do currículo.
func LanguagesFromText(text string) []Language {
	var out []Language
	seen := map[string]bool{}
	for _, m := range reLanguageLevel.FindAllStringSubmatch(text, -1) {
		level := languageLevelFromText(m[2] + m[3])
		code := resume.LanguageCode(m[1])
		if level == "" || seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, Language{Language: code, Proficiency: level})
	}
	return out
}

// MergeLanguages junta ao rascunho os idiomas achados no texto que a IA não trouxe.
func (d *Draft) MergeLanguages(found []Language) {
	have := map[string]bool{}
	for _, l := range d.Languages {
		have[strings.ToLower(l.Language)] = true
	}
	for _, l := range found {
		if !have[strings.ToLower(l.Language)] && len(d.Languages) < maxListEntries {
			have[strings.ToLower(l.Language)] = true
			d.Languages = append(d.Languages, l)
		}
	}
}
