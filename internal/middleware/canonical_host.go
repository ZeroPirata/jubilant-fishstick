package middleware

import (
	"net"
	"net/http"
	"strings"
)

// CanonicalHost manda quem chega por um nome antigo para o nome atual, com 301 e o
// mesmo caminho e query (o "#/vaga/..." o navegador mantém sozinho). Só os nomes
// listados em legacy são redirecionados: localhost, IP e o próprio nome atual passam
// direto (o healthcheck e o smoke test usam localhost). Vazio desliga.
func CanonicalHost(canonical string, legacy []string) func(http.Handler) http.Handler {
	old := map[string]bool{}
	for _, h := range legacy {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			old[h] = true
		}
	}
	return func(next http.Handler) http.Handler {
		if canonical == "" || len(old) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := strings.ToLower(r.Host)
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			if !old[host] {
				next.ServeHTTP(w, r)
				return
			}
			http.Redirect(w, r, "https://"+canonical+r.URL.RequestURI(), http.StatusMovedPermanently)
		})
	}
}
