-- +goose Up
-- +goose StatementBegin
-- Último uso autenticado do sistema (atualizado no máximo a cada 5 min por usuário).
-- Alimenta "usuários ativos" nas métricas e "último acesso" no painel admin.
ALTER TABLE user_accounts ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_user_accounts_last_seen ON user_accounts(last_seen_at DESC) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_user_accounts_last_seen;
ALTER TABLE user_accounts DROP COLUMN IF EXISTS last_seen_at;
-- +goose StatementEnd
