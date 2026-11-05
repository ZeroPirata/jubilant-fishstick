package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCanonicalHost(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := CanonicalHost("jobpipeline.gsbnunes.com.br", []string{"gerador-curriculo.gsbnunes.com.br"})(ok)
	for _, tc := range []struct {
		host, path string
		code       int
		location   string
	}{
		{"gerador-curriculo.gsbnunes.com.br", "/static/js/app.js?v=1", http.StatusMovedPermanently, "https://jobpipeline.gsbnunes.com.br/static/js/app.js?v=1"},
		{"Gerador-Curriculo.gsbnunes.com.br:443", "/", http.StatusMovedPermanently, "https://jobpipeline.gsbnunes.com.br/"},
		{"jobpipeline.gsbnunes.com.br", "/", http.StatusTeapot, ""},
		{"localhost:8080", "/healthz", http.StatusTeapot, ""},
		{"127.0.0.1:8080", "/healthz", http.StatusTeapot, ""},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.Host = tc.host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.code || rec.Header().Get("Location") != tc.location {
			t.Errorf("%s%s: %d %q, esperado %d %q", tc.host, tc.path, rec.Code, rec.Header().Get("Location"), tc.code, tc.location)
		}
	}
	// Sem configuração, não muda nada.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "gerador-curriculo.gsbnunes.com.br"
	CanonicalHost("", nil)(ok).ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Errorf("sem configuração: %d", rec.Code)
	}
}
