-- name: QueryInsertUrlJob :one
INSERT INTO jobs(external_url, user_id, mode)
VALUES (@external_url, @user_id, @mode) RETURNING id;

-- name: QueryInsertFullJob :one
INSERT INTO jobs(external_url, user_id, company_name, job_title, description,
    source_description, tech_stack, requirements, language, mode)
VALUES (
    @external_url, @user_id, @company_name, @job_title, @description,
    @description, @tech_stack, @requirements, @language, @mode
) RETURNING id;


-- name: QuerySelectJobsForUser :many
SELECT j.*, COUNT(*) OVER() AS total_count
FROM jobs j
INNER JOIN user_accounts acc ON acc.id = j.user_id
WHERE
    j.user_id = @user_id AND
    j.deleted_at IS NULL AND
    acc.deleted_at IS NULL AND
    (NULLIF(@status::TEXT, '') IS NULL OR j.status = @status::job_status) AND
    (NULLIF(@quality::TEXT, '') IS NULL OR j.quality = @quality::job_quality) AND
    -- Etapa da candidatura, como as colunas do quadro: ready = currículo pronto e
    -- ainda não enviado; closed = contratado, não seguiu ou desistência.
    (NULLIF(@stage::TEXT, '') IS NULL
        OR (@stage::TEXT = 'ready'  AND j.application_stage IS NULL AND j.status = 'completed')
        OR (@stage::TEXT = 'closed' AND j.application_stage IN ('hired', 'rejected', 'withdrawn'))
        OR j.application_stage = @stage::TEXT) AND
    -- Origem: auto = escolhida pela Seleção com IA; manual = cadastrada por você.
    (NULLIF(@origin::TEXT, '') IS NULL OR j.auto_selected = (@origin::TEXT = 'auto'))
ORDER BY j.created_at DESC
LIMIT @size OFFSET @cursor;


-- name: QuerySelectJobFitRows :many
-- Só o necessário para calcular a compatibilidade (a % não fica gravada: depende das
-- skills de agora). Usada quando a lista filtra ou ordena por %: o Go calcula, filtra,
-- ordena e pagina, e depois busca as vagas da página com QuerySelectJobsByIds.
SELECT j.id, j.tech_stack, j.created_at
FROM jobs j
INNER JOIN user_accounts acc ON acc.id = j.user_id
WHERE
    j.user_id = @user_id AND
    j.deleted_at IS NULL AND
    acc.deleted_at IS NULL AND
    (NULLIF(@status::TEXT, '') IS NULL OR j.status = @status::job_status) AND
    (NULLIF(@quality::TEXT, '') IS NULL OR j.quality = @quality::job_quality) AND
    -- Etapa da candidatura, como as colunas do quadro: ready = currículo pronto e
    -- ainda não enviado; closed = contratado, não seguiu ou desistência.
    (NULLIF(@stage::TEXT, '') IS NULL
        OR (@stage::TEXT = 'ready'  AND j.application_stage IS NULL AND j.status = 'completed')
        OR (@stage::TEXT = 'closed' AND j.application_stage IN ('hired', 'rejected', 'withdrawn'))
        OR j.application_stage = @stage::TEXT) AND
    -- Origem: auto = escolhida pela Seleção com IA; manual = cadastrada por você.
    (NULLIF(@origin::TEXT, '') IS NULL OR j.auto_selected = (@origin::TEXT = 'auto'))
ORDER BY j.created_at DESC
LIMIT 5000;


-- name: QuerySelectJobsByIds :many
-- Mesmas colunas de QuerySelectJobsForUser (total_count vem do Go).
SELECT j.*, 0::BIGINT AS total_count
FROM jobs j
WHERE j.user_id = @user_id AND j.deleted_at IS NULL AND j.id = ANY(@ids::UUID[]);


-- name: QuerySelectJobForUser :one
SELECT j.*
FROM jobs j
WHERE j.id = @id AND j.user_id = @user_id AND j.deleted_at IS NULL;


-- name: QueryReprocessJob :exec
UPDATE jobs
SET
    status     = 'pending',
    mode       = COALESCE(NULLIF(@mode::text, ''), jobs.mode),
    language   = COALESCE(NULLIF(@language::text, ''), jobs.language),
    fresh_generation = @fresh_generation,
    updated_at = now()
WHERE id = @id AND user_id = @user_id;


-- name: QueryUpdateJobQuality :exec
-- A listagem recalcula a compatibilidade com o perfil atual; grava quando mudou para o
-- filtro de qualidade continuar batendo com o que a tela mostra.
UPDATE jobs
SET quality = @quality
WHERE id = @id AND user_id = @user_id;


