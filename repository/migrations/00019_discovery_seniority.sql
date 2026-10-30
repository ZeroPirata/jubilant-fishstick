-- +goose Up
-- +goose StatementBegin
-- Nível das vagas do catálogo (intern | junior | mid | senior | expert). Vazio quando
-- a vaga não informa. Quem preenche é discovery.ClassifySeniority a cada varredura; o
-- UPDATE abaixo só adianta as vagas que já existem, pelo título, com as mesmas regras.
ALTER TABLE discovered_jobs ADD COLUMN IF NOT EXISTS seniority TEXT[] NOT NULL DEFAULT '{}';

WITH t AS (
    SELECT id, regexp_replace(
        translate(lower(title), 'áàâãäéêèíîóôõöúüç', 'aaaaaeeeiioooouuc'),
        'pl[/ -]sql', 'plsql', 'g') AS ti
    FROM discovered_jobs
)
UPDATE discovered_jobs d SET seniority = array_remove(ARRAY[
    CASE WHEN t.ti ~ '\m(estagio|estagiario|estagiaria|intern|internship|trainee|aprendiz)\M' THEN 'intern' END,
    CASE WHEN t.ti ~ '\m(junior|jr|entry[ -]level)\M' THEN 'junior' END,
    CASE WHEN t.ti ~ '\m(pleno|plena|pl|mid[ -]level|mid-senior|ii)\M' THEN 'mid' END,
    CASE WHEN t.ti ~ '\m(senior|sr|iii)\M' THEN 'senior' END,
    CASE WHEN t.ti ~ '\m(especialista|staff|principal|lead|leader|lider|tech lead|arquiteto|arquiteta|architect)\M' THEN 'expert' END
], NULL)
FROM t WHERE t.id = d.id;

-- Filtro de nível da aba Descobrir. Sem nível marcado não filtra; seniority_unknown
-- deixa passar as vagas que não informam o nível (a maioria).
ALTER TABLE user_discovery_preferences
    ADD COLUMN IF NOT EXISTS seniorities TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS seniority_unknown BOOLEAN NOT NULL DEFAULT TRUE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_discovery_preferences
    DROP COLUMN IF EXISTS seniority_unknown,
    DROP COLUMN IF EXISTS seniorities;
ALTER TABLE discovered_jobs DROP COLUMN IF EXISTS seniority;
-- +goose StatementEnd
