-- +goose Up
-- +goose StatementBegin
-- Alvo do Descobrir (0.6.0): a função procurada (back-end, front-end, dados...) entra
-- nas preferências, junto com as tecnologias, o nível e o modo de trabalho que já
-- estavam lá. A lista esconde vagas de outra função e ordena pela função.
ALTER TABLE user_discovery_preferences ADD COLUMN IF NOT EXISTS roles TEXT[] NOT NULL DEFAULT '{}';

-- Buscas do LinkedIn geradas pelo Alvo: gerar de novo troca só as automáticas.
ALTER TABLE user_linkedin_searches ADD COLUMN IF NOT EXISTS auto BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_linkedin_searches DROP COLUMN IF EXISTS auto;
ALTER TABLE user_discovery_preferences DROP COLUMN IF EXISTS roles;
-- +goose StatementEnd
