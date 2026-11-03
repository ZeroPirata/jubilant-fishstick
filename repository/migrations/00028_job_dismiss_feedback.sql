-- +goose Up
-- +goose StatementBegin
-- Vagas do Descobrir que o usuário dispensou: somem da lista e dos alertas só para ele
-- (a vaga continua no catálogo). Sai junto com a vaga quando ela deixa o catálogo.
CREATE TABLE IF NOT EXISTS user_dismissed_jobs(
    user_id    UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    job_id     UUID NOT NULL REFERENCES discovered_jobs(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY(user_id, job_id)
);

-- Avaliação dos alertas (útil / não útil), para medir se a regra acerta. Sem FK para a
-- vaga: a avaliação sobrevive à vaga sair do catálogo. quality e coverage_pct são a
-- compatibilidade no momento do alerta. Apagada depois de 180 dias.
CREATE TABLE IF NOT EXISTS job_alert_feedback(
    user_id      UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    job_id       UUID NOT NULL,
    relevant     BOOLEAN NOT NULL,
    quality      TEXT NOT NULL CHECK (quality IN ('high', 'mid', 'low')),
    coverage_pct INT NOT NULL CHECK (coverage_pct BETWEEN 0 AND 100),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY(user_id, job_id)
);
CREATE INDEX IF NOT EXISTS idx_job_alert_feedback_created ON job_alert_feedback(created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS job_alert_feedback;
DROP TABLE IF EXISTS user_dismissed_jobs;
-- +goose StatementEnd
