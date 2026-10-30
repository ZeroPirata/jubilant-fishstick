-- +goose Up
-- +goose StatementBegin
-- Texto da vaga como foi lido do site (ou da descrição colada / do catálogo), antes do
-- resumo da IA. A tela da vaga mostra a descrição completa; description continua
-- sendo o resumo curto.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS scraped_description TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE jobs DROP COLUMN IF EXISTS scraped_description;
-- +goose StatementEnd
