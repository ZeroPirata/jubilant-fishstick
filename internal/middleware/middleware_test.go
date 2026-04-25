package middleware

import (
	"context"
	"errors"
	"hackton-treino/internal/security"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

// =============================================================================
// AuthMiddleware
// =============================================================================

// tokenProviderStub simula um TokenProvider sem depender do JWT real.
// Cada teste controla o que Validate devolve, isolando o middleware da criptografia.
type tokenProviderStub struct {
	returnClaims security.ValidatedClaims
	returnErr    error
}

func (s *tokenProviderStub) Generate(userID string) (string, error) { return "", nil }
func (s *tokenProviderStub) Validate(token string) (security.ValidatedClaims, error) {
	return s.returnClaims, s.returnErr
}

// okHandler é um handler sentinela que sempre responde 200.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestAuthMiddleware_SemHeader(t *testing.T) {
	mw := AuthMiddleware(&tokenProviderStub{returnClaims: security.ValidatedClaims{UserID: "qualquer"}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// sem header Authorization

	mw(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAuthMiddleware_FormatoInvalido(t *testing.T) {
	mw := AuthMiddleware(&tokenProviderStub{returnClaims: security.ValidatedClaims{UserID: "qualquer"}})

	cases := []struct {
		label  string
		header string
	}{
		{"sem prefixo Bearer", "somente-o-token"},
		{"prefixo errado", "Basic token123"},
		{"tres partes", "Bearer token extra"},
		{"bearer minusculo", "bearer token"},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tc.header)

			mw(okHandler).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
		})
	}
}

func TestAuthMiddleware_TokenInvalido(t *testing.T) {
	// provider retorna erro → middleware deve barrar com 401
	mw := AuthMiddleware(&tokenProviderStub{returnErr: errors.New("token expirado")})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer qualquer-token")

	mw(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAuthMiddleware_TokenValido_PassaContexto(t *testing.T) {
	// Com token válido, o handler downstream deve receber o userID no context.
	const expectedUserID = "550e8400-e29b-41d4-a716-446655440000"
	mw := AuthMiddleware(&tokenProviderStub{returnClaims: security.ValidatedClaims{UserID: expectedUserID}})

	var capturedCtx context.Context
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token-valido")

	mw(downstream).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	// GetUserID deve extrair o UUID do context sem erro
	_, ok := GetUserID(capturedCtx)
	if !ok {
		t.Error("GetUserID retornou ok=false; userID deveria estar no context")
	}
}

func TestAuthMiddleware_TokenValido_NaoBloqueia(t *testing.T) {
	// Garantia de que a requisição passa — o handler downstream é chamado
	mw := AuthMiddleware(&tokenProviderStub{returnClaims: security.ValidatedClaims{UserID: "550e8400-e29b-41d4-a716-446655440000"}})

	called := false
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token-valido")

	mw(downstream).ServeHTTP(rec, req)

	if !called {
		t.Error("handler downstream não foi chamado com token válido")
	}
}

// =============================================================================
// MiddlewarePanicRecovery
// =============================================================================

func TestPanicRecovery_HandlerNormal(t *testing.T) {
	// Sem panic → comportamento transparente, status do handler é preservado
	mw := MiddlewarePanicRecovery(zap.NewNop())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
}

func TestPanicRecovery_HandlerPanica(t *testing.T) {
	// Um handler que entra em panic não deve derrubar o servidor.
	// O middleware deve capturar e devolver 500.
	mw := MiddlewarePanicRecovery(zap.NewNop())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("erro catastrófico simulado")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestPanicRecovery_PanicComErro(t *testing.T) {
	// Garante que panic com um error (não só string) também é recuperado
	mw := MiddlewarePanicRecovery(zap.NewNop())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(errors.New("erro tipado"))
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestPanicRecovery_MultiplasRequisicoes(t *testing.T) {
	// Após um panic recuperado, o middleware deve continuar funcionando
	// para requisições subsequentes — verifica que não há estado corrompido.
	mw := MiddlewarePanicRecovery(zap.NewNop())

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("falha")
	})
	normalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// primeira: panic → 500
	rec1 := httptest.NewRecorder()
	mw(panicHandler).ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec1.Code != http.StatusInternalServerError {
		t.Errorf("após panic: status = %d, want 500", rec1.Code)
	}

	// segunda: normal → 200
	rec2 := httptest.NewRecorder()
	mw(normalHandler).ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec2.Code != http.StatusOK {
		t.Errorf("após recovery: status = %d, want 200", rec2.Code)
	}
}

// =============================================================================
// TimeoutMiddleware
// =============================================================================

func TestTimeoutMiddleware_PassaDeadlineNoContexto(t *testing.T) {
	// O middleware deve adicionar um deadline ao context do request.
	var temDeadline bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, temDeadline = r.Context().Deadline()
		w.WriteHeader(http.StatusOK)
	})

	mw := TimeoutMiddleware(5 * time.Second)(handler)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !temDeadline {
		t.Error("context deveria ter deadline após TimeoutMiddleware")
	}
}

func TestTimeoutMiddleware_CancelaContextoAposTimeout(t *testing.T) {
	// Um handler que aguarda mais que o timeout deve receber ctx.Done().
	// Isso verifica que o context é cancelado com DeadlineExceeded.
	var ctxErr error
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			ctxErr = r.Context().Err()
			w.WriteHeader(http.StatusServiceUnavailable)
		case <-time.After(200 * time.Millisecond):
			// handler mais lento que o timeout — não deveria chegar aqui
			w.WriteHeader(http.StatusOK)
		}
	})

	// timeout menor que o sleep do handler
	mw := TimeoutMiddleware(10 * time.Millisecond)(handler)
	mw.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Errorf("esperava DeadlineExceeded, recebeu: %v", ctxErr)
	}
}

func TestTimeoutMiddleware_HandlerRapidoNaoECancelado(t *testing.T) {
	// Handler que termina antes do timeout não deve ter o context cancelado.
	var ctxErr error
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxErr = r.Context().Err() // deve ser nil
		w.WriteHeader(http.StatusOK)
	})

	mw := TimeoutMiddleware(5 * time.Second)(handler)
	mw.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if ctxErr != nil {
		t.Errorf("context não deveria estar cancelado: %v", ctxErr)
	}
}
