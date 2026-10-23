package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

var (
	ErrSecretBoxDisabled = errors.New("SETTINGS_ENCRYPTION_KEY não configurada")
	ErrSecretTooShort    = errors.New("secretbox: ciphertext muito curto")
)

// SecretBox criptografa segredos (ex.: API keys) para guardar no banco com AES-256-GCM.
// O formato salvo é nonce || ciphertext.
type SecretBox struct {
	aead cipher.AEAD
}

// NewSecretBox recebe a chave em base64 (32 bytes). Chave vazia retorna (nil, nil):
// a aplicação sobe normalmente, só não consegue salvar/ler segredos do banco.
func NewSecretBox(b64Key string) (*SecretBox, error) {
	if b64Key == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(b64Key)
	if err != nil {
		return nil, fmt.Errorf("secretbox: chave não é base64 válido: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("secretbox: chave precisa ter 32 bytes, tem %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretBox{aead: aead}, nil
}

func (b *SecretBox) Seal(plaintext string) ([]byte, error) {
	if b == nil {
		return nil, ErrSecretBoxDisabled
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (b *SecretBox) Open(sealed []byte) (string, error) {
	if b == nil {
		return "", ErrSecretBoxDisabled
	}
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return "", ErrSecretTooShort
	}
	plain, err := b.aead.Open(nil, sealed[:n], sealed[n:], nil)
	if err != nil {
		return "", fmt.Errorf("secretbox: falha ao descriptografar: %w", err)
	}
	return string(plain), nil
}
