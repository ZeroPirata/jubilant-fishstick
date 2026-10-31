-- +goose Up
-- +goose StatementBegin
-- Alertas de vagas novas (sininho na plataforma). Vaga nova = entrou no catálogo do
-- Descobrir (first_seen_at) depois de seen_at, com compatibilidade a partir de
-- min_quality. Abrir o sininho ou o Descobrir avança seen_at: a vaga não é avisada de
-- novo. Sem linha = padrões (ligado, só compatibilidade alta, últimas 24h).
CREATE TABLE IF NOT EXISTS user_job_alerts(
    user_id     UUID PRIMARY KEY REFERENCES user_accounts(id) ON DELETE CASCADE,
    enabled     BOOLEAN NOT NULL DEFAULT true,
    min_quality TEXT NOT NULL DEFAULT 'high' CHECK (min_quality IN ('high', 'mid')),
    seen_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_discovered_jobs_first_seen ON discovered_jobs(first_seen_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_discovered_jobs_first_seen;
DROP TABLE IF EXISTS user_job_alerts;
-- +goose StatementEnd
