-- +goose Up
-- +goose StatementBegin
-- Primeiras empresas no Workable e no Recruitee, conferidas em 2026-10-31 (vagas no
-- quadro / vagas que entram no catálogo): Blueground 9/2, Platzi 25/1, RemotePass 75/1,
-- Netguru 23/0 e bunq 25/0. Pouca coisa no Brasil ou remota para cá: as que estão com 0
-- entram para a fonte ser lida e pegar as vagas que abrirem. Só acrescenta, como a 00017.
WITH new(source, boards) AS (VALUES
  ('workable', '[
    {"token": "blueground", "company": "Blueground"},
    {"token": "platzi",     "company": "Platzi"},
    {"token": "remotepass", "company": "RemotePass"},
    {"token": "netguru",    "company": "Netguru"}
  ]'::jsonb),
  ('recruitee', '[{"token": "bunq", "company": "bunq"}]'::jsonb)
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
