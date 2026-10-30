-- +goose Up
-- +goose StatementBegin
-- Vagas encontradas pela busca automática (aba "Descobrir"). São globais: a mesma vaga
-- da Gupy aparece para todo usuário que tem uma palavra-chave que a encontrou.
-- last_seen_at é renovado a cada busca em que a vaga ainda aparece; vaga que some da
-- fonte deixa de ser listada e depois é apagada.
CREATE TABLE IF NOT EXISTS discovered_jobs(
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source         TEXT NOT NULL,
    external_id    TEXT NOT NULL,
    title          TEXT NOT NULL,
    company        TEXT NOT NULL DEFAULT '',
    location       TEXT NOT NULL DEFAULT '',
    workplace_type TEXT NOT NULL DEFAULT '',
    url            TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    published_at   TIMESTAMPTZ,
    expires_at     TIMESTAMPTZ,
    first_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(source, external_id)
);
CREATE INDEX IF NOT EXISTS idx_discovered_jobs_last_seen ON discovered_jobs(last_seen_at);

-- Qual palavra-chave (já normalizada: minúscula, sem espaços nas pontas) encontrou a vaga.
CREATE TABLE IF NOT EXISTS discovered_job_keywords(
    job_id  UUID NOT NULL REFERENCES discovered_jobs(id) ON DELETE CASCADE,
    keyword TEXT NOT NULL,

    PRIMARY KEY(job_id, keyword)
);
CREATE INDEX IF NOT EXISTS idx_discovered_job_keywords_keyword ON discovered_job_keywords(keyword);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS discovered_job_keywords;
DROP TABLE IF EXISTS discovered_jobs;
-- +goose StatementEnd
