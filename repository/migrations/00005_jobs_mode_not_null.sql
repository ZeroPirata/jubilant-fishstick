-- +goose Up
-- +goose StatementBegin
-- Alinha bancos que criaram "mode" pela 00003 (nullable, sem default) com a 00001
-- (NOT NULL DEFAULT 'full'), que é o que o código Go espera (Mode string).
UPDATE jobs SET mode = 'full' WHERE mode IS NULL;
ALTER TABLE jobs ALTER COLUMN mode SET DEFAULT 'full';
ALTER TABLE jobs ALTER COLUMN mode SET NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE jobs ALTER COLUMN mode DROP NOT NULL;
ALTER TABLE jobs ALTER COLUMN mode DROP DEFAULT;
-- +goose StatementEnd
