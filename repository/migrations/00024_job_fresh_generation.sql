-- +goose Up
-- +goose StatementBegin
-- O worker guarda a resposta da IA por hash do prompt (vaga + perfil + feedbacks +
-- idioma + modo + versão do prompt) e reaproveita quando a mesma geração é pedida de
-- novo. fresh_generation pede uma resposta nova mesmo assim ("Nova versão" no Refazer).
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS fresh_generation BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE jobs DROP COLUMN IF EXISTS fresh_generation;
-- +goose StatementEnd
