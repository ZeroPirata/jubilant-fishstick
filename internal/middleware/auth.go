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
	userID, ok := ctx.Value(ContextKeyUserID).(string)

	pgUuid, err := util.ParseUUID(userID)
	if err != nil {
		return pgtype.UUID{}, false
	}

	return pgUuid, ok
}

func AuthMiddleware(provider security.TokenProvider) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing Authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "invalid Authorization header", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]
			token, err := provider.Validate(tokenString)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUserID, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