-- name: QueryUpdateJob :exec
UPDATE jobs
SET
    company_name = COALESCE(sqlc.narg('company_name'), company_name),
    job_title = COALESCE(sqlc.narg('job_title'), job_title),
    description = COALESCE(sqlc.narg('description'), description),
    -- Descrição editada pelo usuário passa a ser a vaga no próximo processamento.
    source_description = COALESCE(sqlc.narg('description'), source_description),
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

-- name: QueryDeleteResume :one
-- RETURNING para o handler responder 404 quando nada foi apagado, em vez de 204.
DELETE FROM generated_resumes
WHERE id = @id AND job_id = @job_id AND user_id = @user_id
RETURNING id;
-- ── Quadro de candidaturas ──

-- name: QueryApplicationJobOwned :one
SELECT j.id FROM jobs j
WHERE j.id = @id AND j.user_id = @user_id AND j.deleted_at IS NULL
FOR UPDATE;

-- name: QueryInsertApplicationEvent :one
INSERT INTO job_application_events(job_id, user_id, stage, note, happened_at)
VALUES (@job_id, @user_id, @stage, @note, @happened_at)
RETURNING id;

-- name: QueryDeleteApplicationEvent :one
DELETE FROM job_application_events
WHERE id = @id AND job_id = @job_id AND user_id = @user_id
RETURNING id;

-- name: QueryRecomputeApplicationStage :exec
-- Etapa atual = a última mudança de etapa do histórico (anotações não contam);
-- 'ready' ou nenhuma = ainda não enviado.
UPDATE jobs j
SET application_stage = NULLIF(last.stage, 'ready'),
    application_stage_at = CASE WHEN last.stage IS NULL OR last.stage = 'ready' THEN NULL ELSE last.happened_at END
FROM (SELECT @job_id::uuid AS job_id) k
LEFT JOIN LATERAL (
    SELECT e.stage, e.happened_at
    FROM job_application_events e
    WHERE e.job_id = k.job_id AND e.stage <> 'note'
    ORDER BY e.happened_at DESC, e.created_at DESC
    LIMIT 1
) last ON true
WHERE j.id = k.job_id AND j.user_id = @user_id;

-- name: QueryListApplicationEvents :many
SELECT e.id, e.stage, e.note, e.happened_at, e.created_at
FROM job_application_events e
WHERE e.job_id = @job_id AND e.user_id = @user_id
ORDER BY e.happened_at DESC, e.created_at DESC
LIMIT 200;

-- name: QueryApplicationBoard :many
-- Vagas do quadro: as concluídas (currículo pronto) e as que já têm etapa. applied_at
-- é o primeiro envio; reached_interview diz se chegou a entrevista (taxa de resposta),
-- mesmo que depois tenha sido encerrada.
SELECT
    j.id, j.job_title, j.company_name, j.external_url, j.quality, j.status,
    j.application_stage, j.application_stage_at, j.created_at, j.auto_selected,
    (SELECT max(gr.created_at) FROM generated_resumes gr WHERE gr.job_id = j.id)::timestamptz AS resume_at,
    (SELECT min(e.happened_at) FROM job_application_events e WHERE e.job_id = j.id AND e.stage = 'applied')::timestamptz AS applied_at,
    EXISTS (SELECT 1 FROM job_application_events e WHERE e.job_id = j.id AND e.stage IN ('interview', 'offer', 'hired')) AS reached_interview,
    COALESCE((SELECT e.note FROM job_application_events e WHERE e.job_id = j.id AND e.note <> ''
              ORDER BY e.happened_at DESC, e.created_at DESC LIMIT 1), '')::text AS last_note
FROM jobs j
WHERE j.user_id = @user_id AND j.deleted_at IS NULL
  AND (j.application_stage IS NOT NULL OR j.status = 'completed')
ORDER BY COALESCE(j.application_stage_at, j.created_at) DESC
LIMIT 500;

-- name: QueryInsertAutoJob :one
-- Vaga posta pela seleção automática (tag "auto-gerado"). Já na pipeline: não duplica.
INSERT INTO jobs(external_url, user_id, company_name, job_title, language, mode, auto_selected, auto_score, auto_reason)
VALUES (@external_url, @user_id, @company_name, @job_title, @language, @mode, true, @auto_score, @auto_reason)
ON CONFLICT (user_id, external_url) DO NOTHING
RETURNING id;
