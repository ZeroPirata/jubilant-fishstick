-- +goose Up
-- +goose StatementBegin
-- Seleção automática (0.4.0): a IA escolhe, entre as vagas do Descobrir mais
-- compatíveis, as que valem a candidatura e as coloca na pipeline. A pessoa só envia.

-- Cadeia de IA própria para a triagem (vazia = usa a cadeia principal).
ALTER TABLE ai_providers DROP CONSTRAINT IF EXISTS ai_providers_purpose_check;
ALTER TABLE ai_providers ADD CONSTRAINT ai_providers_purpose_check CHECK (purpose IN ('main', 'scrape', 'triage'));

-- Quem pode usar além de admin e master (que sempre podem).
ALTER TABLE user_accounts ADD COLUMN IF NOT EXISTS auto_select_access BOOLEAN NOT NULL DEFAULT FALSE;

-- Configuração única, editada no painel admin.
--   per_run: vagas escolhidas por clique; per_day: teto de vagas escolhidas por dia
--   (por pessoa, dia de Brasília); candidates: quantas das mais compatíveis a IA lê;
--   min_score: nota mínima da IA (0–100); min_quality: compatibilidade mínima da stack
--   para entrar na leitura; mode: o que gerar (como em Nova vaga).
CREATE TABLE IF NOT EXISTS auto_select_settings(
    id          SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    per_run     INT NOT NULL DEFAULT 5 CHECK (per_run BETWEEN 1 AND 20),
    per_day     INT NOT NULL DEFAULT 10 CHECK (per_day BETWEEN 1 AND 100),
    candidates  INT NOT NULL DEFAULT 20 CHECK (candidates BETWEEN 5 AND 50),
    min_score   INT NOT NULL DEFAULT 60 CHECK (min_score BETWEEN 0 AND 100),
    min_quality TEXT NOT NULL DEFAULT 'mid' CHECK (min_quality IN ('high', 'mid', 'low')),
    mode        TEXT NOT NULL DEFAULT 'full' CHECK (mode IN ('full', 'resume_only', 'cover_only')),
    updated_by  UUID REFERENCES user_accounts(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO auto_select_settings(id) VALUES (1) ON CONFLICT DO NOTHING;

-- Cada clique no botão. Uma rodada por vez por pessoa (índice parcial).
CREATE TABLE IF NOT EXISTS auto_select_runs(
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    status      TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'done', 'empty', 'error')),
    requested   INT NOT NULL,
    candidates  INT NOT NULL DEFAULT 0,
    analyzed    INT NOT NULL DEFAULT 0,
    selected    INT NOT NULL DEFAULT 0,
    error       TEXT NOT NULL DEFAULT '',
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_auto_select_runs_user ON auto_select_runs(user_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_auto_select_runs_started ON auto_select_runs(started_at);
CREATE UNIQUE INDEX IF NOT EXISTS uq_auto_select_running ON auto_select_runs(user_id) WHERE status = 'running';

-- Cada vaga lida pela IA numa rodada: nota, motivo e se foi escolhida. job_id fica
-- NULL quando a pessoa apaga a vaga da pipeline ("descartada" nas métricas). Também
-- evita que a mesma vaga seja lida de novo por 14 dias.
CREATE TABLE IF NOT EXISTS auto_select_picks(
    run_id       UUID NOT NULL REFERENCES auto_select_runs(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    url          TEXT NOT NULL,
    title        TEXT NOT NULL DEFAULT '',
    company      TEXT NOT NULL DEFAULT '',
    coverage_pct INT NOT NULL DEFAULT 0,
    quality      TEXT NOT NULL DEFAULT '',
    score        INT,
    selected     BOOLEAN NOT NULL DEFAULT FALSE,
    reason       TEXT NOT NULL DEFAULT '',
    job_id       UUID REFERENCES jobs(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY(run_id, url)
);
CREATE INDEX IF NOT EXISTS idx_auto_select_picks_user ON auto_select_picks(user_id, created_at DESC);

-- Vagas da pipeline postas pela seleção automática (tag "auto-gerado").
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS auto_selected BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS auto_score INT;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS auto_reason TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE jobs DROP COLUMN IF EXISTS auto_reason;
ALTER TABLE jobs DROP COLUMN IF EXISTS auto_score;
ALTER TABLE jobs DROP COLUMN IF EXISTS auto_selected;
DROP TABLE IF EXISTS auto_select_picks;
DROP TABLE IF EXISTS auto_select_runs;
DROP TABLE IF EXISTS auto_select_settings;
ALTER TABLE user_accounts DROP COLUMN IF EXISTS auto_select_access;
DELETE FROM ai_providers WHERE purpose = 'triage';
ALTER TABLE ai_providers DROP CONSTRAINT IF EXISTS ai_providers_purpose_check;
ALTER TABLE ai_providers ADD CONSTRAINT ai_providers_purpose_check CHECK (purpose IN ('main', 'scrape'));
-- +goose StatementEnd
