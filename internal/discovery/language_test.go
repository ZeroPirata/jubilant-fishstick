package discovery

import "testing"

func TestGuessLanguage(t *testing.T) {
	cases := []struct{ text, want string }{
		{"Senior Go Engineer\nWe are looking for an engineer to join our team. You will work with Go and Kubernetes.", "en"},
		{"Desenvolvedor Go Sênior\nBuscamos uma pessoa para o time de pagamentos, com experiência em Go e Kubernetes.", "pt-br"},
		// Vaga brasileira com stack em inglês continua em português.
		{"Pessoa Desenvolvedora Backend\nRequisitos: experiência com Go, Docker, CI/CD e testes de integração na nuvem.", "pt-br"},
		{"Go Developer", "pt-br"}, // curto demais para saber
		{"", "pt-br"},
	}
	for _, c := range cases {
		if got := GuessLanguage(c.text); got != c.want {
			t.Errorf("GuessLanguage(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}
