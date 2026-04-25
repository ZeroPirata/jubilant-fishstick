package middleware

import (
	"context"
	"hackton-treino/internal/security"
	"hackton-treino/internal/util"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

type contextKey string

const (
	ContextKeyUserID contextKey = "userID"
)

func GetUserID(ctx context.Context) (pgtype.UUID, bool) {
	claims, ok := ctx.Value(ContextKeyUserID).(security.ValidatedClaims)
	if !ok {
		return pgtype.UUID{}, false
	}
	pgUuid, err := util.ParseUUID(claims.UserID)
	if err != nil {
		return pgtype.UUID{}, false
	}
	return pgUuid, true
}

func GetAuthClaims(ctx context.Context) (security.ValidatedClaims, bool) {
	claims, ok := ctx.Value(ContextKeyUserID).(security.ValidatedClaims)
	return claims, ok
}

func AuthMiddleware(provider security.TokenProvider) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenString string

			if h := r.Header.Get("Authorization"); h != "" {
				parts := strings.Split(h, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					tokenString = parts[1]
				}
			}
			// Fallback para query param — usado pelo EventSource (SSE) que não suporta headers
			if tokenString == "" {
				tokenString = r.URL.Query().Get("token")
			}
			if tokenString == "" {
				http.Error(w, "missing Authorization header", http.StatusUnauthorized)
				return
			}
			claims, err := provider.Validate(tokenString)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUserID, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
