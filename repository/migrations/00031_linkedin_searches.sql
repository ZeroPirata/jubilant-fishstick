-- +goose Up
-- +goose StatementBegin
-- Buscas salvas no LinkedIn (0.5.0): o JobPipeline só monta o link de busca com os
-- filtros escolhidos; quem abre é o usuário, no próprio navegador. Os valores seguem
-- os parâmetros da URL do LinkedIn (internal/linkedinsearch). Listas vazias e textos
-- vazios = sem filtro.
CREATE TABLE IF NOT EXISTS user_linkedin_searches(
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    name           TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    keywords       TEXT NOT NULL DEFAULT '' CHECK (char_length(keywords) <= 200),
    geo_id         TEXT NOT NULL DEFAULT '' CHECK (geo_id ~ '^[0-9]{0,20}$'),
    geo_label      TEXT NOT NULL DEFAULT '' CHECK (char_length(geo_label) <= 80),
    posted_within  TEXT NOT NULL DEFAULT '' CHECK (posted_within IN ('', 'r86400', 'r604800', 'r2592000')),
    workplace      TEXT[] NOT NULL DEFAULT '{}' CHECK (workplace <@ ARRAY['1','2','3']),
    experience     TEXT[] NOT NULL DEFAULT '{}' CHECK (experience <@ ARRAY['1','2','3','4','5','6']),
    job_types      TEXT[] NOT NULL DEFAULT '{}' CHECK (job_types <@ ARRAY['F','P','C','T','I','V','O']),
    easy_apply     BOOLEAN NOT NULL DEFAULT false,
    few_applicants BOOLEAN NOT NULL DEFAULT false,
    in_network     BOOLEAN NOT NULL DEFAULT false,
    sort           TEXT NOT NULL DEFAULT '' CHECK (sort IN ('', 'DD', 'R')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_user_linkedin_searches_user ON user_linkedin_searches(user_id, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_linkedin_searches;
-- +goose StatementEnd
