package services

import (
	"hackton-treino/config"
	"hackton-treino/internal/db"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
)

type MatchResult struct {
	Experiencias  []db.Experiencia
	Habilidades   []db.Habilidade
	Projetos      []db.Projeto
	Formacoes     []db.Formacao
	Certificacoes []db.Certificaco
	Excelentes    [][]byte
	Feedbacks     []pgtype.Text
}

type LLMResponse struct {
	Curriculo   string `json:"curriculo"`
	CoverLetter string `json:"cover_letter"`
	Error       string `json:"error,omitempty"`
}

type AiService struct {
	Config     *config.Config
	httpClient *http.Client
}
