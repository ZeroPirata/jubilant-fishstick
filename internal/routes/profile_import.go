package routes

import (
	"hackton-treino/internal/handler"
	"hackton-treino/internal/profileimport"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/repository/aliases"
	ucache "hackton-treino/internal/repository/cache"
	"hackton-treino/internal/repository/users"
	"hackton-treino/internal/services"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// O envio do currículo é o único body acima do limite global de 1 MB.
const (
	ImportUploadPath  = "/api/v1/users/me/import"
	ImportUploadLimit = profileimport.MaxFileSize + 64<<10
)

func setupProfileImportRoutes(mux *http.ServeMux, logger *zap.Logger, db *pgxpool.Pool, rds *redis.Client, llm *services.AiService) {
	svc := &profileimport.Service{
		AI:      llm,
		Store:   profileimport.NewStore(rds),
		Users:   users.New(db),
		Aliases: aliases.New(db),
		Logger:  logger,
	}
	h := handler.NewProfileImportHandler(logger, svc, repository.NewBase(db), ucache.New(rds))

	mux.HandleFunc("POST /users/me/import", h.Upload)
	mux.HandleFunc("GET /users/me/import/{id}", h.Status)
	mux.HandleFunc("DELETE /users/me/import/{id}", h.Cancel)
	mux.HandleFunc("POST /users/me/import/apply", h.Apply)
}
