package middleware

import (
	"net/http"
	"runtime/debug"

	"go.uber.org/zap"
)

func MiddlewarePanicRecovery(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("PANIC RECOVERED", zap.Any("error", err), zap.String("stack", string(debug.Stack())))
					w.Header().Set("Connection", "close")
					w.WriteHeader(http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
