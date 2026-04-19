package routes

import (
	"hackton-treino/internal/handler"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func setupUserRoutes(mux *http.ServeMux, logger *zap.Logger, db *pgxpool.Pool, rds *redis.Client) {
	h := handler.NewUserHandler(logger, db, rds)

	mux.HandleFunc("GET /users/me", h.GetProfile)
	mux.HandleFunc("PUT /users/me/profile", h.UpsertProfile)
	mux.HandleFunc("PUT /users/me/links", h.UpsertLinks)

	mux.HandleFunc("GET /users/me/experiences", h.ListExperiences)
	mux.HandleFunc("POST /users/me/experiences", h.InsertExperience)
	mux.HandleFunc("PUT /users/me/experiences/{id}", h.UpdateExperience)
	mux.HandleFunc("DELETE /users/me/experiences/{id}", h.DeleteExperience)

	mux.HandleFunc("GET /users/me/academic", h.ListAcademic)
	mux.HandleFunc("POST /users/me/academic", h.InsertAcademic)
	mux.HandleFunc("PUT /users/me/academic/{id}", h.UpdateAcademic)
	mux.HandleFunc("DELETE /users/me/academic/{id}", h.DeleteAcademic)

	mux.HandleFunc("GET /users/me/skills", h.ListSkills)
	mux.HandleFunc("POST /users/me/skills", h.InsertSkill)
	mux.HandleFunc("PUT /users/me/skills/{id}", h.UpdateSkill)
	mux.HandleFunc("DELETE /users/me/skills/{id}", h.DeleteSkill)

	mux.HandleFunc("GET /users/me/projects", h.ListProjects)
	mux.HandleFunc("POST /users/me/projects", h.InsertProject)
	mux.HandleFunc("PUT /users/me/projects/{id}", h.UpdateProject)
	mux.HandleFunc("DELETE /users/me/projects/{id}", h.DeleteProject)

	mux.HandleFunc("GET /users/me/certificates", h.ListCertificates)
	mux.HandleFunc("POST /users/me/certificates", h.InsertCertificate)
	mux.HandleFunc("PUT /users/me/certificates/{id}", h.UpdateCertificate)
	mux.HandleFunc("DELETE /users/me/certificates/{id}", h.DeleteCertificate)
}
