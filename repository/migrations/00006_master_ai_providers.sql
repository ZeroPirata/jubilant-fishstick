-- +goose Up
-- +goose StatementBegin
-- Master: único nível que vê/edita as configurações de IA (API keys) e promove outros masters.
-- Todo master também é admin.
ALTER TABLE user_accounts ADD COLUMN IF NOT EXISTS is_master BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE user_accounts
    ADD CONSTRAINT chk_user_accounts_master_is_admin CHECK (is_admin OR NOT is_master);

-- Cadeia de fallback de providers de IA, editável pelo painel.
-- purpose: 'main' (geração do currículo) | 'scrape' (extração da vaga).
-- Tenta em ordem de position; só passa para o próximo se o anterior falhar.
-- Sem linhas para um purpose, a aplicação usa as variáveis do .env.
CREATE TABLE IF NOT EXISTS ai_providers (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    purpose     TEXT        NOT NULL CHECK (purpose IN ('main', 'scrape')),
    position    INT         NOT NULL,
    provider    TEXT        NOT NULL,
    url         TEXT        NOT NULL,
    model       TEXT        NOT NULL DEFAULT '',
    api_key_enc BYTEA,      -- AES-256-GCM (nonce || ciphertext) com SETTINGS_ENCRYPTION_KEY
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    updated_by  UUID        REFERENCES user_accounts(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_providers_purpose ON ai_providers(purpose, position);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ai_providers;
ALTER TABLE user_accounts DROP CONSTRAINT IF EXISTS chk_user_accounts_master_is_admin;
ALTER TABLE user_accounts DROP COLUMN IF EXISTS is_master;
-- +goose StatementEnd
