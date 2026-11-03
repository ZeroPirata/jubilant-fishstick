package discovery

import (
	"regexp"
	"slices"
	"strings"
)

// Níveis de senioridade, do menor para o maior. Vaga sem nível reconhecido fica com a
// lista vazia ("não informado"); vaga "Pleno/Sênior" fica com os dois.
const (
	LevelIntern = "intern"
	LevelJunior = "junior"
	LevelMid    = "mid"
	LevelSenior = "senior"
	LevelExpert = "expert"
	// LevelManager: cargos de gestão (gerente, diretor, head, coordenador...). Não é
	// um degrau acima de especialista, é outra trilha; fica separado para quem procura
	// vaga técnica poder deixar de fora.
	LevelManager = "manager"
)

var SeniorityLevels = []string{LevelIntern, LevelJunior, LevelMid, LevelSenior, LevelExpert, LevelManager}

// Marcadores em minúsculas e sem acento, procurados como palavra inteira. Ficaram de
// fora os que dão falso positivo demais: "mid" sozinho ("mid-market") e "specialist"
// ("Support Specialist" é cargo de entrada; "Specialist I" sai pelo numeral). Os
// numerais "I" e "IV" têm regra própria (romanLevel).
var seniorityMarkers = []struct {
	level   string
	markers []string
}{
	{LevelIntern, []string{"estagio", "estagiario", "estagiaria", "intern", "internship", "trainee", "aprendiz"}},
	{LevelJunior, []string{"junior", "jr", "entry level", "entry-level"}},
	{LevelMid, []string{"pleno", "plena", "pl", "mid-level", "mid level", "mid-senior", "ii"}},
	{LevelSenior, []string{"senior", "sr", "mid-senior", "iii"}},
	{LevelExpert, []string{"especialista", "staff", "principal", "lead", "leader", "lider", "tech lead", "arquiteto", "arquiteta", "architect"}},
	// "Product/Account Manager" entram aqui também: não gerem pessoas, mas também não
	// são vaga de entrada. "Head" é palavra inteira ("headless" não casa).
	{LevelManager, []string{"manager", "gerente", "gestor", "gestora", "director", "diretor", "diretora", "head", "vp", "vice president", "vice-president",
		"coordenador", "coordenadora", "coordinator", "supervisor", "supervisora", "superintendente", "cto", "cio", "ciso", "ceo"}},
}

// romanLevel: "Analista I", "Specialist I | Investimentos", "Engineer IV". O numeral
// vem depois de pelo menos uma palavra e fecha o título ou vem antes de um separador:
// assim "I" não casa no meio de uma frase. "II" e "III" já estão nos marcadores.
var romanLevel = regexp.MustCompile(`\S\s+(i|iv)\s*(?:$|[|(,/:\-–—])`)

var romanLevels = map[string]string{"i": LevelJunior, "iv": LevelSenior}

// "PL/SQL" não é pleno.
var seniorityNoise = strings.NewReplacer("pl/sql", "plsql", "pl-sql", "plsql", "pl sql", "plsql")

// ClassifySeniority acha os níveis citados num texto curto: título da vaga ou campo de
// nível da fonte. A descrição não entra: "pleno domínio de SQL" e "time de sêniores"
// dariam falso positivo.
func ClassifySeniority(texts ...string) []string {
	found := map[string]bool{}
	for _, t := range texts {
		t = seniorityNoise.Replace(foldAccents(strings.ToLower(t)))
		for _, m := range seniorityMarkers {
			if !found[m.level] && hasAny(t, m.markers) {
				found[m.level] = true
			}
		}
		for _, m := range romanLevel.FindAllStringSubmatch(t, -1) {
			found[romanLevels[m[1]]] = true
		}
	}
	out := []string{}
	for _, l := range SeniorityLevels {
		if found[l] {
			out = append(out, l)
		}
	}
	return out
}

// MergeSeniority junta níveis de origens diferentes, na ordem de SeniorityLevels.
func MergeSeniority(lists ...[]string) []string {
	out := []string{}
	for _, l := range SeniorityLevels {
		for _, list := range lists {
			if slices.Contains(list, l) {
				out = append(out, l)
				break
			}
		}
	}
	return out
}

// SeniorityPrefs é o filtro de nível do usuário. Sem nível marcado, não filtra.
type SeniorityPrefs struct {
	Levels []string
	// IncludeUnknown deixa passar as vagas sem nível informado (a maioria).
	IncludeUnknown bool
}

func (p SeniorityPrefs) Matches(levels []string) bool {
	if len(p.Levels) == 0 {
		return true
	}
	if len(levels) == 0 {
		return p.IncludeUnknown
	}
	for _, l := range levels {
		if slices.Contains(p.Levels, l) {
			return true
		}
	}
	return false
}
