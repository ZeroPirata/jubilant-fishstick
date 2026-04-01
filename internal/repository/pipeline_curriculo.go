package repository

import (
	"context"
	"fmt"
	"hackton-treino/internal/db"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dbRepository struct {
	conn *pgxpool.Pool
	q    *db.Queries
}

type Repository interface {
	ExecTx(ctx context.Context, fn func(*db.Queries) error) *AppError
	QueryFindVagaByUrl(ctx context.Context, url string) (int64, *AppError)
	QueryInsertVagas(ctx context.Context, arg db.QueryInsertVagaParams) *AppError
	QueryFindVagasPendentes(ctx context.Context) ([]db.Vaga, *AppError)
	QueryFindVagasPendentesLimit(ctx context.Context, limit int32) ([]db.Vaga, *AppError)
	QueryUpdateVagaStatusToProcessando(ctx context.Context, id int64) *AppError
	QueryUpdateVagaStatus(ctx context.Context, params db.QueryUpdateVagaStatusParams) *AppError
	QuerySelectAllFiltros(ctx context.Context) ([]string, *AppError)
	QuerySelectExperienciasByTags(ctx context.Context, tags []string) ([]db.Experiencia, *AppError)
	QuerySelectHabilidadesByTags(ctx context.Context, tags []string) ([]db.Habilidade, *AppError)
	QuerySelectProjetosByTags(ctx context.Context, tags []string) ([]db.Projeto, *AppError)
	QuerySelectBasicInfo(ctx context.Context) (db.InformacoesBasica, *AppError)
	QueryInsertCurriculoGerado(ctx context.Context, arg db.QueryInsertCurriculoGeradoParams) *AppError
	QueryDeleteVaga(ctx context.Context, id int64) *AppError
	QueryListarVagas(ctx context.Context, arg db.QueryListarVagasParams) ([]db.Vaga, *AppError)
	QueryListarCurriculos(ctx context.Context, arg db.QueryListarCurriculosParams) ([]db.QueryListarCurriculosRow, *AppError)
	QueryInsertFeedback(ctx context.Context, arg db.QueryInsertFeedbackParams) *AppError
	QueryDeleteCurriculoGerado(ctx context.Context, id int64) *AppError
	QueryUpdateVagaStatusOnly(ctx context.Context, arg db.QueryUpdateVagaStatusOnlyParams) *AppError
	QueryUpsertInformacoesBasicas(ctx context.Context, arg db.QueryUpsertInformacoesBasicasParams) (db.InformacoesBasica, *AppError)
	QuerySelectAllExperiencias(ctx context.Context) ([]db.Experiencia, *AppError)
	QueryInsertExperiencia(ctx context.Context, arg db.QueryInsertExperienciaParams) (db.Experiencia, *AppError)
	QueryDeleteExperiencia(ctx context.Context, id int64) *AppError
	QuerySelectAllHabilidades(ctx context.Context) ([]db.Habilidade, *AppError)
	QueryInsertHabilidade(ctx context.Context, arg db.QueryInsertHabilidadeParams) (db.Habilidade, *AppError)
	QueryDeleteHabilidade(ctx context.Context, id int64) *AppError
	QuerySelectAllProjetos(ctx context.Context) ([]db.Projeto, *AppError)
	QueryInsertProjeto(ctx context.Context, arg db.QueryInsertProjetoParams) (db.Projeto, *AppError)
	QueryDeleteProjeto(ctx context.Context, id int64) *AppError
	QuerySelectAllFiltrosWithID(ctx context.Context) ([]db.Filtro, *AppError)
	QueryInsertFiltro(ctx context.Context, keyword string) (db.Filtro, *AppError)
	QueryDeleteFiltro(ctx context.Context, id int64) *AppError
	QuerySelectAllFormacoes(ctx context.Context) ([]db.Formacao, *AppError)
	QuerySelectFeedbackMidGood(ctx context.Context) ([]pgtype.Text, *AppError)
	QuerySelectCurriculoExcelente(ctx context.Context) ([][]byte, *AppError)
	QueryGetCurriculoComVaga(ctx context.Context, id int64) (db.QueryGetCurriculoComVagaRow, *AppError)
	QueryUpdateCurriculoPaths(ctx context.Context, arg db.QueryUpdateCurriculoPathsParams) *AppError
	QuerySelectAllCertificacoes(ctx context.Context) ([]db.Certificaco, *AppError)
	QueryInsertCertificacao(ctx context.Context, arg db.QueryInsertCertificacaoParams) (db.Certificaco, *AppError)
	QueryDeleteCertificacao(ctx context.Context, id int64) *AppError
}

func NewRepository(conn *pgxpool.Pool) Repository {
	return &dbRepository{
		conn: conn,
		q:    db.New(conn),
	}
}

func (r *dbRepository) ExecTx(ctx context.Context, fn func(*db.Queries) error) *AppError {
	tx, err := r.conn.Begin(ctx)
	if err != nil {
		return HandleDatabaseError(err)
	}
	qtx := r.q.WithTx(tx)
	err = fn(qtx)
	if err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return &AppError{StatusCode: http.StatusInternalServerError, Message: fmt.Sprintf("tx err: %v, rb err: %v", err, rbErr)}
		}
		return HandleDatabaseError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return HandleDatabaseError(err)
	}
	return nil // success
}

