package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler responde 200 — usado para verificar se o request chegou ao downstream
var corsOKHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestCORSMiddleware_HeadersPresentes(t *testing.T) {
	// Todos os requests devem receber os headers de CORS, independente do método
	mw := CORSMiddleware("*")(corsOKHandler)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	mw.ServeHTTP(rec, req)

	cases := []struct{ header, want string }{
		{"Access-Control-Allow-Origin", "*"},
		{"Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS"},
		{"Access-Control-Allow-Headers", "Content-Type, Authorization"},
	}
	for _, tc := range cases {
		if got := rec.Header().Get(tc.header); got != tc.want {
			t.Errorf("%s = %q, want %q", tc.header, got, tc.want)
		}
	}
}

func TestCORSMiddleware_PreflightRetorna204(t *testing.T) {
	// OPTIONS (preflight) deve retornar 204 sem chamar o handler downstream.
	// Browsers enviam preflight antes de requests com Authorization header.
	chamou := false
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamou = true
		w.WriteHeader(http.StatusOK)
	})

	mw := CORSMiddleware("*")(downstream)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/", nil)

	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", rec.Code)
	}
	if chamou {
		t.Error("handler downstream não deve ser chamado no preflight OPTIONS")
	}
}

func TestCORSMiddleware_GetPassaDownstream(t *testing.T) {
	// Requests normais (não OPTIONS) devem chegar ao handler downstream
	chamou := false
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamou = true
		w.WriteHeader(http.StatusOK)
	})

	mw := CORSMiddleware("*")(downstream)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)

	mw.ServeHTTP(rec, req)

	if !chamou {
		t.Error("handler downstream deveria ser chamado para GET")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestCORSMiddleware_PostPassaDownstream(t *testing.T) {
	mw := CORSMiddleware("*")(corsOKHandler)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", nil)

	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("POST status = %d, want 200", rec.Code)
	}
}

func TestCORSMiddleware_PreflightTemHeadersCorretos(t *testing.T) {
	// O preflight OPTIONS ainda precisa dos headers de CORS na resposta,
	// senão o browser recusa a requisição real que vem depois
	mw := CORSMiddleware("*")(corsOKHandler)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/", nil)

	mw.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("preflight Access-Control-Allow-Origin = %q, want \"*\"", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("preflight Access-Control-Allow-Headers não deve estar vazio")
	}
}
