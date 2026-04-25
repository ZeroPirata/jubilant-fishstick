package middleware

import (
	"hackton-treino/internal/security"
	"net/http"
)

// RevocationMiddleware rejeita tokens cujo JTI está na blacklist Redis.
// Deve ser composto APÓS AuthMiddleware, que já armazenou ValidatedClaims no contexto.
func RevocationMiddleware(revoker security.TokenRevoker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetAuthClaims(r.Context())
			if ok && revoker.IsRevoked(r.Context(), claims.JTI) {
				http.Error(w, "token revogado", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
