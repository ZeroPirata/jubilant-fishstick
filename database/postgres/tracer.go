package postgres

import (
	"context"
	"regexp"
	"strings"
	"time"

	"hackton-treino/internal/metrics"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	reSqlcName = regexp.MustCompile(`^--\s*name:\s*(\w+)`)
	reTable    = regexp.MustCompile(`(?i)\b(?:from|into|update)\s+([a-z_][a-z0-9_]*)`)
	// FROM dentro de funções (extract(epoch FROM col), substring(x FROM n)) não é tabela.
	reFuncFrom = regexp.MustCompile(`(?i)\b(?:extract|substring|trim|overlay|position)\s*\([^()]*\)`)
)

// queryLabel dá um nome de baixa cardinalidade para a consulta: o nome do sqlc
// ("-- name: QuerySelectAllSkills") ou "comando tabela" para SQL escrito à mão.
func queryLabel(sql string) string {
	sql = strings.TrimSpace(sql)
	if m := reSqlcName.FindStringSubmatch(sql); m != nil {
		return m[1]
	}
	verb := strings.ToUpper(strings.SplitN(sql, " ", 2)[0])
	if m := reTable.FindStringSubmatch(reFuncFrom.ReplaceAllString(sql, "")); m != nil {
		return verb + " " + strings.ToLower(m[1])
	}
	return verb
}

type queryTracer struct{}

type traceKey struct{}

type traceData struct {
	start time.Time
	label string
}

func (queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, traceKey{}, traceData{start: time.Now(), label: queryLabel(data.SQL)})
}

func (queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	td, ok := ctx.Value(traceKey{}).(traceData)
	if !ok {
		return
	}
	status := "ok"
	if data.Err != nil && data.Err != pgx.ErrNoRows {
		status = "error"
	}
	metrics.DBQueryDuration.WithLabelValues(td.label, status).Observe(time.Since(td.start).Seconds())
}

// RegisterPoolMetrics expõe o estado do pool de conexões no /metrics.
func RegisterPoolMetrics(pool *pgxpool.Pool) {
	gauge := func(name, help string, f func(*pgxpool.Stat) float64) {
		prometheus.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: name, Help: help},
			func() float64 { return f(pool.Stat()) }))
	}
	counter := func(name, help string, f func(*pgxpool.Stat) float64) {
		prometheus.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{Name: name, Help: help},
			func() float64 { return f(pool.Stat()) }))
	}
	gauge("hackton_db_pool_max_conns", "Máximo de conexões do pool.", func(s *pgxpool.Stat) float64 { return float64(s.MaxConns()) })
	gauge("hackton_db_pool_total_conns", "Conexões abertas.", func(s *pgxpool.Stat) float64 { return float64(s.TotalConns()) })
	gauge("hackton_db_pool_acquired_conns", "Conexões em uso agora.", func(s *pgxpool.Stat) float64 { return float64(s.AcquiredConns()) })
	gauge("hackton_db_pool_idle_conns", "Conexões ociosas.", func(s *pgxpool.Stat) float64 { return float64(s.IdleConns()) })
	counter("hackton_db_pool_acquire_total", "Total de aquisições de conexão.", func(s *pgxpool.Stat) float64 { return float64(s.AcquireCount()) })
	counter("hackton_db_pool_empty_acquire_total", "Aquisições que tiveram de esperar (pool cheio).", func(s *pgxpool.Stat) float64 { return float64(s.EmptyAcquireCount()) })
	counter("hackton_db_pool_acquire_seconds_total", "Tempo total esperando conexão.", func(s *pgxpool.Stat) float64 { return s.AcquireDuration().Seconds() })
}
