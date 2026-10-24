package worker

import "testing"

func TestFillCoverLetter_VariacoesDoPlaceholderDoNome(t *testing.T) {
	cases := map[string]string{
		"Atenciosamente,\n{{CANDIDATO_NOME}}":     "Atenciosamente,\nMaria Silva",
		"Atenciosamente,\n[CANDIDATO_NOME]":       "Atenciosamente,\nMaria Silva",
		"Atenciosamente,\n{ CANDIDATO_NOME }":     "Atenciosamente,\nMaria Silva",
		"Best regards,\n[Your Name]":              "Best regards,\nMaria Silva",
		"Atenciosamente,\n[Seu nome]":             "Atenciosamente,\nMaria Silva",
		"Texto [TELEFONE] fim.\n[CANDIDATO_NOME]": "Texto  fim.\nMaria Silva",
		"Uso [React] e [Go] no dia a dia.":        "Uso [React] e [Go] no dia a dia.",
	}
	for in, want := range cases {
		if got := fillCoverLetter(in, "Maria Silva"); got != want {
			t.Errorf("fillCoverLetter(%q) = %q, want %q", in, got, want)
		}
	}
	if got := fillCoverLetter("Atenciosamente,\n[CANDIDATO_NOME]", ""); got != "Atenciosamente," {
		t.Errorf("sem nome: %q", got)
	}
}
