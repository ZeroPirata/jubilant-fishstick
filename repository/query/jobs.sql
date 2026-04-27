-- name: QueryInsertUrlJob :one
INSERT INTO jobs(external_url, user_id)
VALUES (@external_url, @user_id) RETURNING id;

-- name: QueryInsertFullJob :one
INSERT INTO jobs(external_url, user_id, company_name, job_title, description,
    tech_stack, requirements, language)
VALUES (
    @external_url, @user_id, @company_name, @job_title, @description,
    @tech_stack, @requirements, @language
) RETURNING id;


-- name: QuerySelectJobsForUser :many
SELECT j.*, COUNT(*) OVER() AS total_count
FROM jobs j
INNER JOIN user_accounts acc ON acc.id = j.user_id
WHERE
    j.user_id = @user_id AND
    j.deleted_at IS NULL AND
    acc.deleted_at IS NULL AND
    (@status::TEXT IS NULL OR j.status = @status::job_status) AND
    (@quality::TEXT IS NULL OR j.quality = @quality::job_quality)
ORDER BY j.created_at DESC
LIMIT @size OFFSET @cursor;


-- name: QueryReprocessJob :exec
UPDATE jobs
SET
    status     = 'pending',
    updated_at = now()
WHERE id = @id AND user_id = @user_id;


-- name: QueryUpdateJob :exec
UPDATE jobs
SET
    company_name = COALESCE(sqlc.narg('company_name'), company_name),
    job_title = COALESCE(sqlc.narg('job_title'), job_title),
    description = COALESCE(sqlc.narg('description'), description),
    tech_stack = COALESCE(sqlc.narg('tech_stack'), tech_stack),
    requirements = COALESCE(sqlc.narg('requirements'), requirements),
    language = COALESCE(sqlc.narg('language'), language)
WHERE
    id = @id and user_id = @user_id;

-- name: QueryDeleteJob :exec
DELETE FROM jobs 
WHERE id = @job_id 
AND user_id = @user_id;

-- name: QuerySelectResumeJob :one
SELECT 
    gr.id, gr.content_json, gr.resume_pdf_path, gr.cover_letter_path,
    j.company_name, j.job_title, j.language, j.quality
FROM generated_resumes gr
JOIN jobs j ON j.id = gr.job_id
WHERE gr.id = @id AND gr.user_id = @user_id;

-- name: QueryUpdateResumePaths :exec
UPDATE generated_resumes
SET 
    resume_pdf_path = @resume_pdf_path,
    cover_letter_path = @cover_letter_path
WHERE
    id = @id AND user_id = @user_id;

-- name: QueryFindJobByUrl :one
SELECT j.id FROM jobs j
WHERE j.external_url = @external_url AND j.user_id = @user_id;


-- name: QueryListResumesForUser :many
SELECT
    gr.id, gr.job_id, gr.resume_pdf_path, gr.cover_letter_path, gr.created_at, gr.content_json,
    j.company_name, j.job_title, j.quality,
    COUNT(*) OVER() AS total_count
FROM generated_resumes gr
JOIN jobs j ON j.id = gr.job_id
WHERE gr.user_id = @user_id AND j.id = @job_id
ORDER BY gr.created_at DESC
LIMIT @size OFFSET @cursor;

-- name: QueryDeleteResume :exec
DELETE FROM generated_resumes
WHERE id = @id AND user_id = @user_id;