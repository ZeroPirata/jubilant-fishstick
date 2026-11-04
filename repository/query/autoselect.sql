-- ── Seleção automática (0.4.0) ──

-- name: AutoSelectGetSettings :one
SELECT enabled, per_run, per_day, candidates, min_score, min_quality, mode, updated_at
FROM auto_select_settings WHERE id = 1;

-- name: AutoSelectSaveSettings :exec
INSERT INTO auto_select_settings(id, enabled, per_run, per_day, candidates, min_score, min_quality, mode, updated_by, updated_at)
VALUES (1, @enabled, @per_run, @per_day, @candidates, @min_score, @min_quality, @mode, @updated_by, now())
ON CONFLICT (id) DO UPDATE SET
    enabled = EXCLUDED.enabled, per_run = EXCLUDED.per_run, per_day = EXCLUDED.per_day,
    candidates = EXCLUDED.candidates, min_score = EXCLUDED.min_score,
    min_quality = EXCLUDED.min_quality, mode = EXCLUDED.mode,
    updated_by = EXCLUDED.updated_by, updated_at = now();

-- name: AutoSelectAccess :one
-- Admin e master sempre podem; os outros, quando o admin libera.
SELECT (is_admin OR is_master OR auto_select_access)::bool AS allowed
FROM user_accounts WHERE id = @user_id AND deleted_at IS NULL;

-- name: AutoSelectSetAccess :execrows
UPDATE user_accounts SET auto_select_access = @allowed, updated_at = now()
WHERE id = @user_id AND deleted_at IS NULL;

-- name: AutoSelectExpireStale :exec
-- Rodada que ficou "running" (servidor reiniciou no meio) não pode travar a pessoa.
UPDATE auto_select_runs
SET status = 'error', error = 'interrompida (o servidor reiniciou)', finished_at = now()
WHERE status = 'running' AND started_at < @before;

-- name: AutoSelectStartRun :one
INSERT INTO auto_select_runs(user_id, requested) VALUES (@user_id, @requested)
RETURNING id, started_at;

-- name: AutoSelectFinishRun :exec
UPDATE auto_select_runs
SET status = @status, candidates = @candidates, analyzed = @analyzed, selected = @selected,
    error = @error, finished_at = now()
WHERE id = @id;

-- name: AutoSelectUsedSince :one
-- Vagas escolhidas desde o início do dia: o teto diário conta vagas, não cliques.
SELECT count(*)::int FROM auto_select_picks
WHERE user_id = @user_id AND selected AND created_at >= @since;

-- name: AutoSelectExcludedUrls :many
-- Já estão na pipeline, ou a IA já leu nos últimos 14 dias (escolhida ou não).
SELECT j.external_url::text AS url FROM jobs j WHERE j.user_id = @user_id
UNION
SELECT p.url FROM auto_select_picks p WHERE p.user_id = @user_id AND p.created_at >= now() - interval '14 days';

-- name: AutoSelectDescriptions :many
SELECT id, description FROM discovered_jobs WHERE id = ANY(@ids::uuid[]);

-- name: AutoSelectInsertPick :exec
INSERT INTO auto_select_picks(run_id, user_id, url, title, company, coverage_pct, quality, score, selected, reason, job_id)
VALUES (@run_id, @user_id, @url, @title, @company, @coverage_pct, @quality, sqlc.narg('score'), @selected, @reason, sqlc.narg('job_id'))
ON CONFLICT (run_id, url) DO NOTHING;

-- name: AutoSelectLatestRun :one
SELECT id, status, requested, candidates, analyzed, selected, error, started_at, finished_at
FROM auto_select_runs WHERE user_id = @user_id
ORDER BY started_at DESC LIMIT 1;

-- name: AutoSelectGetRun :one
SELECT id, status, requested, candidates, analyzed, selected, error, started_at, finished_at
FROM auto_select_runs WHERE id = @id AND user_id = @user_id;

-- name: AutoSelectRunPicks :many
SELECT p.url, p.title, p.company, p.coverage_pct, p.quality, p.score, p.selected, p.reason,
       p.job_id, COALESCE(j.status::text, '')::text AS job_status
