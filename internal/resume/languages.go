package resume

import "strings"

// Idiomas do perfil. Os conhecidos são guardados pelo código (en, es, ...) para o
// currículo mostrar o nome no idioma do documento; os demais ficam como digitados.
type langNames struct{ pt, en string }

var knownLanguages = map[string]langNames{
	"pt":     {"Português", "Portuguese"},
	"en":     {"Inglês", "English"},
	"es":     {"Espanhol", "Spanish"},
	"fr":     {"Francês", "French"},
	"de":     {"Alemão", "German"},
	"it":     {"Italiano", "Italian"},
	"ja":     {"Japonês", "Japanese"},
	"zh":     {"Mandarim", "Mandarin Chinese"},
	"ko":     {"Coreano", "Korean"},
	"ru":     {"Russo", "Russian"},
	"ar":     {"Árabe", "Arabic"},
	"nl":     {"Holandês", "Dutch"},
	"hi":     {"Hindi", "Hindi"},
	"libras": {"Libras", "Brazilian Sign Language (Libras)"},
}

// Níveis padrão, do menor para o maior.
var LanguageLevels = []string{"basic", "intermediate", "advanced", "fluent", "native"}

var levelNames = map[string]langNames{
	"basic":        {"Básico", "Basic"},
	"intermediate": {"Intermediário", "Intermediate"},
	"advanced":     {"Avançado", "Advanced"},
	"fluent":       {"Fluente", "Fluent"},
	"native":       {"Nativo", "Native"},
}

// Variações de nome (sem acento, minúsculas) que viram o código.
var languageAliases = map[string]string{
	"portugues": "pt", "portuguese": "pt", "portugues brasileiro": "pt", "brazilian portuguese": "pt",
	"ingles": "en", "english": "en", "espanhol": "es", "spanish": "es", "castelhano": "es", "espanol": "es",
	"frances": "fr", "french": "fr", "alemao": "de", "german": "de", "italiano": "it", "italian": "it",
	"japones": "ja", "japanese": "ja", "mandarim": "zh", "chines": "zh", "chinese": "zh", "mandarin": "zh",
	"mandarin chinese": "zh", "coreano": "ko", "korean": "ko", "russo": "ru", "russian": "ru",
	"arabe": "ar", "arabic": "ar", "holandes": "nl", "dutch": "nl", "neerlandes": "nl", "hindi": "hi",
	"libras": "libras", "lingua brasileira de sinais": "libras", "brazilian sign language": "libras",
}

var unaccent = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i",
	"ó", "o", "ô", "o", "õ", "o", "ú", "u", "ü", "u", "ç", "c", "ñ", "n")

func foldName(s string) string {
	return strings.Join(strings.Fields(unaccent.Replace(strings.ToLower(s))), " ")
}

// LanguageCode devolve o código de um idioma conhecido (por código ou nome, em pt ou en)
// ou o próprio texto, limpo, quando não é conhecido.
func LanguageCode(s string) string {
	f := foldName(s)
	if _, ok := knownLanguages[f]; ok {
		return f
	}
	if code, ok := languageAliases[f]; ok {
		return code
	}
	return strings.Join(strings.Fields(s), " ")
}

// LanguageName mostra o idioma no idioma do documento ("en" → "Inglês" ou "English").
func LanguageName(code, lang string) string {
	n, ok := knownLanguages[code]
	if !ok {
		return code
	}
	if lang == "en" {
		return n.en
	}
	return n.pt
}

func ValidLanguageLevel(level string) bool {
	_, ok := levelNames[level]
	return ok
}

func LanguageLevelName(level, lang string) string {
	n, ok := levelNames[level]
	if !ok {
		return level
	}
	if lang == "en" {
		return n.en
	}
	return n.pt
}
