-- +goose Up
-- +goose StatementBegin
-- Descrição que o usuário colou no cadastro (ou editou). A coluna description é
-- sobrescrita pelo worker com o resumo da IA; sem esta, o "Refazer" não tem como saber
-- se a descrição é do usuário (usar e não abrir o site) ou o resumo (abrir o site).
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS source_description TEXT;

-- Vagas que o worker ainda não leu com sucesso: a descrição só pode ter vindo do
-- usuário (o worker grava a stack junto com o resumo).
UPDATE jobs SET source_description = description
WHERE description IS NOT NULL AND btrim(description) <> ''
  AND status IN ('pending', 'error')
  AND (tech_stack IS NULL OR cardinality(tech_stack) = 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE jobs DROP COLUMN IF EXISTS source_description;
-- +goose StatementEnd
