-- +goose Up
-- +goose StatementBegin
-- Vaga de empresa de fora (remota, aceitando candidatos no Brasil): entra em "Exterior".
-- Antes era deduzido pela fonte; agora cada fonte decide por vaga.
ALTER TABLE discovered_jobs ADD COLUMN IF NOT EXISTS international BOOLEAN NOT NULL DEFAULT false;
UPDATE discovered_jobs SET international = true WHERE source = 'himalayas' AND location <> 'Brasil';

-- Empresas lidas nas fontes de ATS (Greenhouse, Lever): {"greenhouse": [{"token": "stone", "company": "Stone"}]}.
-- O token é o da URL pública do quadro de vagas da empresa.
ALTER TABLE discovery_settings ADD COLUMN IF NOT EXISTS boards JSONB NOT NULL DEFAULT '{}';
UPDATE discovery_settings SET boards = '{
  "greenhouse": [
    {"token": "stone",           "company": "Stone"},
    {"token": "c6bank",          "company": "C6 Bank"},
    {"token": "xpinc",           "company": "XP Inc."},
    {"token": "quintoandar",     "company": "QuintoAndar"},
    {"token": "gympass",         "company": "Wellhub (Gympass)"},
    {"token": "ebanx",           "company": "EBANX"},
    {"token": "vtex",            "company": "VTEX"},
    {"token": "wildlifestudios", "company": "Wildlife Studios"},
    {"token": "rdstation",       "company": "RD Station"}
  ],
  "lever": [
    {"token": "ciandt", "company": "CI&T"},
    {"token": "neon",   "company": "Neon"}
  ]
}'::jsonb
WHERE boards = '{}'::jsonb;

-- partial: varredura do painel só com alguns sites ou só com o catálogo. Não conta
-- como "última varredura completa" para o agendador.
-- sources: os sites escolhidos (vazio = todos os ligados).
-- jobs_new: vagas que não existiam no catálogo (jobs_saved conta também as atualizadas).
ALTER TABLE discovery_runs
    ADD COLUMN IF NOT EXISTS partial  BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS sources  TEXT[]  NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS jobs_new INT     NOT NULL DEFAULT 0;

-- kind: search (um passo por termo) | list (baixa a lista inteira do site; os passos
-- são as listas/empresas lidas).
ALTER TABLE discovery_run_sources
    ADD COLUMN IF NOT EXISTS kind     TEXT NOT NULL DEFAULT 'search' CHECK (kind IN ('search', 'list')),
    ADD COLUMN IF NOT EXISTS jobs_new INT  NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE discovery_run_sources DROP COLUMN IF EXISTS kind, DROP COLUMN IF EXISTS jobs_new;
ALTER TABLE discovery_runs DROP COLUMN IF EXISTS partial, DROP COLUMN IF EXISTS sources, DROP COLUMN IF EXISTS jobs_new;
ALTER TABLE discovery_settings DROP COLUMN IF EXISTS boards;
ALTER TABLE discovered_jobs DROP COLUMN IF EXISTS international;
-- +goose StatementEnd
