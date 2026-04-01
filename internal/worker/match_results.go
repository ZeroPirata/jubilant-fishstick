package worker

import (
	"context"
	"errors"
	"hackton-treino/internal/db"
	"hackton-treino/internal/scraper"
	"hackton-treino/internal/services"

	"go.uber.org/zap"
)

func (w *Worker) matchComBancoPessoal(ctx context.Context, result *scraper.ScraperResult, vaga *db.Vaga) (services.MatchResult, error) {
	keywordsVaga := w.extrairKeywordsDaVaga(result)

	experiencias, err := w.Repository.QuerySelectExperienciasByTags(ctx, keywordsVaga)
	if err != nil {
		w.Logger.Error("Erro ao buscar experiências relacionadas às keywords da vaga", zap.Int64("id", vaga.ID), zap.Error(err))
		return services.MatchResult{}, errors.New("erro ao buscar experiências relacionada às keywords da vaga")
	}

	habilidades, err := w.Repository.QuerySelectHabilidadesByTags(ctx, keywordsVaga)
	if err != nil {
		w.Logger.Error("Erro ao buscar habilidades relacionadas às keywords da vaga", zap.Int64("id", vaga.ID), zap.Error(err))
		return services.MatchResult{}, errors.New("erro ao buscar habilidades relacionada às keywords da vaga")
	}

	projetos, err := w.Repository.QuerySelectProjetosByTags(ctx, keywordsVaga)
	if err != nil {
		w.Logger.Error("Erro ao buscar projetos relacionados às keywords da vaga", zap.Int64("id", vaga.ID), zap.Error(err))
		return services.MatchResult{}, errors.New("erro ao buscar projetos relacionados às keywords da vaga")
	}

	formacoes, err := w.Repository.QuerySelectAllFormacoes(ctx)
	if err != nil {
		w.Logger.Error("Erro ao buscar formações acadêmicas", zap.Int64("id", vaga.ID), zap.Error(err))
		return services.MatchResult{}, errors.New("erro ao buscar formações acadêmicas")
	}

	// certificacoes, err := w.Repository.QuerySelectAllCertificacoes(ctx)
	// if err != nil {
	// 	w.Logger.Error("Erro ao buscar certificações", zap.Int64("id", vaga.ID), zap.Error(err))
	// }

	excelentes, err := w.Repository.QuerySelectCurriculoExcelente(ctx)
	if err != nil {
		w.Logger.Error("Erro ao buscar curriculos excelentes", zap.Int64("id", vaga.ID), zap.Error(err))
	}

	feedback, err := w.Repository.QuerySelectFeedbackMidGood(ctx)
	if err != nil {
		w.Logger.Error("Erro ao buscar feedbacks", zap.Int64("id", vaga.ID), zap.Error(err))
	}

	return services.MatchResult{
		Experiencias: experiencias,
		Habilidades:  habilidades,
		Projetos:     projetos,
		Formacoes:    formacoes,
		Excelentes:   excelentes,
		Feedbacks:    feedback,
		// Certificacoes: certificacoes,
	}, nil

}
