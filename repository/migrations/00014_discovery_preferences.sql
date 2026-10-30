-- +goose Up
-- +goose StatementBegin
-- "Local de interesse" da aba Descobrir. Cidades valem para vagas híbridas e
-- presenciais; vaga remota passa de qualquer lugar. Listas vazias = sem filtro.
CREATE TABLE IF NOT EXISTS user_discovery_preferences(
    user_id         UUID PRIMARY KEY REFERENCES user_accounts(id) ON DELETE CASCADE,
    workplace_types TEXT[] NOT NULL DEFAULT '{}',
    cities          TEXT[] NOT NULL DEFAULT '{}',
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- A Gupy grava "on-site"; o resto do código usa "onsite".
UPDATE discovered_jobs SET workplace_type = 'onsite' WHERE workplace_type = 'on-site';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_discovery_preferences;
-- +goose StatementEnd
