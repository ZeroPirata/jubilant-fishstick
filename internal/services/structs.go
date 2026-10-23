package services

import (
	"hackton-treino/config"
	"hackton-treino/internal/db"
	"net/http"
)

// ErrRateLimit is returned when the LLM API responds with 429 after all retries are exhausted.
type ErrRateLimit struct{ Msg string }

func (e *ErrRateLimit) Error() string { return e.Msg }

type MatchResult struct {
	Experiencias  []db.UserExperience
	Habilidades   []db.UserSkill
	Projetos      []db.UserProject
	Formacoes     []db.UserAcademicHistory
	Certificacoes []db.UserCertificate
	Excelentes    [][]byte
	Feedbacks     []string
}


type AiService struct {
	Config           *config.Config
	Source           ProviderSource
	httpClient       *http.Client
	scrapeHttpClient *http.Client
}
