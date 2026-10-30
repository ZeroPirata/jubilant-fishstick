-- +goose Up
-- +goose StatementBegin
-- Análise da IA sobre a aderência do candidato à vaga (resumo, pontos fortes, lacunas e
-- recomendação), escrita na mesma chamada que gera o currículo. NULL nas vagas
-- processadas antes disso ou quando a IA não devolveu a análise.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS fit_analysis JSONB;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE jobs DROP COLUMN IF EXISTS fit_analysis;
-- +goose StatementEnd
