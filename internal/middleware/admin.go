package middleware

import (
	"hackton-treino/internal/repository/admin"
	"net/http"
)

func AdminMiddleware(repo admin.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := GetUserID(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			isAdmin, err := repo.IsAdmin(r.Context(), userID.String())
			if err != nil || !isAdmin {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
