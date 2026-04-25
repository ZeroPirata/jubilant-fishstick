package middleware

import (
	"context"
	"hackton-treino/internal/security"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// revokerStub implements security.TokenRevoker without Redis.
type revokerStub struct {
	revoked map[string]bool
}

func newRevokerStub() *revokerStub {
	return &revokerStub{revoked: make(map[string]bool)}
}

func (r *revokerStub) Revoke(_ context.Context, jti string, _ time.Time) {
	r.revoked[jti] = true
}

func (r *revokerStub) IsRevoked(_ context.Context, jti string) bool {
	return r.revoked[jti]
}

var revocationOKHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestRevocationMiddleware_SemClaimsNoContexto(t *testing.T) {
	// No claims in context (unauthenticated request): middleware must let it through.
	// AuthMiddleware already blocks missing tokens; this layer only acts after auth.
	stub := newRevokerStub()
	mw := RevocationMiddleware(stub)(revocationOKHandler)

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("sem claims: status = %d, want 200", rec.Code)
	}
}

func TestRevocationMiddleware_TokenNaoRevogado(t *testing.T) {
	// Valid, non-revoked token: request passes through.
	stub := newRevokerStub()
	mw := RevocationMiddleware(stub)(revocationOKHandler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ContextKeyUserID, security.ValidatedClaims{
		UserID: "user-1",
		JTI:    "jti-valido",
		Exp:    time.Now().Add(time.Hour),
	}))

	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("token válido: status = %d, want 200", rec.Code)
	}
}

func TestRevocationMiddleware_TokenRevogado(t *testing.T) {
	// JTI in the blacklist: must return 401.
	stub := newRevokerStub()
	stub.revoked["jti-revogado"] = true

	mw := RevocationMiddleware(stub)(revocationOKHandler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ContextKeyUserID, security.ValidatedClaims{
		UserID: "user-1",
		JTI:    "jti-revogado",
		Exp:    time.Now().Add(time.Hour),
	}))

	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("token revogado: status = %d, want 401", rec.Code)
	}
}

func TestRevocationMiddleware_NaoChaamaDownstreamQuandoRevogado(t *testing.T) {
	// With a revoked JTI the downstream handler must not be called.
	stub := newRevokerStub()
	stub.revoked["jti-revogado"] = true

	chamou := false
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamou = true
		w.WriteHeader(http.StatusOK)
	})

	mw := RevocationMiddleware(stub)(downstream)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ContextKeyUserID, security.ValidatedClaims{
		JTI: "jti-revogado",
		Exp: time.Now().Add(time.Hour),
	}))

	mw.ServeHTTP(rec, req)

	if chamou {
		t.Error("downstream não deve ser chamado quando o token é revogado")
	}
}

func TestRevocationMiddleware_TokenDiferenteNaoAffeta(t *testing.T) {
	// Only the exact JTI is blocked; a different JTI on the same user still passes.
	stub := newRevokerStub()
	stub.revoked["jti-antigo"] = true

	mw := RevocationMiddleware(stub)(revocationOKHandler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ContextKeyUserID, security.ValidatedClaims{
		UserID: "user-1",
		JTI:    "jti-novo",
		Exp:    time.Now().Add(time.Hour),
	}))

	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("JTI diferente do revogado: status = %d, want 200", rec.Code)
	}
}