func (r *dbRepository) QueryDeleteVaga(ctx context.Context, id int64) *AppError {
	return HandleDatabaseError(r.q.QueryDeleteVaga(ctx, id))
}

func (r *dbRepository) QueryInsertVagas(ctx context.Context, arg db.QueryInsertVagaParams) *AppError {
	return HandleDatabaseError(r.q.QueryInsertVaga(ctx, arg))
}

func (r *dbRepository) QueryFindVagaByUrl(ctx context.Context, url string) (int64, *AppError) {
	id, err := r.q.QueryFindVagaByUrl(ctx, url)
	return id, HandleDatabaseError(err)
}

func (r *dbRepository) QueryFindVagasPendentes(ctx context.Context) ([]db.Vaga, *AppError) {
	vagas, err := r.q.QueryFindVagasPendentes(ctx)
	return vagas, HandleDatabaseError(err)
}

func (r *dbRepository) QueryFindVagasPendentesLimit(ctx context.Context, limit int32) ([]db.Vaga, *AppError) {
	vagas, err := r.q.QueryFindVagasPendentesLimit(ctx, limit)
	return vagas, HandleDatabaseError(err)
}

func (r *dbRepository) QueryUpdateVagaStatusToProcessando(ctx context.Context, id int64) *AppError {
	return HandleDatabaseError(r.q.QueryUpdateVagaStatusToProcessando(ctx, id))
}

func (r *dbRepository) QueryUpdateVagaStatus(ctx context.Context, params db.QueryUpdateVagaStatusParams) *AppError {
	return HandleDatabaseError(r.q.QueryUpdateVagaStatus(ctx, params))
}

func (r *dbRepository) QuerySelectAllFiltros(ctx context.Context) ([]string, *AppError) {
	filtros, err := r.q.QuerySelectAllFiltros(ctx)
	return filtros, HandleDatabaseError(err)
}

func (r *dbRepository) QuerySelectExperienciasByTags(ctx context.Context, tags []string) ([]db.Experiencia, *AppError) {
	experiencias, err := r.q.QuerySelectExperienciasByTags(ctx, tags)
	return experiencias, HandleDatabaseError(err)
}

func (r *dbRepository) QuerySelectHabilidadesByTags(ctx context.Context, tags []string) ([]db.Habilidade, *AppError) {
	habilidades, err := r.q.QuerySelectHabilidadesByTags(ctx, tags)
	return habilidades, HandleDatabaseError(err)
}

func (r *dbRepository) QuerySelectProjetosByTags(ctx context.Context, tags []string) ([]db.Projeto, *AppError) {
	projetos, err := r.q.QuerySelectProjetosByTags(ctx, tags)
	return projetos, HandleDatabaseError(err)
}

func (r *dbRepository) QuerySelectBasicInfo(ctx context.Context) (db.InformacoesBasica, *AppError) {
	informacoes, err := r.q.QuerySelectBasicInfo(ctx)
	return informacoes, HandleDatabaseError(err)
}

func (r *dbRepository) QueryInsertCurriculoGerado(ctx context.Context, arg db.QueryInsertCurriculoGeradoParams) *AppError {
	err := r.q.QueryInsertCurriculoGerado(ctx, arg)
	return HandleDatabaseError(err)
}

func (r *dbRepository) QueryListarVagas(ctx context.Context, arg db.QueryListarVagasParams) ([]db.Vaga, *AppError) {
	vagas, err := r.q.QueryListarVagas(ctx, arg)
	return vagas, HandleDatabaseError(err)
}

func (r *dbRepository) QueryListarCurriculos(ctx context.Context, arg db.QueryListarCurriculosParams) ([]db.QueryListarCurriculosRow, *AppError) {
	rows, err := r.q.QueryListarCurriculos(ctx, arg)
	return rows, HandleDatabaseError(err)
}

func (r *dbRepository) QueryInsertFeedback(ctx context.Context, arg db.QueryInsertFeedbackParams) *AppError {
	return HandleDatabaseError(r.q.QueryInsertFeedback(ctx, arg))
}

func (r *dbRepository) QueryDeleteCurriculoGerado(ctx context.Context, id int64) *AppError {
	return HandleDatabaseError(r.q.QueryDeleteCurriculoGerado(ctx, id))
}

func (r *dbRepository) QueryUpdateVagaStatusOnly(ctx context.Context, arg db.QueryUpdateVagaStatusOnlyParams) *AppError {
	return HandleDatabaseError(r.q.QueryUpdateVagaStatusOnly(ctx, arg))
}

func (r *dbRepository) QueryUpsertInformacoesBasicas(ctx context.Context, arg db.QueryUpsertInformacoesBasicasParams) (db.InformacoesBasica, *AppError) {
	info, err := r.q.QueryUpsertInformacoesBasicas(ctx, arg)
	return info, HandleDatabaseError(err)
}

