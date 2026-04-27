-- =============================================================================
-- WORKER QUERIES
-- Queries exclusivas do worker interno. Sem validação de user_id pois o worker
-- é um processo confiável que opera sobre qualquer job da pipeline.
-- NÃO exponha estas queries via API.
-- =============================================================================


-- name: WorkerSelectPendingJobs :many
SELECT j.*
FROM jobs j
INNER JOIN user_accounts acc ON acc.id = j.user_id
WHERE
    j.status = 'pending' AND
    j.deleted_at IS NULL AND
    acc.deleted_at IS NULL
ORDER BY j.created_at
LIMIT @max_count;


-- name: WorkerUpdateJobStatus :exec
UPDATE jobs
SET
    status     = @status,
    updated_at = now()
WHERE id = @id;


-- name: WorkerUpdateJob :exec
UPDATE jobs
SET
    company_name = @company_name,
    job_title    = @job_title,
    description  = @description,
    tech_stack   = @tech_stack,
    requirements = @requirements,
    language     = @language,
    updated_at   = now()
WHERE id = @id;


-- name: WorkerUpdateJobQuality :exec
UPDATE jobs
SET
    quality    = @quality,
    updated_at = now()
WHERE id = @id;


-- name: WorkerInsertGeneratedResume :one
INSERT INTO generated_resumes(job_id, user_id, content_json)
VALUES (@job_id, @user_id, @content_json)
RETURNING id;


-- name: WorkerRecoverStuckJobs :execrows
-- Retorna jobs travados em 'processing' por mais de @cutoff para 'pending'.
UPDATE jobs
SET
    status     = 'pending',
    updated_at = now()
WHERE
    status     = 'processing'
    AND deleted_at IS NULL
    AND updated_at < @cutoff;
