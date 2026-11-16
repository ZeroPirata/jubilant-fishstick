package handler

import (
	"context"
	"encoding/json"
	"hackton-treino/internal/db"
	"hackton-treino/internal/discovery"
	"hackton-treino/internal/linkedinsearch"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/repository"
	discoveryRepo "hackton-treino/internal/repository/discovery"
	repo "hackton-treino/internal/repository/linkedin"
	"hackton-treino/internal/util"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ── Buscas no LinkedIn (aba Descobrir) ──
// O usuário salva buscas com os filtros que quiser (nenhum vem ligado) e abre o link
// no próprio navegador. O servidor só monta a URL: não acessa o LinkedIn.

const (
	linkedinSearchMax  = 20
	linkedinNameMax    = 80
	linkedinURLMaxSize = 4096
)

type LinkedInSearchHandler struct {
	*BaseHandler
	Repo repo.Repository
	// Target: o Alvo do Descobrir do usuário (para gerar as buscas).
	Target func(ctx context.Context, userID pgtype.UUID) (discovery.Target, *repository.RepositoryError)
}

func NewLinkedInSearchHandler(logger *zap.Logger, conn *pgxpool.Pool) *LinkedInSearchHandler {
	disc := discoveryRepo.New(conn)
	return &LinkedInSearchHandler{
		BaseHandler: NewBaseHandler(logger),
		Repo:        repo.New(conn),
		Target: func(ctx context.Context, userID pgtype.UUID) (discovery.Target, *repository.RepositoryError) {
			p, errR := disc.SelectPreferences(ctx, userID)
			return discovery.Target{Roles: p.Roles, Technologies: p.Technologies, Seniorities: p.Seniorities, WorkplaceTypes: p.WorkplaceTypes}, errR
		},
	}
}

// linkedinSearchBody: o corpo de POST/PUT e a resposta (com id e url).
type linkedinSearchBody struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	linkedinsearch.Search
	URL string `json:"url,omitempty"`
	// Auto: gerada pelo Alvo ("Gerar pelo meu Alvo" troca as automáticas).
	Auto bool `json:"auto"`
}

func linkedinSearchView(row db.UserLinkedinSearch) linkedinSearchBody {
	s := linkedinsearch.Search{
		Keywords: row.Keywords, GeoID: row.GeoID, GeoLabel: row.GeoLabel, PostedWithin: row.PostedWithin,
		Workplace: nonNil(row.Workplace), Experience: nonNil(row.Experience), JobTypes: nonNil(row.JobTypes),
		EasyApply: row.EasyApply, FewApplicants: row.FewApplicants, InNetwork: row.InNetwork, Sort: row.Sort,
	}
	return linkedinSearchBody{ID: row.ID.String(), Name: row.Name, Search: s, URL: linkedinsearch.URL(s), Auto: row.Auto}
}

// readLinkedinSearch lê e valida o corpo. O nome é obrigatório só para salvar.
func readLinkedinSearch(w http.ResponseWriter, r *http.Request, needName bool) (string, linkedinsearch.Search, bool) {
	var body linkedinSearchBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return "", linkedinsearch.Search{}, false
	}
	name := strings.Join(strings.Fields(body.Name), " ")
	if needName && name == "" {
		writeError(w, http.StatusBadRequest, "dê um nome para a busca")
		return "", linkedinsearch.Search{}, false
	}
	if utf8.RuneCountInString(name) > linkedinNameMax {
		writeError(w, http.StatusBadRequest, "nome longo demais (até 80 caracteres)")
		return "", linkedinsearch.Search{}, false
	}
	s, err := linkedinsearch.Clean(body.Search)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return "", linkedinsearch.Search{}, false
	}
	return name, s, true
}

