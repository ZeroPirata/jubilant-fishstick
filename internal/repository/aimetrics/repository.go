// Package aimetrics guarda e agrega as tentativas de chamada aos providers de IA.
package aimetrics

import (
	"context"
	"time"

	"hackton-treino/internal/services"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Retention é por quanto tempo as tentativas ficam guardadas.
const Retention = 90 * 24 * time.Hour

type Summary struct {
	Calls            int     `json:"calls"`
	OK               int     `json:"ok"`
	SuccessRate      float64 `json:"success_rate"`
	AvgOKMs          float64 `json:"avg_ok_ms"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	// Sucessos que vieram de um provider que não era o primeiro da cadeia.
	FallbackSaves int `json:"fallback_saves"`
}

type ModelStats struct {
	Purpose         string         `json:"purpose"`
	Provider        string         `json:"provider"`
	Model           string         `json:"model"`
	Calls           int            `json:"calls"`
	OK              int            `json:"ok"`
	ByStatus        map[string]int `json:"by_status"`
	P50Ms           float64        `json:"p50_ms"`
	P95Ms           float64        `json:"p95_ms"`
	AvgPromptTokens float64        `json:"avg_prompt_tokens"`
	AvgOutputTokens float64        `json:"avg_output_tokens"`
	LastUsed        time.Time      `json:"last_used"`
	LastError       string         `json:"last_error,omitempty"`
}

type Bucket struct {
	Start  time.Time `json:"start"`
	OK     int       `json:"ok"`
	Failed int       `json:"failed"`
}

type Failure struct {
	At       time.Time `json:"at"`
	Purpose  string    `json:"purpose"`
	Provider string    `json:"provider"`
	Model    string    `json:"model"`
	Status   string    `json:"status"`
	Ms       int       `json:"ms"`
	Error    string    `json:"error"`
}

type Report struct {
	Since    time.Time    `json:"since"`
	Summary  Summary      `json:"summary"`
	Models   []ModelStats `json:"models"`
	Timeline []Bucket     `json:"timeline"`
	Failures []Failure    `json:"failures"`
}

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// RecordAICall implementa services.CallRecorder. Falha ao gravar só gera log:
// métrica nunca pode derrubar a geração do currículo.
func (r *Repository) RecordAICall(ctx context.Context, c services.AICall) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var jobID pgtype.UUID
	if c.JobID != "" {
		_ = jobID.Scan(c.JobID)
	}
	nullInt := func(n int) pgtype.Int4 { return pgtype.Int4{Int32: int32(n), Valid: n > 0} }
	errText := pgtype.Text{String: c.Error, Valid: c.Error != ""}

	_, err := r.db.Exec(ctx,
		`INSERT INTO ai_calls (purpose, provider, model, position, status, duration_ms, prompt_tokens, completion_tokens, job_id, error)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		string(c.Purpose), c.Provider, c.Model, c.Position, c.Status, int(c.Duration.Milliseconds()),
		nullInt(c.PromptTokens), nullInt(c.CompletionTokens), jobID, errText,
	)
	if err != nil {
		zap.L().Warn("aimetrics: falha ao registrar chamada", zap.Error(err))
	}
}

// Prune apaga o que passou da retenção.
func (r *Repository) Prune(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM ai_calls WHERE created_at < $1`, time.Now().Add(-Retention))
	return tag.RowsAffected(), err
}

// Report agrega as tentativas desde since. bucket define a granularidade da linha do tempo.
func (r *Repository) Report(ctx context.Context, since time.Time, bucket time.Duration) (Report, error) {
	rep := Report{Since: since, Models: []ModelStats{}, Timeline: []Bucket{}, Failures: []Failure{}}

	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE status = 'ok'),
		       COALESCE(AVG(duration_ms) FILTER (WHERE status = 'ok'), 0),
		       COALESCE(SUM(prompt_tokens), 0),
		       COALESCE(SUM(completion_tokens), 0),
		       COUNT(*) FILTER (WHERE status = 'ok' AND position > 0)
		FROM ai_calls WHERE created_at >= $1`, since,
	).Scan(&rep.Summary.Calls, &rep.Summary.OK, &rep.Summary.AvgOKMs,
		&rep.Summary.PromptTokens, &rep.Summary.CompletionTokens, &rep.Summary.FallbackSaves)
	if err != nil {
		return rep, err
	}
	if rep.Summary.Calls > 0 {
		rep.Summary.SuccessRate = float64(rep.Summary.OK) / float64(rep.Summary.Calls)
	}

	rows, err := r.db.Query(ctx, `
		SELECT purpose, provider, model,
		       COUNT(*),
		       COUNT(*) FILTER (WHERE status = 'ok'),
		       COALESCE(percentile_cont(0.5)  WITHIN GROUP (ORDER BY duration_ms) FILTER (WHERE status = 'ok'), 0),
		       COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms) FILTER (WHERE status = 'ok'), 0),
		       COALESCE(AVG(prompt_tokens), 0),
		       COALESCE(AVG(completion_tokens), 0),
		       MAX(created_at),
		       COALESCE((ARRAY_AGG(error ORDER BY created_at DESC) FILTER (WHERE status <> 'ok'))[1], ''),
		       jsonb_object_agg_strict(status, n)
		FROM (
		    SELECT *, COUNT(*) OVER (PARTITION BY purpose, provider, model, status) AS n
		    FROM ai_calls WHERE created_at >= $1
		) c
		GROUP BY purpose, provider, model
		ORDER BY purpose, COUNT(*) DESC`, since)
	if err != nil {
		return rep, err
	}
	for rows.Next() {
		var m ModelStats
		if err := rows.Scan(&m.Purpose, &m.Provider, &m.Model, &m.Calls, &m.OK, &m.P50Ms, &m.P95Ms,
			&m.AvgPromptTokens, &m.AvgOutputTokens, &m.LastUsed, &m.LastError, &m.ByStatus); err != nil {
			rows.Close()
			return rep, err
		}
		rep.Models = append(rep.Models, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return rep, err
	}

	rows, err = r.db.Query(ctx, `
		SELECT to_timestamp(floor(extract(epoch FROM created_at) / $2) * $2) AS b,
		       COUNT(*) FILTER (WHERE status = 'ok'),
		       COUNT(*) FILTER (WHERE status <> 'ok')
		FROM ai_calls WHERE created_at >= $1
		GROUP BY b ORDER BY b`, since, bucket.Seconds())
	if err != nil {
		return rep, err
	}
	for rows.Next() {
		var b Bucket
		if err := rows.Scan(&b.Start, &b.OK, &b.Failed); err != nil {
			rows.Close()
			return rep, err
		}
		rep.Timeline = append(rep.Timeline, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return rep, err
	}

	rows, err = r.db.Query(ctx, `
		SELECT created_at, purpose, provider, model, status, duration_ms, COALESCE(error, '')
		FROM ai_calls WHERE created_at >= $1 AND status <> 'ok'
		ORDER BY created_at DESC LIMIT 20`, since)
	if err != nil {
		return rep, err
	}
	defer rows.Close()
	for rows.Next() {
		var f Failure
		if err := rows.Scan(&f.At, &f.Purpose, &f.Provider, &f.Model, &f.Status, &f.Ms, &f.Error); err != nil {
			return rep, err
		}
		rep.Failures = append(rep.Failures, f)
	}
	return rep, rows.Err()
}
