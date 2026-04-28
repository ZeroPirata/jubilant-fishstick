-- +goose Up
-- +goose StatementBegin
-- Se for um texto simples, use TEXT. Se for um ENUM, use o nome do seu tipo.
ALTER TABLE jobs ADD COLUMN "mode" TEXT; 
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE jobs DROP COLUMN IF EXISTS "mode";
-- +goose StatementEnd