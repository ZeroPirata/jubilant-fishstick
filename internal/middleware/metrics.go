package middleware

import (
	"hackton-treino/internal/metrics"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var reUUID = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// normalizePath substitui UUIDs por {id} para evitar explosão de cardinalidade no Prometheus.
func normalizePath(p string) string {
	return reUUID.ReplaceAllString(p, "{id}")
}

// MetricsMiddleware instrumenta todas as rotas com contador e histograma de latência.
// Rotas de arquivo estático (/static/, /output/) e a própria /metrics são excluídas.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/metrics" || path == "/api/v1/jobs/events" || strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/output/") {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)

		normalizedPath := normalizePath(path)
		statusStr := strconv.Itoa(lrw.statusCode)

		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, normalizedPath, statusStr).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, normalizedPath).Observe(time.Since(start).Seconds())
	})
}
