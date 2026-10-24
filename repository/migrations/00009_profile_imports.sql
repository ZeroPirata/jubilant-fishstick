-- +goose Up
-- +goose StatementBegin
-- Histórico das importações de currículo para o painel admin: quem enviou, quais modelos
-- foram tentados, quanto tempo levou e por que falhou. Nada do conteúdo do currículo
-- fica aqui (só formato, tamanho e quantos itens a IA encontrou).
CREATE TABLE IF NOT EXISTS profile_imports (
    id          TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    file_kind   TEXT NOT NULL,
    file_size   INTEGER NOT NULL,
    status      TEXT NOT NULL DEFAULT 'processing', -- processing | done | error | canceled
    error       TEXT,
    attempts    JSONB NOT NULL DEFAULT '[]',
    result      JSONB,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    applied_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_profile_imports_started_at ON profile_imports(started_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS profile_imports;
-- +goose StatementEnd
