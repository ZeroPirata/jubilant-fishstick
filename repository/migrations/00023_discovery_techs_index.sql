-- +goose Up
-- +goose StatementBegin
-- O Descobrir deixa de rodar regex sobre o texto de todas as vagas a cada requisição.
--
-- techs: tecnologias do catálogo (discovery.Technologies) citadas na vaga, calculadas
-- na varredura com as mesmas regras de contexto da tela ("go-to" não conta como Go).
-- Filtra com índice (techs && skills) e é a stack que o JobFit usa no ranking.
--
-- techs_version: versão do catálogo e das regras com que techs foi calculada. Quando
-- muda (discovery.TechsVersion), o serviço recalcula as vagas antigas em lotes.
--
-- language: idioma estimado da vaga (discovery.GuessLanguage), calculado junto.
--
-- search_vec: texto da vaga indexado para as skills do perfil que não estão no
-- catálogo ("APIs REST", "Arquitetura Hexagonal"): busca de frase com índice.
ALTER TABLE discovered_jobs
    ADD COLUMN IF NOT EXISTS techs TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS techs_version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS language TEXT NOT NULL DEFAULT '',
    -- A barra vira espaço: o parser trata "Scrum/Kanban" como um token só (caminho de
    -- arquivo) e a busca por "scrum" não acharia.
    ADD COLUMN IF NOT EXISTS search_vec tsvector
        GENERATED ALWAYS AS (to_tsvector('simple'::regconfig, translate(title || ' ' || description, '/\', '  '))) STORED;

CREATE INDEX IF NOT EXISTS idx_discovered_jobs_techs ON discovered_jobs USING GIN (techs);
CREATE INDEX IF NOT EXISTS idx_discovered_jobs_search ON discovered_jobs USING GIN (search_vec);
-- Recálculo em lotes das vagas com versão antiga.
CREATE INDEX IF NOT EXISTS idx_discovered_jobs_techs_version ON discovered_jobs (techs_version);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_discovered_jobs_techs_version;
DROP INDEX IF EXISTS idx_discovered_jobs_search;
DROP INDEX IF EXISTS idx_discovered_jobs_techs;
ALTER TABLE discovered_jobs
    DROP COLUMN IF EXISTS search_vec,
    DROP COLUMN IF EXISTS language,
    DROP COLUMN IF EXISTS techs_version,
    DROP COLUMN IF EXISTS techs;
-- +goose StatementEnd
