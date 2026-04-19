package routes

import (
	"hackton-treino/internal/handler"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func setupJobRoutes(mux *http.ServeMux, logger *zap.Logger, db *pgxpool.Pool) {
	pdf := handler.NewPDFService(logger)
	h := handler.NewJobHandler(logger, db, pdf)

	// Job
	mux.HandleFunc("POST /jobs", h.CreateJob)
	mux.HandleFunc("GET /jobs", h.ListJobs)
	mux.HandleFunc("DELETE /jobs/{id}", h.DeleteJob)
	mux.HandleFunc("PUT /jobs/{id}", h.UpdateFullJob)
	mux.HandleFunc("PUT /jobs/{id}/retry", h.RetryJobProcessing)

	// Resume
	mux.HandleFunc("GET /jobs/{id}/resumes", h.ListResumes)
	mux.HandleFunc("GET /jobs/{id}/resumes/{resume_id}", h.GetResume)
	mux.HandleFunc("DELETE /jobs/{id}/resumes/{resume_id}", h.DeleteResume)
	mux.HandleFunc("POST /jobs/{id}/resumes/{resume_id}/pdf", h.GeneratePDF)

	// Feedback
	mux.HandleFunc("POST /jobs/{id}/resumes/{resume_id}/feedback", h.InsertFeedback)
}
