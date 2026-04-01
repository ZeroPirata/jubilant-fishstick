-- name: QueryInsertAlert :exec
INSERT INTO alerta (sensor_id, status) VALUES ($1, $2);

-- name: QueryInsertAlertHistory :exec
INSERT INTO historico (sensor_id, value) VALUES ($1, $2);

-- name: QueryUpdateSensorStatus :exec
UPDATE sensores SET status = $1 WHERE id = $2;

-- name: QueryInsertVaga :exec
INSERT INTO vagas (url, titulo, descricao, empresa, stack, requisitos, idioma) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: QueryFindVagaByUrl :one
SELECT id FROM vagas WHERE url = $1;

-- name: QueryFindVagasPendentes :many
SELECT * FROM vagas where status = 'pendente';

-- name: QueryFindVagasPendentesLimit :many
SELECT * FROM vagas where status = 'pendente' ORDER BY id LIMIT $1;

-- name: QueryUpdateVagaStatusToProcessando :exec
UPDATE vagas SET status = 'processando' WHERE id = $1;

-- name: QueryUpdateVagaStatus :exec
UPDATE vagas SET 
    status = $2,
    empresa = $3,
    titulo = $4,
    stack = $5, 
    requisitos = $6,
    descricao = $7
WHERE id = $1;

-- name: QuerySelectAllFiltros :many
SELECT keyword FROM filtros;

-- name: QuerySelectBasicInfo :one
select * from public.informacoes_basicas order by criado_em desc limit 1;

-- name: QuerySelectExperienciasByTags :many
SELECT * FROM experiencias WHERE tags && $1::TEXT[] ORDER BY data_inicio DESC;

-- name: QuerySelectHabilidadesByTags :many
SELECT * FROM habilidades WHERE tags && $1::TEXT[];

-- name: QuerySelectProjetosByTags :many
SELECT * FROM projetos WHERE tags && $1::TEXT[];

-- name: QueryInsertCurriculoGerado :exec
INSERT INTO curriculos_gerados(vaga_id, conteudo_json) VALUES(@vaga_id, @curriculo_gerado::JSONB);

-- name: QueryDeleteVaga :exec
DELETE FROM vagas WHERE id = $1;

-- name: QueryListarVagas :many
SELECT * FROM public.vagas WHERE (@status = '' OR status::text = @status)
ORDER BY criado_em DESC LIMIT @tamanho OFFSET @pagina;

-- name: QueryListarCurriculos :many
SELECT cg.id, cg.vaga_id, cg.conteudo_json, cg.criado_em,
       v.empresa, v.titulo, v.url
FROM curriculos_gerados cg
JOIN vagas v ON v.id = cg.vaga_id
ORDER BY cg.criado_em DESC
LIMIT $1 OFFSET $2;

-- name: QueryInsertFeedback :exec
INSERT INTO feedback (curriculo_id, vaga_id, status, comentario)
VALUES ($1, $2, $3, $4);

-- name: QueryDeleteCurriculoGerado :exec
DELETE FROM curriculos_gerados WHERE id = $1;

-- name: QueryUpdateVagaStatusOnly :exec
UPDATE vagas SET status = $2 WHERE id = $1;

-- name: QueryUpsertInformacoesBasicas :one
INSERT INTO informacoes_basicas(id, nome, email, telefone, linkedin, github, portfolio, resumo)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET
    nome      = EXCLUDED.nome,
    email     = EXCLUDED.email,
    telefone  = EXCLUDED.telefone,
    linkedin  = EXCLUDED.linkedin,
    github    = EXCLUDED.github,
    portfolio = EXCLUDED.portfolio,
    resumo    = EXCLUDED.resumo
RETURNING *;

-- name: QuerySelectAllExperiencias :many
SELECT * FROM experiencias ORDER BY data_inicio DESC;

-- name: QueryInsertExperiencia :one
INSERT INTO experiencias(empresa, cargo, descricao, atual, data_inicio, data_fim, stack, conquistas, tags)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: QueryDeleteExperiencia :exec
DELETE FROM experiencias WHERE id = $1;

-- name: QuerySelectAllHabilidades :many
SELECT * FROM habilidades ORDER BY nome;

-- name: QueryInsertHabilidade :one
INSERT INTO habilidades(nome, nivel, tags)
VALUES ($1, $2, $3::TEXT[])
RETURNING *;

-- name: QueryDeleteHabilidade :exec
DELETE FROM habilidades WHERE id = $1;

-- name: QuerySelectAllProjetos :many
SELECT * FROM projetos ORDER BY data_inicio DESC;

-- name: QueryInsertProjeto :one
INSERT INTO projetos(nome, descricao, link, tags, data_inicio, data_fim, facultativo)
VALUES ($1, $2, $3, $4::TEXT[], $5, $6, $7)
RETURNING *;

-- name: QueryDeleteProjeto :exec
DELETE FROM projetos WHERE id = $1;

-- name: QuerySelectAllFiltrosWithID :many
SELECT id, keyword FROM filtros ORDER BY keyword;

-- name: QueryInsertFiltro :one
INSERT INTO filtros(keyword) VALUES ($1) RETURNING *;

-- name: QueryDeleteFiltro :exec
DELETE FROM filtros WHERE id = $1;

-- name: QuerySelectAllFormacoes :many
SELECT * FROM formacao ORDER BY data_inicio DESC;

-- name: QuerySelectFeedbackMidGood :many
SELECT comentario FROM feedback 
WHERE status IN ('medio', 'bom') 
AND comentario IS NOT NULL
ORDER BY criado_em DESC 
LIMIT 5;

-- name: QueryGetCurriculoComVaga :one
SELECT cg.id, cg.conteudo_json, cg.resume_path, cg.cover_letter_path,
       v.empresa, v.titulo, v.idioma
FROM curriculos_gerados cg
JOIN vagas v ON v.id = cg.vaga_id
WHERE cg.id = $1;

-- name: QueryUpdateCurriculoPaths :exec
UPDATE curriculos_gerados
SET resume_path = $2, cover_letter_path = $3
WHERE id = $1;

-- name: QuerySelectCurriculoExcelente :many
SELECT cg.conteudo_json FROM feedback f
JOIN curriculos_gerados cg ON cg.id = f.curriculo_id
WHERE f.status = 'excelente'
ORDER BY f.criado_em DESC
LIMIT 3;

-- name: QuerySelectAllCertificacoes :many
SELECT * FROM certificacoes ORDER BY emitido_em DESC;

-- name: QueryInsertCertificacao :one
INSERT INTO certificacoes(nome, emissor, emitido_em, codigo, link, tags)
VALUES ($1, $2, $3, $4, $5, $6::TEXT[])
RETURNING *;

-- name: QueryDeleteCertificacao :exec
DELETE FROM certificacoes WHERE id = $1;
