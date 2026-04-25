package security

import (
	"strings"
	"sync"
	"testing"
	"time"

	"hackton-treino/config"
)

// --- helpers ---

// fastHashConfig usa params mínimos para que os testes não demorem 1s cada.
// Em produção os valores reais ficam no .env; aqui só testamos o comportamento.
func fastHashConfig() config.HashConfig {
	return config.HashConfig{
		Argon2Memory:      4 * 1024, // 4 MB (produção usa 32–64 MB)
		Argon2Iterations:  1,
		Argon2Parallelism: 1,
		Argon2SaltLen:     16,
		Argon2KeyLen:      32,
		Argon2Pepper:      "test-pepper",
	}
}

// =============================================================================
// JWT
// =============================================================================

func TestJwt_RoundTrip(t *testing.T) {
	// Generate → Validate deve devolver o mesmo userID
	m := NewJwtManager("super-secret", time.Hour)
	userID := "550e8400-e29b-41d4-a716-446655440000"

	token, err := m.Generate(userID)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	got, err := m.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got.UserID != userID {
		t.Errorf("userID = %q, want %q", got.UserID, userID)
	}
}

func TestJwt_SecretoErrado(t *testing.T) {
	// Token gerado com secreto A não pode ser validado com secreto B
	gerador := NewJwtManager("segredo-A", time.Hour)
	validador := NewJwtManager("segredo-B", time.Hour)

	token, err := gerador.Generate("qualquer-id")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	_, err = validador.Validate(token)
	if err == nil {
		t.Fatal("esperava erro ao validar com segredo diferente, mas não recebeu")
	}
}

func TestJwt_TokenExpirado(t *testing.T) {
	// Expiration negativa → token já nasce expirado
	m := NewJwtManager("segredo", -time.Second)

	token, err := m.Generate("qualquer-id")
	if err != nil {
		t.Fatalf("Generate com expiration negativa: %v", err)
	}

	_, err = m.Validate(token)
	if err == nil {
		t.Fatal("esperava erro para token expirado")
	}
}

func TestJwt_TokenMalformado(t *testing.T) {
	m := NewJwtManager("segredo", time.Hour)

	cases := []struct {
		label string
		token string
	}{
		{"string vazia", ""},
		{"lixo aleatorio", "nao.sou.um.jwt"},
		{"so header", "eyJhbGciOiJIUzI1NiJ9"},
		{"ponto extra", "a.b.c.d"},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			_, err := m.Validate(tc.token)
			if err == nil {
				t.Errorf("Validate(%q): esperava erro, recebeu nil", tc.token)
			}
		})
	}
}

func TestJwt_TokensDistintosPorChamada(t *testing.T) {
	// Cada Generate deve produzir um token diferente (JTI é UUID aleatório)
	m := NewJwtManager("segredo", time.Hour)
	userID := "550e8400-e29b-41d4-a716-446655440000"

	t1, _ := m.Generate(userID)
	t2, _ := m.Generate(userID)

	if t1 == t2 {
		t.Error("dois tokens gerados para o mesmo userID não devem ser iguais")
	}
}

// =============================================================================
// Argon2id
// =============================================================================

func TestArgon2_RoundTrip(t *testing.T) {
	h := NewHasher(fastHashConfig())

	encoded, err := h.Hash("minha-senha-secreta")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	ok, err := h.Verify("minha-senha-secreta", encoded)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Error("Verify deveria retornar true para senha correta")
	}
}

func TestArgon2_SenhaErrada(t *testing.T) {
	h := NewHasher(fastHashConfig())

	encoded, _ := h.Hash("senha-correta")

	ok, err := h.Verify("senha-errada", encoded)
	if err != nil {
		t.Fatalf("Verify: erro inesperado: %v", err)
	}
	if ok {
		t.Error("Verify deveria retornar false para senha errada")
	}
}