func (r *dbRepository) QuerySelectAllExperiencias(ctx context.Context) ([]db.Experiencia, *AppError) {
	rows, err := r.q.QuerySelectAllExperiencias(ctx)
	return rows, HandleDatabaseError(err)
}

func (r *dbRepository) QueryInsertExperiencia(ctx context.Context, arg db.QueryInsertExperienciaParams) (db.Experiencia, *AppError) {
	row, err := r.q.QueryInsertExperiencia(ctx, arg)
	return row, HandleDatabaseError(err)
}

func (r *dbRepository) QueryDeleteExperiencia(ctx context.Context, id int64) *AppError {
	return HandleDatabaseError(r.q.QueryDeleteExperiencia(ctx, id))
}

func (r *dbRepository) QuerySelectAllHabilidades(ctx context.Context) ([]db.Habilidade, *AppError) {
	rows, err := r.q.QuerySelectAllHabilidades(ctx)
	return rows, HandleDatabaseError(err)
}

func (r *dbRepository) QueryInsertHabilidade(ctx context.Context, arg db.QueryInsertHabilidadeParams) (db.Habilidade, *AppError) {
	row, err := r.q.QueryInsertHabilidade(ctx, arg)
	return row, HandleDatabaseError(err)
}

func (r *dbRepository) QueryDeleteHabilidade(ctx context.Context, id int64) *AppError {
	return HandleDatabaseError(r.q.QueryDeleteHabilidade(ctx, id))
}

func (r *dbRepository) QuerySelectAllProjetos(ctx context.Context) ([]db.Projeto, *AppError) {
	rows, err := r.q.QuerySelectAllProjetos(ctx)
	return rows, HandleDatabaseError(err)
}

func (r *dbRepository) QueryInsertProjeto(ctx context.Context, arg db.QueryInsertProjetoParams) (db.Projeto, *AppError) {
	row, err := r.q.QueryInsertProjeto(ctx, arg)
	return row, HandleDatabaseError(err)
}

func (r *dbRepository) QueryDeleteProjeto(ctx context.Context, id int64) *AppError {
	return HandleDatabaseError(r.q.QueryDeleteProjeto(ctx, id))
}

func (r *dbRepository) QuerySelectAllFiltrosWithID(ctx context.Context) ([]db.Filtro, *AppError) {
	rows, err := r.q.QuerySelectAllFiltrosWithID(ctx)
	return rows, HandleDatabaseError(err)
}

func (r *dbRepository) QueryInsertFiltro(ctx context.Context, keyword string) (db.Filtro, *AppError) {
	row, err := r.q.QueryInsertFiltro(ctx, keyword)
	return row, HandleDatabaseError(err)
}

func (r *dbRepository) QueryDeleteFiltro(ctx context.Context, id int64) *AppError {
	return HandleDatabaseError(r.q.QueryDeleteFiltro(ctx, id))
}

func (r *dbRepository) QuerySelectAllFormacoes(ctx context.Context) ([]db.Formacao, *AppError) {
	rows, err := r.q.QuerySelectAllFormacoes(ctx)
	return rows, HandleDatabaseError(err)
}

func (r *dbRepository) QuerySelectFeedbackMidGood(ctx context.Context) ([]pgtype.Text, *AppError) {
	feedback, err := r.q.QuerySelectFeedbackMidGood(ctx)
	return feedback, HandleDatabaseError(err)
}

func (r *dbRepository) QuerySelectCurriculoExcelente(ctx context.Context) ([][]byte, *AppError) {
	feedback, err := r.q.QuerySelectCurriculoExcelente(ctx)
	return feedback, HandleDatabaseError(err)
}

func (r *dbRepository) QueryGetCurriculoComVaga(ctx context.Context, id int64) (db.QueryGetCurriculoComVagaRow, *AppError) {
	row, err := r.q.QueryGetCurriculoComVaga(ctx, id)
	return row, HandleDatabaseError(err)
}

func (r *dbRepository) QueryUpdateCurriculoPaths(ctx context.Context, arg db.QueryUpdateCurriculoPathsParams) *AppError {
	return HandleDatabaseError(r.q.QueryUpdateCurriculoPaths(ctx, arg))
}

func (r *dbRepository) QuerySelectAllCertificacoes(ctx context.Context) ([]db.Certificaco, *AppError) {
	rows, err := r.q.QuerySelectAllCertificacoes(ctx)
	return rows, HandleDatabaseError(err)
}

func (r *dbRepository) QueryInsertCertificacao(ctx context.Context, arg db.QueryInsertCertificacaoParams) (db.Certificaco, *AppError) {
	row, err := r.q.QueryInsertCertificacao(ctx, arg)
	return row, HandleDatabaseError(err)
}

func (r *dbRepository) QueryDeleteCertificacao(ctx context.Context, id int64) *AppError {
	return HandleDatabaseError(r.q.QueryDeleteCertificacao(ctx, id))
}
