-- =============================================================================
-- DISCOVERY QUERIES
-- Busca automática de vagas por palavra-chave (aba "Descobrir").
-- =============================================================================


-- name: DiscoverySelectAllKeywords :many
SELECT DISTINCT lower(trim(f.keyword))::text AS keyword
FROM user_job_filters f
INNER JOIN user_accounts acc ON acc.id = f.user_id
WHERE f.deleted_at IS NULL AND acc.deleted_at IS NULL;


-- name: DiscoverySelectAllTechnologies :many
-- Tecnologias de interesse de todos os usuários ativos: entram na varredura completa.
SELECT DISTINCT unnest(p.technologies)::text AS technology
FROM user_discovery_preferences p
INNER JOIN user_accounts acc ON acc.id = p.user_id
WHERE acc.deleted_at IS NULL;


-- name: DiscoverySelectUserKeywords :many
SELECT DISTINCT lower(trim(keyword))::text AS keyword
FROM user_job_filters
WHERE user_id = @user_id AND deleted_at IS NULL;


-- name: DiscoveryUpsertJob :one
INSERT INTO discovered_jobs(source, external_id, title, company, location, workplace_type, international, url, description, seniority, published_at, expires_at, techs, techs_version, language)
VALUES (@source, @external_id, @title, @company, @location, @workplace_type, @international, @url, @description, @seniority::text[], @published_at, @expires_at, @techs::text[], @techs_version, @language)
ON CONFLICT (source, external_id) DO UPDATE SET
    title          = EXCLUDED.title,
    company        = EXCLUDED.company,
    location       = EXCLUDED.location,
    workplace_type = EXCLUDED.workplace_type,
    international  = EXCLUDED.international,
    url            = EXCLUDED.url,
    description    = EXCLUDED.description,
    seniority      = EXCLUDED.seniority,
    published_at   = EXCLUDED.published_at,
    expires_at     = EXCLUDED.expires_at,
    techs          = EXCLUDED.techs,
    techs_version  = EXCLUDED.techs_version,
    language       = EXCLUDED.language,
    last_seen_at   = now()
-- xmax = 0 só na linha recém-inserida: separa vaga nova de vaga atualizada.
RETURNING id, (xmax = 0)::boolean AS inserted;


-- name: DiscoveryLinkKeyword :exec
INSERT INTO discovered_job_keywords(job_id, keyword)
VALUES (@job_id, @keyword)
ON CONFLICT DO NOTHING;


-- name: DiscoveryDeleteStale :many
-- Apaga as vagas que sumiram das buscas (last_seen_at < @cutoff), que venceram ou que
-- foram publicadas antes de @published_cutoff (vaga velha que o site ainda lista), e
-- devolve quantas saíram de cada fonte. Vaga sem data de publicação só sai pelas
-- outras regras.
WITH removed AS (
    DELETE FROM discovered_jobs
    WHERE last_seen_at < @cutoff
       OR expires_at < now()
       OR published_at < @published_cutoff
    RETURNING source
)
SELECT source, count(*)::bigint AS removed
FROM removed
GROUP BY source
ORDER BY source;


-- name: DiscoveryListCandidates :many
-- Vagas ativas que citam alguma das tecnologias (índice GIN em techs) ou que a busca
-- de texto achou (@extra_ids, ver DiscoverySkillHits). Só colunas leves: a descrição
-- fica para DiscoveryPageDetails, das vagas da página. Todas as candidatas vêm para o
-- ranking (JobFit) e as contagens dos filtros; @max_count é só um teto de segurança.
SELECT
    d.id, d.source, d.title, d.company, d.location, d.workplace_type, d.international, d.url,
    d.seniority, d.published_at, d.techs, d.language
FROM discovered_jobs d
WHERE d.last_seen_at >= @active_since
  AND (d.expires_at IS NULL OR d.expires_at >= now())
  AND (d.techs && @techs::text[] OR d.id = ANY(@extra_ids::uuid[]))
ORDER BY d.published_at DESC NULLS LAST, d.id
LIMIT @max_count;


