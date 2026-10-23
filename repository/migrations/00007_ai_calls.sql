-- +goose Up
-- +goose StatementBegin
-- Cada tentativa de chamada a um provider de IA (inclusive as que falharam e
-- passaram a vez para o próximo da cadeia). Alimenta a aba Métricas → IA.
CREATE TABLE IF NOT EXISTS ai_calls (
    id                BIGSERIAL   PRIMARY KEY,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    purpose           TEXT        NOT NULL,           -- main | scrape
    provider          TEXT        NOT NULL,
    model             TEXT        NOT NULL,
    position          INT         NOT NULL,           -- posição na cadeia (0 = primeiro)
    status            TEXT        NOT NULL,           -- ok | rate_limit | timeout | truncated | empty | parse_error | http_error | network_error | error
    duration_ms       INT         NOT NULL,
    prompt_tokens     INT,
    completion_tokens INT,
    job_id            UUID        REFERENCES jobs(id) ON DELETE SET NULL,
    error             TEXT
);

CREATE INDEX IF NOT EXISTS idx_ai_calls_created_at ON ai_calls(created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ai_calls;
-- +goose StatementEnd
