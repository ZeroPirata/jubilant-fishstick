-- +goose Up
-- +goose StatementBegin
-- Mais empresas nos ATS, todas conferidas com pelo menos 3 vagas no Brasil ou remotas
-- abertas para cá. Só acrescenta: empresa já cadastrada (ou removida e readicionada no
-- painel) fica como está.
WITH new(source, boards) AS (VALUES
  ('greenhouse', '[
    {"token": "btgpactual", "company": "BTG Pactual"},
    {"token": "inter",      "company": "Banco Inter"},
    {"token": "agibank",    "company": "Agibank"},
    {"token": "bancopan",   "company": "Banco PAN"},
    {"token": "getnet",     "company": "Getnet"},
    {"token": "sumup",      "company": "SumUp"},
    {"token": "abinbev",    "company": "AB InBev"},
    {"token": "jusbrasil",  "company": "Jusbrasil"},
    {"token": "datadog",    "company": "Datadog"},
    {"token": "airbnb",     "company": "Airbnb"},
    {"token": "databricks", "company": "Databricks"},
    {"token": "binance",    "company": "Binance"},
    {"token": "mongodb",    "company": "MongoDB"},
    {"token": "gitlab",     "company": "GitLab"},
    {"token": "stripe",     "company": "Stripe"},
    {"token": "figma",      "company": "Figma"},
    {"token": "elastic",    "company": "Elastic"},
    {"token": "coinbase",   "company": "Coinbase"}
  ]'::jsonb),
  ('lever', '[
    {"token": "despegar",  "company": "Despegar (Decolar)"},
    {"token": "flashapp",  "company": "Flash"},
    {"token": "swile",     "company": "Swile"},
    {"token": "bv",        "company": "Banco BV"},
    {"token": "toptal",    "company": "Toptal"},
    {"token": "yuno",      "company": "Yuno"},
    {"token": "starkbank", "company": "Stark Bank"}
  ]'::jsonb)
)
UPDATE discovery_settings st SET boards = (
    SELECT st.boards || jsonb_object_agg(
        n.source,
        COALESCE(st.boards -> n.source, '[]'::jsonb) || COALESCE((
            SELECT jsonb_agg(b) FROM jsonb_array_elements(n.boards) b
            WHERE NOT EXISTS (
                SELECT 1 FROM jsonb_array_elements(COALESCE(st.boards -> n.source, '[]'::jsonb)) old
                WHERE old ->> 'token' = b ->> 'token'
            )
        ), '[]'::jsonb)
    )
    FROM new n
)
WHERE st.id;
-- +goose StatementEnd

-- +goose Down
-- Nada a desfazer: as empresas podem ter sido editadas no painel.
SELECT 1;
