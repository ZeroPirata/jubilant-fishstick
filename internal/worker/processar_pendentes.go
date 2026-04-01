package worker

import (
	"context"
	"encoding/json"
	"hackton-treino/internal/db"
	"hackton-treino/internal/scraper"
	"hackton-treino/internal/util"
	"strings"

	"go.uber.org/zap"
)

func (w *Worker) processarPendentes(ctx context.Context) {
	const batchSize int32 = 20
	vagas, err := w.Repository.QueryFindVagasPendentesLimit(ctx, batchSize)
	if err != nil {
		w.Logger.Error("Erro ao buscar vagas pendentes", zap.Error(err))
		return
	}

	w.Logger.Info("Quantidade de vagas pendentes", zap.Int("quantity", len(vagas)))

	for _, vaga := range vagas {
		w.processarVaga(ctx, &vaga)
	}
}

func (w *Worker) processarVaga(ctx context.Context, vaga *db.Vaga) {
	w.Logger.Info("Processando vaga", zap.Int64("id", vaga.ID))

	err := w.Repository.QueryUpdateVagaStatusToProcessando(ctx, vaga.ID)
	if err != nil {
		w.Logger.Error("Erro ao atualizar status da vaga para processando", zap.Int64("id", vaga.ID), zap.Error(err))
		return
	}

	newScraper := scraper.NewScraper(vaga.Url, w.Logger)
	result, scrapeErr := newScraper.Scrape()
	if scrapeErr != nil {
		w.Logger.Error("Erro ao fazer scrape da vaga", zap.Int64("id", vaga.ID), zap.Error(scrapeErr))
		if statusErr := w.Repository.QueryUpdateVagaStatus(ctx, db.QueryUpdateVagaStatusParams{Status: db.StatusVagaImprocessavel, ID: vaga.ID}); statusErr != nil {
			w.Logger.Error("Erro ao atualizar status da vaga para falha", zap.Int64("id", vaga.ID), zap.Error(statusErr))
		}
		return
	}

	params := db.QueryUpdateVagaStatusParams{
		Status:     db.StatusVagaProcessando,
		ID:         vaga.ID,
		Empresa:    util.ConvertToPgText(result.Company),
		Titulo:     util.ConvertToPgText(result.Title),
		Stack:      util.ConvertToPgTextArray(result.Stack),
		Requisitos: util.ConvertToPgTextArray(result.Requirements),
		Descricao:  util.ConvertToPgText(result.Description),
	}

	if statusErr := w.Repository.QueryUpdateVagaStatus(ctx, params); statusErr != nil {
		w.Logger.Error("Erro ao atualizar status da vaga para gerado", zap.Int64("id", vaga.ID), zap.Error(statusErr))
		return
	}

	relevante := w.isVagaRelevante(&result)
	if !relevante {
		w.Logger.Info("Vaga descartada: não atingiu os requisitos dos filtros", zap.Int64("id", vaga.ID))
		if err := w.Repository.QueryUpdateVagaStatusOnly(ctx, db.QueryUpdateVagaStatusOnlyParams{ID: vaga.ID, Status: db.StatusVagaForaDoPerfil}); err != nil {
			w.Logger.Error("Erro ao atualizar status da vaga para fora do perfil", zap.Int64("id", vaga.ID), zap.Error(err))
			return
		}
		return
	}

	matches, errM := w.matchComBancoPessoal(ctx, &result, vaga)
	if errM != nil {
		w.Logger.Error("Erro ao fazer o processamento da vaga", zap.Int64("id", vaga.ID), zap.Error(errM))
		if statusErr := w.Repository.QueryUpdateVagaStatusOnly(ctx, db.QueryUpdateVagaStatusOnlyParams{ID: vaga.ID, Status: db.StatusVagaImprocessavel}); statusErr != nil {
			w.Logger.Error("Erro ao atualizar status da vaga", zap.Int64("id", vaga.ID), zap.Error(statusErr))
			return
		}
		return
	}

	str, errJ := w.buildUserPrompt(vaga, &matches)
	if errJ != nil {
		w.Logger.Error("Erro ao fazer o json para enviar no prompt", zap.Int64("id", vaga.ID), zap.Error(errJ))
		if statusErr := w.Repository.QueryUpdateVagaStatusOnly(ctx, db.QueryUpdateVagaStatusOnlyParams{ID: vaga.ID, Status: db.StatusVagaImprocessavel}); statusErr != nil {
			w.Logger.Error("Erro ao atualizar status da vaga", zap.Int64("id", vaga.ID), zap.Error(statusErr))
			return
		}
		return
	}

	promptPath := promptPTBRPath
	if vaga.Idioma.String == "en" {
		promptPath = promptENPath
		w.Logger.Warn("Idioma da vaga não informado, usando PT-BR como default", zap.Int64("id", vaga.ID))
	} else if vaga.Idioma.String != "ptbr" {
		w.Logger.Warn("Idioma da vaga não informado, usando PT-BR como default", zap.Int64("id", vaga.ID))
	}

	prompt, errP := w.loadPrompt(promptPath)
	if errP != nil {
		w.Logger.Error("Erro ao carregar system prompt", zap.String("path", promptPath), zap.Error(errP))
		if statusErr := w.Repository.QueryUpdateVagaStatusOnly(ctx, db.QueryUpdateVagaStatusOnlyParams{ID: vaga.ID, Status: db.StatusVagaImprocessavel}); statusErr != nil {
			w.Logger.Error("Erro ao atualizar status da vaga", zap.Int64("id", vaga.ID), zap.Error(statusErr))
		}
		return
	}

	llmResponse, errLLM := w.LLM.GerarCurriculo(ctx, prompt, str)
	if errLLM != nil {
		w.Logger.Error("Erro ao chamar LLM", zap.Int64("id", vaga.ID), zap.Error(errLLM))
		if statusErr := w.Repository.QueryUpdateVagaStatusOnly(ctx, db.QueryUpdateVagaStatusOnlyParams{ID: vaga.ID, Status: db.StatusVagaImprocessavel}); statusErr != nil {
			w.Logger.Error("Erro ao atualizar status da vaga", zap.Int64("id", vaga.ID), zap.Error(statusErr))
			return
		}
		return
	}

	replacer := strings.NewReplacer(
		"{{CANDIDATO_NOME}}", w.inf.Nome,
		"{{CANDIDATO_EMAIL}}", w.inf.Email,
		"{{CANDIDATO_LINKEDIN}}", w.inf.Linkedin.String,
		"{{CANDIDATO_GITHUB}}", w.inf.Github.String,
		"{{CANDIDATO_PORTFOLIO}}", w.inf.Portfolio.String,
		"{{CANDIDATO_TELEFONE}}", w.inf.Telefone.String,
		"{{VAGA_EMPRESA}}", vaga.Empresa.String,
		"{{VAGA_TITULO}}", vaga.Titulo.String,
	)

	conteudo := struct {
		Curriculo   string `json:"curriculo"`
		CoverLetter string `json:"cover_letter"`
	}{
		Curriculo:   replacer.Replace(llmResponse.Curriculo),
		CoverLetter: replacer.Replace(llmResponse.CoverLetter),
	}

	conteudoJSON, errJSON := json.Marshal(conteudo)
	if errJSON != nil {
		if statusErr := w.Repository.QueryUpdateVagaStatusOnly(ctx, db.QueryUpdateVagaStatusOnlyParams{ID: vaga.ID, Status: db.StatusVagaImprocessavel}); statusErr != nil {
			w.Logger.Error("Erro ao atualizar status da vaga", zap.Int64("id", vaga.ID), zap.Error(statusErr))
			return
		}
		w.Logger.Error("Erro ao serializar conteudo do curriculo", zap.Int64("id", vaga.ID), zap.Error(errJSON))
		return
	}

	paramsCurriculo := db.QueryInsertCurriculoGeradoParams{
		VagaID:          vaga.ID,
		CurriculoGerado: conteudoJSON,
	}

	err = w.Repository.QueryInsertCurriculoGerado(ctx, paramsCurriculo)
	if err != nil {
		w.Logger.Error("Não foi possivel inserir o curriculo gerado", zap.Int64("id", vaga.ID), zap.Error(err))
		if statusErr := w.Repository.QueryUpdateVagaStatusOnly(ctx, db.QueryUpdateVagaStatusOnlyParams{ID: vaga.ID, Status: db.StatusVagaImprocessavel}); statusErr != nil {
			w.Logger.Error("Erro ao atualizar status da vaga", zap.Int64("id", vaga.ID), zap.Error(statusErr))
			return
		}
		return
	}

	if statusErr := w.Repository.QueryUpdateVagaStatusOnly(ctx, db.QueryUpdateVagaStatusOnlyParams{ID: vaga.ID, Status: db.StatusVagaGerado}); statusErr != nil {
		w.Logger.Error("Erro ao atualizar status da vaga para gerado", zap.Int64("id", vaga.ID), zap.Error(statusErr))
		return
	}

	w.Logger.Info("Vaga processada com sucesso", zap.String("url", vaga.Url))
}
