package handler

import (
	"context"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/security"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

// logoutRevokerStub implements security.TokenRevoker for LogoutHandler tests.
type logoutRevokerStub struct {
	revoked map[string]bool
}

func newLogoutRevokerStub() *logoutRevokerStub {
	return &logoutRevokerStub{revoked: make(map[string]bool)}
}

func (r *logoutRevokerStub) Revoke(_ context.Context, jti string, _ time.Time) {
	r.revoked[jti] = true
}

func (r *logoutRevokerStub) IsRevoked(_ context.Context, jti string) bool {
	return r.revoked[jti]
}

func TestLogout_SemClaims(t *testing.T) {
	// No claims in context → 204 without calling Revoke.
	stub := newLogoutRevokerStub()
	h := NewLogoutHandler(zap.NewNop(), stub)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)

	h.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if len(stub.revoked) != 0 {
		t.Error("Revoke não deve ser chamado sem claims no context")
	}
}

func TestLogout_ComClaims_RevogaJTI(t *testing.T) {
	// Valid claims → 204 and the JTI is added to the blacklist.
	const jti = "token-jti-abc123"
	stub := newLogoutRevokerStub()
	h := NewLogoutHandler(zap.NewNop(), stub)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, security.ValidatedClaims{
		UserID: "550e8400-e29b-41d4-a716-446655440000",
		JTI:    jti,
		Exp:    time.Now().Add(time.Hour),
	}))

	h.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if !stub.revoked[jti] {
		t.Errorf("JTI %q deveria ter sido revogado após logout", jti)
	}
}

func TestLogout_NaoRevogaOutrosTokens(t *testing.T) {
	// Logout revokes only the current token's JTI, not others.
	stub := newLogoutRevokerStub()
	stub.revoked["outro-jti"] = true
	h := NewLogoutHandler(zap.NewNop(), stub)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, security.ValidatedClaims{
		UserID: "user-1",
		JTI:    "meu-jti",
		Exp:    time.Now().Add(time.Hour),
	}))

	h.Logout(rec, req)

	if !stub.revoked["outro-jti"] {
		t.Error("logout não deve afetar outros JTIs")
	}
	if !stub.revoked["meu-jti"] {
		t.Error("o JTI atual deve ter sido revogado")
	}
}
