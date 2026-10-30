-- +goose Up
-- +goose StatementBegin
-- "Tecnologias de interesse" da aba Descobrir: nomes do catálogo do código
-- (discovery.Technologies). Filtram a lista e entram nas buscas dos sites.
ALTER TABLE user_discovery_preferences ADD COLUMN IF NOT EXISTS technologies TEXT[] NOT NULL DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_discovery_preferences DROP COLUMN IF EXISTS technologies;
-- +goose StatementEnd
