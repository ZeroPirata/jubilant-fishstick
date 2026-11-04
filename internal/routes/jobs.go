package routes

import (
	"hackton-treino/internal/handler"
	"hackton-treino/internal/repository/secevents"
	"hackton-treino/internal/sse"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func setupJobRoutes(mux *http.ServeMux, logger *zap.Logger, db *pgxpool.Pool, bus *sse.Bus, secLog secevents.Repository) *handler.JobHandler {
	pdf := handler.NewPDFService(logger)
	h := handler.NewJobHandler(logger, db, pdf, bus, secLog)

	// Job
	mux.HandleFunc("POST /jobs", h.CreateJob)
	mux.HandleFunc("GET /jobs", h.ListJobs)
	mux.HandleFunc("GET /jobs/{id}", h.GetJob)
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

	// Quadro de candidaturas: etapa (enviado, entrevista...) e histórico de cada vaga.
	mux.HandleFunc("GET /applications", h.ApplicationBoard)
	mux.HandleFunc("GET /jobs/{id}/application", h.GetApplication)
	mux.HandleFunc("POST /jobs/{id}/application/events", h.AddApplicationEvent)
	mux.HandleFunc("DELETE /jobs/{id}/application/events/{event_id}", h.DeleteApplicationEvent)

	return h
}
