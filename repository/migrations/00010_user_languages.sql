-- +goose Up
-- +goose StatementBegin
-- Idiomas do perfil. language guarda o código (en, es, ...) quando é um idioma conhecido,
-- para o currículo em inglês mostrar "English" e o em português "Inglês"; outro idioma
-- fica com o nome digitado.
CREATE TABLE IF NOT EXISTS user_languages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    language    TEXT NOT NULL,
    proficiency TEXT NOT NULL CHECK (proficiency IN ('basic', 'intermediate', 'advanced', 'fluent', 'native')),
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ,
    deleted_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_user_languages_user_id ON user_languages(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_languages;
-- +goose StatementEnd