-- name: DiscoverySkillHits :many
-- Skills do perfil que não estão no catálogo de tecnologias ("APIs REST", "Arquitetura
-- Hexagonal"): busca de frase no texto da vaga, com o índice GIN de search_vec. Uma
-- linha por (termo, vaga); o Go sabe de qual skill é cada termo.
SELECT DISTINCT t.term::text AS term, d.id
FROM unnest(@terms::text[]) AS t(term)
JOIN discovered_jobs d ON d.search_vec @@ phraseto_tsquery('simple', t.term)
WHERE d.last_seen_at >= @active_since
  AND (d.expires_at IS NULL OR d.expires_at >= now());


-- name: DiscoveryPageDetails :many
-- O que só a página mostra: a descrição (trecho e casamento exato) e a vaga na
-- pipeline (Vagas), a mais recente com a mesma URL, para mostrar o andamento.
SELECT
    d.id, d.description,
    pj.id AS pipeline_job_id,
    COALESCE(pj.status::text, '')::text AS pipeline_status
FROM discovered_jobs d
LEFT JOIN LATERAL (
    SELECT j.id, j.status FROM jobs j
    WHERE j.user_id = @user_id AND j.external_url = d.url AND j.deleted_at IS NULL
    ORDER BY j.created_at DESC
    LIMIT 1
) pj ON true
WHERE d.id = ANY(@ids::uuid[]);


-- name: DiscoverySelectTechsPending :many
-- Vagas com techs calculada por outra versão do catálogo (ou nunca calculada).
SELECT id, title, description
FROM discovered_jobs
WHERE techs_version <> @version
LIMIT @max_count;


-- name: DiscoveryUpdateTechs :exec
UPDATE discovered_jobs
SET techs = @techs::text[], language = @language, techs_version = @version
WHERE id = @id;


-- name: DiscoverySelectPreferences :one
SELECT workplace_types, cities, technologies, seniorities, seniority_unknown
FROM user_discovery_preferences
WHERE user_id = @user_id;


-- name: DiscoveryUpsertPreferences :exec
INSERT INTO user_discovery_preferences(user_id, workplace_types, cities, technologies, seniorities, seniority_unknown)
VALUES (@user_id, @workplace_types::text[], @cities::text[], @technologies::text[], @seniorities::text[], @seniority_unknown)
ON CONFLICT (user_id) DO UPDATE SET
    workplace_types = EXCLUDED.workplace_types,
    cities          = EXCLUDED.cities,
    technologies    = EXCLUDED.technologies,
    seniorities     = EXCLUDED.seniorities,
    seniority_unknown = EXCLUDED.seniority_unknown,
    updated_at      = now();


-- =============================================================================
-- ADMIN: configuração e histórico das varreduras
-- =============================================================================


-- name: DiscoverySelectSettings :one
SELECT enabled, interval_minutes, catalog_terms, disabled_sources, boards, updated_at
FROM discovery_settings
WHERE id;


-- name: DiscoveryUpdateSettings :exec
UPDATE discovery_settings SET
    enabled          = @enabled,
    interval_minutes = @interval_minutes,
    catalog_terms    = @catalog_terms::text[],
    disabled_sources = @disabled_sources::text[],
    boards           = @boards::jsonb,
    updated_by       = @updated_by,
    updated_at       = now()
WHERE id;


-- name: DiscoveryInsertRun :one
INSERT INTO discovery_runs(trigger, keywords, partial, sources)
VALUES (@trigger, @keywords, @partial, @sources::text[])
RETURNING id;


-- name: DiscoveryFinishRun :exec
UPDATE discovery_runs SET
    status       = @status,
    jobs_saved   = @jobs_saved,
    jobs_new     = @jobs_new,
    jobs_removed = @jobs_removed,
    finished_at  = now()
WHERE id = @id;


-- name: DiscoveryInsertRunSource :exec
INSERT INTO discovery_run_sources(run_id, source, kind, keywords_ok, keywords_failed, jobs_saved, jobs_new, last_error, duration_ms)
VALUES (@run_id, @source, @kind, @keywords_ok, @keywords_failed, @jobs_saved, @jobs_new, @last_error, @duration_ms);


-- name: DiscoveryFailInterruptedRuns :execrows
-- Varredura que ficou "running" é de um processo que morreu no meio.
UPDATE discovery_runs SET status = 'error', finished_at = now()
WHERE status = 'running';


-- name: DiscoveryLastFullRun :one
-- Início da última varredura completa (agendada ou do painel, com todos os sites e
-- termos): base da próxima.
SELECT COALESCE(max(started_at), 'epoch'::timestamptz)::timestamptz AS started_at
FROM discovery_runs
WHERE trigger IN ('schedule', 'admin') AND NOT partial AND status <> 'running';


-- name: DiscoveryListRuns :many
SELECT id, trigger, status, keywords, partial, sources, jobs_saved, jobs_new, jobs_removed, started_at, finished_at
FROM discovery_runs
ORDER BY started_at DESC
LIMIT @max_count;


-- name: DiscoveryListRunSources :many
SELECT run_id, source, kind, keywords_ok, keywords_failed, jobs_saved, jobs_new, last_error, duration_ms
FROM discovery_run_sources
WHERE run_id = ANY(@run_ids::uuid[])
ORDER BY source;


-- name: DiscoveryLastSourceResults :many
-- Último resultado de cada site e quando ele funcionou pela última vez.
SELECT DISTINCT ON (s.source)
    s.source, s.kind, s.keywords_ok, s.keywords_failed, s.jobs_saved, s.jobs_new, s.last_error,
    r.started_at,
    (
        SELECT max(r2.started_at) FROM discovery_run_sources s2
        INNER JOIN discovery_runs r2 ON r2.id = s2.run_id
        WHERE s2.source = s.source AND s2.keywords_ok > 0
    )::timestamptz AS last_success_at
FROM discovery_run_sources s
INNER JOIN discovery_runs r ON r.id = s.run_id
ORDER BY s.source, r.started_at DESC;


-- name: DiscoveryCatalogStats :many
SELECT
    source,
    COUNT(*)::int AS total,
    (COUNT(*) FILTER (WHERE last_seen_at >= @active_since AND (expires_at IS NULL OR expires_at >= now())))::int AS active
FROM discovered_jobs
GROUP BY source
ORDER BY source;


-- name: DiscoveryPruneRuns :execrows
DELETE FROM discovery_runs WHERE started_at < @cutoff;

-- name: DiscoverySelectByURL :one
-- Vaga do catálogo com essa URL. O worker usa a descrição gravada quando a página da
-- vaga não pode ser lida (site sem leitor próprio, ou que bloqueia acesso automatizado).
SELECT title, company, description
FROM discovered_jobs
WHERE url = @url AND description <> ''
ORDER BY last_seen_at DESC
LIMIT 1;


-- name: DiscoveryCleanIndexPending :exec
-- Os índices GIN guardam as inserções numa lista pendente, varrida sequencialmente em
-- toda busca até ser incorporada. Depois de uma carga em massa (varredura, recálculo de
-- techs) ela fica grande e deixa o Descobrir lento: aqui ela é incorporada de uma vez.
SELECT gin_clean_pending_list('idx_discovered_jobs_search'::regclass),
       gin_clean_pending_list('idx_discovered_jobs_techs'::regclass);
