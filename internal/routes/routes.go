package routes

import (
	"hackton-treino/internal/handler"
	"hackton-treino/internal/repository"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func PipeCurriculoStup(mux *http.ServeMux, logger *zap.Logger, dbPool *pgxpool.Pool) {
	r := repository.NewRepository(dbPool)
	h := &handler.PipelineCurriculo{Logger: logger, DataBase: r}
	p := &handler.PerfilHandler{Logger: logger, DataBase: r}
	// Vagas
	mux.HandleFunc("GET /", handler.ServeUI)
	mux.Handle("GET /output/", http.StripPrefix("/output/", http.FileServer(http.Dir("output"))))
	mux.HandleFunc("POST /vagas", h.CreateVaga)
	mux.HandleFunc("GET /vagas", h.ListarVagas)
	mux.HandleFunc("DELETE /vagas/{id}", h.DeleteVaga)
	mux.HandleFunc("PATCH /vagas/{id}/status", h.UpdateVagaStatus)
	mux.HandleFunc("GET /curriculos", h.ListarCurriculos)
	mux.HandleFunc("DELETE /curriculos/{id}", h.DeleteCurriculoGerado)
	mux.HandleFunc("POST /curriculos/{id}/gerar-pdf", h.GerarPDF)
	mux.HandleFunc("POST /feedback", h.InserirFeedback)
	// Perfil
	mux.HandleFunc("GET /perfil/info", p.GetInformacoesBasicas)
	mux.HandleFunc("POST /perfil/info/{id}", p.UpsertInformacoesBasicas)
	mux.HandleFunc("GET /perfil/experiencias", p.ListExperiencias)
	mux.HandleFunc("POST /perfil/experiencias", p.InsertExperiencia)
	mux.HandleFunc("DELETE /perfil/experiencias/{id}", p.DeleteExperiencia)
	mux.HandleFunc("GET /perfil/habilidades", p.ListHabilidades)
	mux.HandleFunc("POST /perfil/habilidades", p.InsertHabilidade)
	mux.HandleFunc("DELETE /perfil/habilidades/{id}", p.DeleteHabilidade)
	mux.HandleFunc("GET /perfil/projetos", p.ListProjetos)
	mux.HandleFunc("POST /perfil/projetos", p.InsertProjeto)
	mux.HandleFunc("DELETE /perfil/projetos/{id}", p.DeleteProjeto)
	mux.HandleFunc("GET /perfil/filtros", p.ListFiltros)
	mux.HandleFunc("POST /perfil/filtros", p.InsertFiltro)
	mux.HandleFunc("DELETE /perfil/filtros/{id}", p.DeleteFiltro)
	mux.HandleFunc("GET /perfil/certificacoes", p.ListCertificacoes)
	mux.HandleFunc("POST /perfil/certificacoes", p.InsertCertificacao)
	mux.HandleFunc("DELETE /perfil/certificacoes/{id}", p.DeleteCertificacao)
}
