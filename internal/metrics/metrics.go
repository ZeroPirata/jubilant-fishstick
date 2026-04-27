package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Jobs processados pelo worker — labels: completed | error | recovered
	JobsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hackton_jobs_processed_total",
		Help: "Total de jobs processados pelo worker pipeline.",
	}, []string{"status"})

	// Duração das chamadas ao LLM — labels: ok | error | rate_limit
	LLMDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "hackton_llm_duration_seconds",
		Help:    "Duração das chamadas ao LLM em segundos.",
		Buckets: []float64{1, 5, 10, 20, 30, 60, 120, 300},
	}, []string{"status"})

	// Duração do scraper (HTTP + NL) por job — labels: ok | error
	ScraperDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "hackton_scraper_duration_seconds",
		Help:    "Duração do scraper por job em segundos.",
		Buckets: []float64{0.5, 1, 2, 5, 10, 30, 60},
	}, []string{"status"})

	// Goroutines ativas processando jobs no momento
	WorkerGoroutines = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "hackton_worker_active_goroutines",
		Help: "Goroutines ativas processando jobs no momento.",
	})

	// Requisições HTTP recebidas pela API — labels: method, path, status
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hackton_http_requests_total",
		Help: "Total de requisições HTTP recebidas pela API.",
	}, []string{"method", "path", "status"})

	// Latência das requisições HTTP — labels: method, path
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "hackton_http_request_duration_seconds",
		Help:    "Duração das requisições HTTP em segundos.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
	}, []string{"method", "path"})
)
