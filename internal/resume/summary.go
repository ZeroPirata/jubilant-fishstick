package resume

import (
	"regexp"
	"strings"
	"time"
)

// summaryCheck encontra em um texto os trechos que quebram uma regra do resumo.
type summaryCheck struct {
	find func(text string) []string
	msg  func(lang string, found []string) string
}

var (
	reZeroYears = regexp.MustCompile(`(?i)\b0\+?\s*(years?|anos?)\b`)
	reDegree    = regexp.MustCompile(`(?i)\b(degree|bachelor'?s?|graduated|graduate|diploma|forma[çc][ãa]o|formad[oa]|gradua[çc][ãa]o|graduad[oa]|graduand[oa]|bacharel\w*|tecn[óo]log[oa])\b`)
	reInflated  = regexp.MustCompile(`(?i)\b(extensive|proven|solid|strong|deep|years of)\s+(experience|background)\b|\bexperienced\b|experi[êe]ncia\s+(s[óo]lida|ampla|extensa|comprovada)|\bexperiente\b`)
)

func matches(re *regexp.Regexp) func(string) []string {
	return func(text string) []string { return re.FindAllString(text, -1) }
}

func bilingual(pt, en string) func(string, []string) string {
	return func(lang string, found []string) string {
		if lang == "en" {
			return strings.ReplaceAll(en, "%s", strings.Join(found, ", "))
		}
		return strings.ReplaceAll(pt, "%s", strings.Join(found, ", "))
	}
}

func summaryChecks(src Source, now time.Time) []summaryCheck {
	var checks []summaryCheck
	noExperience := YearsOfExperience(src.Experiences, now) == 0

	if noExperience {
		checks = append(checks,
			summaryCheck{matches(reZeroYears), bilingual(
				`O resumo diz "%s". É primeiro emprego: não cite anos de experiência.`,
				`The summary says "%s". This is a first job: do not mention years of experience.`)},
			summaryCheck{matches(reInflated), bilingual(
				`O resumo diz "%s", mas o candidato não tem experiência profissional. Fale de projetos.`,
				`The summary says "%s", but the candidate has no professional experience. Talk about projects.`)},
		)
	}
	if len(src.Education) == 0 {
		checks = append(checks, summaryCheck{matches(reDegree), bilingual(
			`O resumo cita formação ("%s"), mas o input não tem formacoes. Não mencione formação.`,
			`The summary mentions education ("%s"), but the input has no formacoes. Do not mention education.`)})
	}

	// Tecnologia que a vaga pede e o candidato não tem: o erro mais grave, porque
	// o resumo é a primeira coisa que o recrutador lê.
	idx := newProfileIndex(src)
	var unknown []*regexp.Regexp
	var names []string
	for _, tech := range src.JobStack {
		if tech = strings.TrimSpace(tech); tech != "" && !idx.has(tech) {
			unknown = append(unknown, wordRegexp(tech))
			names = append(names, tech)
		}
	}
	if len(unknown) > 0 {
		checks = append(checks, summaryCheck{
			find: func(text string) []string {
				var found []string
				for i, re := range unknown {
					if re.MatchString(text) {
						found = append(found, names[i])
					}
				}
				return found
			},
			msg: bilingual(
				`O resumo cita %s, que não está no perfil do candidato. Cite só tecnologias do input.`,
				`The summary mentions %s, which is not in the candidate's profile. Only mention technologies from the input.`),
		})
	}
	return checks
}

// SummaryViolations lista as regras do resumo que a IA quebrou e que o código
// consegue detectar, já no idioma do currículo (vira instrução de correção).
func SummaryViolations(summary string, src Source, now time.Time) []string {
	var out []string
	for _, c := range summaryChecks(src, now) {
		if found := c.find(summary); len(found) > 0 {
			out = append(out, c.msg(src.Lang, found))
		}
	}
	return out
}

var reSentence = regexp.MustCompile(`[^.!?]+[.!?]*`)

// StripViolatingSentences é o último recurso quando a IA insiste no erro:
// remove do resumo as frases que quebram alguma regra.
func StripViolatingSentences(summary string, src Source, now time.Time) string {
	checks := summaryChecks(src, now)
	var kept []string
	for _, sent := range reSentence.FindAllString(summary, -1) {
		bad := false
		for _, c := range checks {
			if len(c.find(sent)) > 0 {
				bad = true
				break
			}
		}
		if s := strings.TrimSpace(sent); !bad && s != "" {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, " ")
}
