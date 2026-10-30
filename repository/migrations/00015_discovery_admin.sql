-- +goose Up
-- +goose StatementBegin
-- Configuração da busca automática de vagas, editável no painel admin. Linha única.
-- disabled_sources (e não "enabled"): fonte nova entra ligada sem precisar de migration.
CREATE TABLE IF NOT EXISTS discovery_settings(
    id               BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    interval_minutes INT     NOT NULL DEFAULT 720 CHECK (interval_minutes BETWEEN 15 AND 10080),
    catalog_terms    TEXT[]  NOT NULL DEFAULT '{}',
    disabled_sources TEXT[]  NOT NULL DEFAULT '{}',
    updated_by       UUID    REFERENCES user_accounts(id) ON DELETE SET NULL,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO discovery_settings(id, catalog_terms) VALUES (TRUE, ARRAY[
    'desenvolvedor', 'programador', 'engenheiro de software', 'software engineer', 'developer',
    'backend', 'frontend', 'full stack', 'mobile', 'devops', 'sre', 'cloud',
    'engenheiro de dados', 'cientista de dados', 'analista de dados', 'qa',
    'analista de sistemas', 'tech lead', 'arquiteto de software', 'segurança da informação'
]) ON CONFLICT (id) DO NOTHING;

-- Uma linha por varredura. trigger: schedule (agendada) | admin (botão do painel) |
-- user (botão "Buscar novas vagas", só com as palavras-chave do usuário).
CREATE TABLE IF NOT EXISTS discovery_runs(
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trigger      TEXT NOT NULL CHECK (trigger IN ('schedule', 'admin', 'user')),
    status       TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'success', 'partial', 'error')),
    keywords     INT  NOT NULL DEFAULT 0,
    jobs_saved   INT  NOT NULL DEFAULT 0,
    jobs_removed INT  NOT NULL DEFAULT 0,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_discovery_runs_started ON discovery_runs(started_at DESC);

-- Resultado de cada site dentro de uma varredura.
CREATE TABLE IF NOT EXISTS discovery_run_sources(
    run_id          UUID NOT NULL REFERENCES discovery_runs(id) ON DELETE CASCADE,
    source          TEXT NOT NULL,
    keywords_ok     INT  NOT NULL DEFAULT 0,
    keywords_failed INT  NOT NULL DEFAULT 0,
    jobs_saved      INT  NOT NULL DEFAULT 0,
    last_error      TEXT NOT NULL DEFAULT '',
    duration_ms     INT  NOT NULL DEFAULT 0,

    PRIMARY KEY(run_id, source)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS discovery_run_sources;
DROP TABLE IF EXISTS discovery_runs;
DROP TABLE IF EXISTS discovery_settings;
-- +goose StatementEnd
