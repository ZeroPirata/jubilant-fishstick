-- +goose Up
-- +goose StatementBegin
-- Primeiras empresas nos ATS novos (Ashby, SmartRecruiters), conferidas nas APIs
-- públicas em 2026-10-30: Nubank com 35 vagas no Brasil, Experian com 208. Workable e
-- Recruitee entram sem empresas (nenhuma brasileira com vagas achada): o painel acha
-- e cadastra. Só acrescenta, como a 00017.
WITH new(source, boards) AS (VALUES
  ('ashby',           '[{"token": "nubank",   "company": "Nubank"}]'::jsonb),
  ('smartrecruiters', '[{"token": "experian", "company": "Experian"}]'::jsonb)
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