func TestArgon2_HashesDistintosMesmaSenha(t *testing.T) {
	// Salt aleatório garante que dois hashes da mesma senha sejam diferentes.
	// Isso é fundamental: se dois usuários tiverem a mesma senha, os hashes
	// no banco devem ser distintos para não vazar informação.
	h := NewHasher(fastHashConfig())

	h1, _ := h.Hash("mesma-senha")
	h2, _ := h.Hash("mesma-senha")

	if h1 == h2 {
		t.Error("dois hashes da mesma senha devem ser diferentes (salt aleatório)")
	}

	// mas ambos devem verificar corretamente
	ok1, _ := h.Verify("mesma-senha", h1)
	ok2, _ := h.Verify("mesma-senha", h2)
	if !ok1 || !ok2 {
		t.Error("ambos os hashes devem verificar a senha corretamente")
	}
}

func TestArgon2_HashFormatoEsperado(t *testing.T) {
	// O hash codificado deve seguir o formato argon2id padrão.
	// Isso garante compatibilidade com bibliotecas externas e que o Verify
	// consiga parsear o resultado do Hash sem erros.
	h := NewHasher(fastHashConfig())
	encoded, _ := h.Hash("senha")

	if !strings.HasPrefix(encoded, "$argon2id$") {
		t.Errorf("hash não começa com $argon2id$: %q", encoded)
	}
	parts := strings.Split(encoded, "$")
	// formato: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<hash>"]
	if len(parts) != 6 {
		t.Errorf("hash tem %d partes separadas por $, esperava 6: %q", len(parts), encoded)
	}
}

func TestArgon2_HashTamperedRetornaErro(t *testing.T) {
	h := NewHasher(fastHashConfig())
	encoded, _ := h.Hash("senha")

	cases := []struct {
		label  string
		hash   string
		wantOk bool
		wantErr bool
	}{
		{
			label:   "hash com partes faltando",
			hash:    "$argon2id$v=19",
			wantErr: true,
		},
		{
			label:   "hash completamente invalido",
			hash:    "nao-sou-um-hash",
			wantErr: true,
		},
		{
			label:   "hash correto → ok sem erro",
			hash:    encoded,
			wantOk:  true,
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			ok, err := h.Verify("senha", tc.hash)
			if tc.wantErr && err == nil {
				t.Error("esperava erro, recebeu nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("erro inesperado: %v", err)
			}
			if ok != tc.wantOk {
				t.Errorf("Verify() = %v, want %v", ok, tc.wantOk)
			}
		})
	}
}

func TestArgon2_PepperIsolado(t *testing.T) {
	// Hash feito com pepper A não pode ser verificado com pepper B.
	// O pepper é o segundo fator além do salt — protege contra vazamento do banco.
	cfgA := fastHashConfig()
	cfgA.Argon2Pepper = "pepper-A"

	cfgB := fastHashConfig()
	cfgB.Argon2Pepper = "pepper-B"

	hA := NewHasher(cfgA)
	hB := NewHasher(cfgB)

	encoded, _ := hA.Hash("senha")

	ok, err := hB.Verify("senha", encoded)
	if err != nil {
		t.Fatalf("Verify com pepper diferente: erro inesperado: %v", err)
	}
	if ok {
		t.Error("Verify com pepper errado deveria retornar false")
	}
}

func TestArgon2_ConcorrenciaSemaphore(t *testing.T) {
	// O semaphore interno limita chamadas paralelas ao argon2 (CPU-bound).
	// Este teste garante que N goroutines simultâneas todas completam sem deadlock.
	cfg := fastHashConfig()
	cfg.Argon2Parallelism = 2 // semaphore de tamanho 2
	h := NewHasher(cfg)

	const goroutines = 10
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			encoded, err := h.Hash("senha")
			if err != nil {
				errs <- err
				return
			}
			ok, err := h.Verify("senha", encoded)
			if err != nil {
				errs <- err
				return
			}
			if !ok {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("goroutine falhou: %v", err)
		}
	}
}
