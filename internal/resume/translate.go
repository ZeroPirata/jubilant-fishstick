package resume

import (
	"regexp"
	"strings"
	"unicode"
)

// Currículo em inglês com perfil em português: o cargo e o nome das skills vêm do
// banco, então a IA devolve a tradução e o código decide se ela entra. A regra é a
// mesma do resto do currículo: traduzir pode, acrescentar não. Na dúvida fica o
// original, que é verdadeiro, só que em português.

// Grupos de senioridade e de função citados no cargo, em português e em inglês. Uma
// tradução não pode citar grupo que o original não cita ("Pleno" → "Senior").
var roleMarkers = map[string][]string{
	"intern":     {"estagio", "estagiario", "estagiaria", "intern", "internship", "trainee", "aprendiz", "apprentice"},
	"junior":     {"junior", "jr", "entry"},
	"mid":        {"pleno", "plena", "mid"},
	"senior":     {"senior", "sr"},
	"lead":       {"lead", "leader", "lider", "lideranca"},
	"staff":      {"staff"},
	"principal":  {"principal"},
	"architect":  {"arquiteto", "arquiteta", "architect"},
	"specialist": {"especialista", "specialist", "expert"},
	"management": {"gerente", "manager", "management", "head", "coordenador", "coordenadora", "coordinator", "supervisor", "supervisora", "diretor", "diretora", "director", "chief", "cto", "vp", "presidente", "president"},
}

// Níveis em algarismo romano ("Desenvolvedor II") e números contam como nível também.
var reRoleNumber = regexp.MustCompile(`^(i{1,3}|iv|v|vi{0,3}|\d+)$`)

func words(s string) []string {
	return strings.FieldsFunc(unaccent.Replace(strings.ToLower(s)), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// roleGroups devolve os grupos de roleMarkers e os números de nível citados no cargo.
func roleGroups(role string) map[string]bool {
	found := map[string]bool{}
	for _, w := range words(role) {
		for group, markers := range roleMarkers {
			for _, m := range markers {
				if w == m {
					found[group] = true
				}
			}
		}
		// "i" sozinho não conta: em inglês é pronome e "Engenheiro I" é raro; o nível
		// começa a contar em "ii".
		if w != "i" && reRoleNumber.MatchString(w) {
			found["n:"+w] = true
		}
	}
	return found
}

// roleTranslationOK: a tradução não cita nível, liderança ou gestão que o original não
// cita, e não apaga estágio ou júnior (seria esconder o nível).
func roleTranslationOK(orig, tr string) bool {
	tr = strings.TrimSpace(tr)
	if tr == "" || len(tr) > 80 || strings.ContainsAny(tr, "\n\r") {
		return false
	}
	o, t := roleGroups(orig), roleGroups(tr)
	for g := range t {
		if !o[g] {
			return false
		}
	}
	for _, keep := range []string{"intern", "junior"} {
		if o[keep] && !t[keep] {
			return false
		}
	}
	return true
}

// translatedRole escolhe o cargo do currículo: a tradução da IA no currículo em inglês,
// quando ela passa em roleTranslationOK; senão o cargo do perfil. warn explica a recusa.
func (s Source) translatedRole(orig, tr string) (role, warn string) {
	tr = strings.TrimSpace(tr)
	if s.Lang != "en" || tr == "" || strings.EqualFold(tr, orig) {
		return orig, ""
	}
	if !roleTranslationOK(orig, tr) {
		return orig, "tradução de cargo recusada: " + orig + " → " + tr
	}
	return tr, ""
}

// Siglas que a tradução pode trazer sem estar no original: são a tradução de siglas
// em português (BD → DB) ou o próprio termo em inglês.
var allowedNewAcronyms = map[string]bool{"DB": true, "API": true, "APIS": true, "UI": true, "UX": true, "IT": true}

var reAcronym = regexp.MustCompile(`\b[A-Z][A-Z0-9]+\b`)
var reDigits = regexp.MustCompile(`\d+`)

// skillTranslationOK: a tradução de uma skill é só o nome em inglês. Não pode trazer
// sigla, número ou parêntese que o original não tem ("Mensageria" → "Messaging (Kafka)"),
// nem crescer demais.
func skillTranslationOK(orig, tr string) bool {
	if tr == "" || len(tr) > 60 || strings.ContainsAny(tr, "\n\r,;/") && !strings.ContainsAny(orig, ",;/") {
		return false
	}
	if len(words(tr)) > len(words(orig))+2 {
		return false
	}
	if strings.Contains(tr, "(") && !strings.Contains(orig, "(") {
		return false
	}
	origUpper := map[string]bool{}
	for _, a := range reAcronym.FindAllString(orig, -1) {
		origUpper[a] = true
	}
	for _, a := range reAcronym.FindAllString(tr, -1) {
		if !origUpper[a] && !allowedNewAcronyms[a] {
			return false
		}
	}
	origDigits := strings.Join(reDigits.FindAllString(orig, -1), " ")
	return strings.Join(reDigits.FindAllString(tr, -1), " ") == origDigits
}

// skillTranslations guarda as traduções aceitas, indexadas pelo nome do perfil (sem
// diferenciar maiúsculas), e o caminho inverso para quando a IA já lista a tradução.
type skillTranslations struct {
	toEN     map[string]string
	fromEN   map[string]string
	rejected []string
}

func (s Source) newSkillTranslations(raw map[string]string) skillTranslations {
	t := skillTranslations{toEN: map[string]string{}, fromEN: map[string]string{}}
	if s.Lang != "en" {
		return t
	}
	for orig, tr := range raw {
		orig, tr = strings.TrimSpace(orig), strings.TrimSpace(tr)
		if orig == "" || tr == "" || strings.EqualFold(orig, tr) {
			continue
		}
		if !skillTranslationOK(orig, tr) {
			t.rejected = append(t.rejected, "tradução de habilidade recusada: "+orig+" → "+tr)
			continue
		}
		t.toEN[strings.ToLower(orig)] = tr
		t.fromEN[strings.ToLower(tr)] = orig
	}
	return t
}

// original devolve o nome do perfil quando item é uma tradução aceita.
func (t skillTranslations) original(item string) string {
	if o, ok := t.fromEN[strings.ToLower(item)]; ok {
		return o
	}
	return item
}

// display devolve o nome a mostrar: a tradução, quando houver.
func (t skillTranslations) display(item string) string {
	if tr, ok := t.toEN[strings.ToLower(item)]; ok {
		return tr
	}
	return item
}
