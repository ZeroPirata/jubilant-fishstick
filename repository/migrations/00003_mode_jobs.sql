-- +goose Up
-- +goose StatementBegin
-- IF NOT EXISTS: a 00001 também passou a criar essa coluna, então bancos novos já a têm.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS "mode" TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE jobs DROP COLUMN IF EXISTS "mode";
-- +goose StatementEnd