FROM auto_select_picks p
LEFT JOIN jobs j ON j.id = p.job_id
WHERE p.run_id = @run_id AND p.user_id = @user_id
ORDER BY p.selected DESC, p.score DESC NULLS LAST, p.coverage_pct DESC;

-- ── Métricas (admin) ──

-- name: AutoSelectMetricsSummary :one
WITH r AS (SELECT * FROM auto_select_runs WHERE started_at >= @since),
     p AS (SELECT * FROM auto_select_picks WHERE created_at >= @since),
     sel AS (
        SELECT p.job_id, j.status,
               EXISTS (SELECT 1 FROM job_application_events e WHERE e.job_id = p.job_id AND e.stage = 'applied') AS applied,
               EXISTS (SELECT 1 FROM job_application_events e WHERE e.job_id = p.job_id AND e.stage IN ('interview', 'offer', 'hired')) AS interview
        FROM p LEFT JOIN jobs j ON j.id = p.job_id
        WHERE p.selected
     ),
     manual AS (
        SELECT EXISTS (SELECT 1 FROM job_application_events e WHERE e.job_id = j.id AND e.stage = 'applied') AS applied
        FROM jobs j
        WHERE j.created_at >= @since AND NOT j.auto_selected AND j.status = 'completed' AND j.deleted_at IS NULL
     ),
     c AS (SELECT * FROM ai_calls WHERE created_at >= @since AND purpose = 'triage')
SELECT
    (SELECT count(*) FROM r)::int AS runs,
    (SELECT count(*) FROM r WHERE status = 'error')::int AS runs_error,
    (SELECT count(*) FROM r WHERE status = 'empty')::int AS runs_empty,
    (SELECT count(DISTINCT user_id) FROM r)::int AS users,
    (SELECT count(*) FROM p)::int AS analyzed,
    (SELECT count(*) FROM p WHERE selected)::int AS selected,
    COALESCE((SELECT avg(score) FROM p WHERE selected), 0)::float8 AS avg_score_selected,
    COALESCE((SELECT avg(score) FROM p WHERE NOT selected AND score IS NOT NULL), 0)::float8 AS avg_score_skipped,
    (SELECT count(*) FROM sel WHERE job_id IS NULL)::int AS discarded,
    (SELECT count(*) FROM sel WHERE status = 'completed')::int AS generated,
    (SELECT count(*) FROM sel WHERE status = 'error')::int AS gen_errors,
    (SELECT count(*) FROM sel WHERE applied)::int AS applied,
    (SELECT count(*) FROM sel WHERE interview)::int AS interviews,
    (SELECT count(*) FROM manual)::int AS manual_generated,
    (SELECT count(*) FROM manual WHERE applied)::int AS manual_applied,
    (SELECT count(*) FROM c)::int AS ai_calls,
    (SELECT count(*) FROM c WHERE status = 'ok')::int AS ai_ok,
    COALESCE((SELECT sum(prompt_tokens) FROM c), 0)::bigint AS prompt_tokens,
    COALESCE((SELECT sum(completion_tokens) FROM c), 0)::bigint AS completion_tokens,
    COALESCE((SELECT avg(duration_ms) FROM c WHERE status = 'ok'), 0)::float8 AS avg_duration_ms;

-- name: AutoSelectMetricsRuns :many
SELECT r.id, u.email, r.status, r.requested, r.candidates, r.analyzed, r.selected, r.error,
       r.started_at, r.finished_at
FROM auto_select_runs r
JOIN user_accounts u ON u.id = r.user_id
WHERE r.started_at >= @since
ORDER BY r.started_at DESC
LIMIT 30;

-- name: AutoSelectMetricsUsers :many
SELECT u.email,
       count(DISTINCT p.run_id)::int AS runs,
       count(*) FILTER (WHERE p.selected)::int AS selected,
       count(*) FILTER (WHERE p.selected AND p.job_id IS NULL)::int AS discarded,
       count(*) FILTER (WHERE p.selected AND EXISTS (
           SELECT 1 FROM job_application_events e WHERE e.job_id = p.job_id AND e.stage = 'applied'))::int AS applied
FROM auto_select_picks p
JOIN user_accounts u ON u.id = p.user_id
WHERE p.created_at >= @since
GROUP BY u.email
ORDER BY selected DESC, runs DESC
LIMIT 50;
