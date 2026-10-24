-- +goose Up
-- +goose StatementBegin
-- Modelo que está tentando agora (model, provider, since): o admin acompanha a leitura ao vivo.
ALTER TABLE profile_imports ADD COLUMN IF NOT EXISTS current JSONB;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE profile_imports DROP COLUMN IF EXISTS current;
-- +goose StatementEnd
