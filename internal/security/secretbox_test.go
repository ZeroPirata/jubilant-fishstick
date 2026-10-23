package security

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

var testKey = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))

func TestSecretBox_SealOpen(t *testing.T) {
	box, err := NewSecretBox(testKey)
	if err != nil {
		t.Fatalf("NewSecretBox: %v", err)
	}

	sealed, err := box.Seal("sk-minha-key")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if strings.Contains(string(sealed), "sk-minha-key") {
		t.Fatal("ciphertext contém o texto puro")
	}

	got, err := box.Open(sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got != "sk-minha-key" {
		t.Errorf("Open = %q, want %q", got, "sk-minha-key")
	}
}

func TestSecretBox_NonceAleatorio(t *testing.T) {
	box, _ := NewSecretBox(testKey)
	a, _ := box.Seal("mesmo")
	b, _ := box.Seal("mesmo")
	if string(a) == string(b) {
		t.Error("dois Seal do mesmo texto geraram o mesmo ciphertext")
	}
}

func TestSecretBox_ChaveErradaFalha(t *testing.T) {
	box, _ := NewSecretBox(testKey)
	sealed, _ := box.Seal("segredo")

	other, _ := NewSecretBox(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 32))))
	if _, err := other.Open(sealed); err == nil {
		t.Error("Open com outra chave deveria falhar")
	}
}

func TestSecretBox_ChaveInvalida(t *testing.T) {
	cases := []struct {
		label string
		key   string
	}{
		{"nao base64", "!!!"},
		{"tamanho errado", base64.StdEncoding.EncodeToString([]byte("curta"))},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if _, err := NewSecretBox(tc.key); err == nil {
				t.Errorf("NewSecretBox(%q) deveria falhar", tc.key)
			}
		})
	}
}

func TestSecretBox_Desabilitado(t *testing.T) {
	box, err := NewSecretBox("")
	if err != nil || box != nil {
		t.Fatalf("chave vazia deveria retornar (nil, nil), recebeu (%v, %v)", box, err)
	}
	if _, err := box.Seal("x"); !errors.Is(err, ErrSecretBoxDisabled) {
		t.Errorf("Seal em box nil: erro = %v, want ErrSecretBoxDisabled", err)
	}
	if _, err := box.Open([]byte("x")); !errors.Is(err, ErrSecretBoxDisabled) {
		t.Errorf("Open em box nil: erro = %v, want ErrSecretBoxDisabled", err)
	}
}