// List (GET /linkedin/searches): as buscas salvas, com o link pronto.
func (h *LinkedInSearchHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	rows, errR := h.Repo.List(r.Context(), userID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	out := make([]linkedinSearchBody, 0, len(rows))
	for _, row := range rows {
		out = append(out, linkedinSearchView(row))
	}
	writeJSON(w, http.StatusOK, out)
}

// Create (POST /linkedin/searches).
func (h *LinkedInSearchHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	name, s, ok := readLinkedinSearch(w, r, true)
	if !ok {
		return
	}
	n, errR := h.Repo.Count(r.Context(), userID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	if n >= linkedinSearchMax {
		writeError(w, http.StatusConflict, "limite de 20 buscas salvas: apague uma antes")
		return
	}
	row, errR := h.Repo.Insert(r.Context(), db.LinkedInInsertSearchParams{
		UserID: userID, Name: name, Keywords: s.Keywords, GeoID: s.GeoID, GeoLabel: s.GeoLabel,
		PostedWithin: s.PostedWithin, Workplace: s.Workplace, Experience: s.Experience, JobTypes: s.JobTypes,
		EasyApply: s.EasyApply, FewApplicants: s.FewApplicants, InNetwork: s.InNetwork, Sort: s.Sort,
	})
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	writeJSON(w, http.StatusCreated, linkedinSearchView(row))
}

// Update (PUT /linkedin/searches/{id}).
func (h *LinkedInSearchHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := linkedinSearchTarget(w, r)
	if !ok {
		return
	}
	name, s, ok := readLinkedinSearch(w, r, true)
	if !ok {
		return
	}
	row, errR := h.Repo.Update(r.Context(), db.LinkedInUpdateSearchParams{
		ID: id, UserID: userID, Name: name, Keywords: s.Keywords, GeoID: s.GeoID, GeoLabel: s.GeoLabel,
		PostedWithin: s.PostedWithin, Workplace: s.Workplace, Experience: s.Experience, JobTypes: s.JobTypes,
		EasyApply: s.EasyApply, FewApplicants: s.FewApplicants, InNetwork: s.InNetwork, Sort: s.Sort,
	})
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	writeJSON(w, http.StatusOK, linkedinSearchView(row))
}

// Delete (DELETE /linkedin/searches/{id}).
func (h *LinkedInSearchHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := linkedinSearchTarget(w, r)
	if !ok {
		return
	}
	found, errR := h.Repo.Delete(r.Context(), db.LinkedInDeleteSearchParams{ID: id, UserID: userID})
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "busca não encontrada")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Preview (POST /linkedin/searches/preview): o link de uma busca ainda não salva,
// para a tela mostrar enquanto o usuário escolhe os filtros.
func (h *LinkedInSearchHandler) Preview(w http.ResponseWriter, r *http.Request) {
	_, s, ok := readLinkedinSearch(w, r, false)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": linkedinsearch.URL(s)})
}

// FromURL (POST /linkedin/searches/from-url {url}): lê uma URL de busca colada do
// LinkedIn e devolve os filtros, para preencher o formulário (não salva).
func (h *LinkedInSearchHandler) FromURL(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil || len(body.URL) > linkedinURLMaxSize {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}
	s, err := linkedinsearch.FromURL(body.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, linkedinSearchBody{Search: s, URL: linkedinsearch.URL(s)})
}

func linkedinSearchTarget(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, bool) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	id, errR := util.ParseUUID(r.PathValue("id"))
	if errR != nil {
		writeRepositoryError(w, errR)
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	return userID, id, true
}

// Generate (POST /linkedin/searches/generate): monta as buscas pelo Alvo do Descobrir
// (função, tecnologias, nível e modo de trabalho) e troca as automáticas de antes. As
// buscas feitas à mão ficam. O local vem da primeira busca salva com local; sem
// nenhuma, ou quando o Alvo é só remoto, é o Brasil.
func (h *LinkedInSearchHandler) Generate(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	target, errR := h.Target(r.Context(), userID)
	if errR != nil && errR.StatusCode != http.StatusNotFound {
		writeRepositoryError(w, errR)
		return
	}
	rows, errR := h.Repo.List(r.Context(), userID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	geoID, geoLabel := linkedinsearch.GeoBrasil, "Brasil"
	onlyRemote := len(target.WorkplaceTypes) > 0 && !slices.ContainsFunc(target.WorkplaceTypes, func(wt string) bool { return wt == "hybrid" || wt == "onsite" })
	manual := 0
	for _, row := range rows {
		if row.Auto {
			continue
		}
		manual++
		if row.GeoID != "" && geoID == linkedinsearch.GeoBrasil && !onlyRemote {
			geoID, geoLabel = row.GeoID, row.GeoLabel
		}
	}
	generated := linkedinsearch.FromTarget(target, geoID, geoLabel)
	if len(generated) == 0 {
		writeError(w, http.StatusBadRequest, "defina o seu Alvo no Descobrir (função ou tecnologia) para gerar as buscas")
		return
	}
	if manual+len(generated) > linkedinSearchMax {
		writeError(w, http.StatusConflict, "limite de 20 buscas salvas: apague uma antes")
		return
	}
	params := make([]db.LinkedInInsertSearchParams, 0, len(generated))
	for _, g := range generated {
		s := g.Search
		params = append(params, db.LinkedInInsertSearchParams{
			Name: g.Name, Keywords: s.Keywords, GeoID: s.GeoID, GeoLabel: s.GeoLabel, PostedWithin: s.PostedWithin,
			Workplace: s.Workplace, Experience: s.Experience, JobTypes: s.JobTypes,
			EasyApply: s.EasyApply, FewApplicants: s.FewApplicants, InNetwork: s.InNetwork, Sort: s.Sort,
		})
	}
	if errR := h.Repo.ReplaceAuto(r.Context(), userID, params); errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	h.List(w, r)
}
