package resume

import "testing"

func TestLanguageCode_NomesConhecidosViramCodigo(t *testing.T) {
	cases := map[string]string{
		"Inglês": "en", "english": "en", "  ESPANHOL ": "es", "en": "en", "Mandarim": "zh",
		"Língua Brasileira de Sinais": "libras", "Português": "pt", "Esperanto": "Esperanto", "  Tupi  guarani ": "Tupi guarani",
	}
	for in, want := range cases {
		if got := LanguageCode(in); got != want {
			t.Errorf("LanguageCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLanguageName_NoIdiomaDoDocumento(t *testing.T) {
	if got := LanguageName("en", "pt-br"); got != "Inglês" {
		t.Errorf("pt-br: %q", got)
	}
	if got := LanguageName("en", "en"); got != "English" {
		t.Errorf("en: %q", got)
	}
	if got := LanguageName("Esperanto", "en"); got != "Esperanto" {
		t.Errorf("desconhecido: %q", got)
	}
	if got := LanguageLevelName("fluent", "pt-br"); got != "Fluente" {
		t.Errorf("nível: %q", got)
	}
}
