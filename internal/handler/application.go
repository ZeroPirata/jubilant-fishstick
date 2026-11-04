package handler

import (
	"encoding/json"
	"hackton-treino/internal/db"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/util"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
)

// ── Quadro de candidaturas ──
// Depois do currículo pronto, a pessoa marca o que aconteceu: enviado, entrevista,
// proposta e o fim (contratado, recusado, desistiu). Cada marcação vira um item do
// histórico com data (pode ser retroativa) e anotação opcional; a etapa atual é a
// última do histórico.

// Etapas aceitas num evento. "ready" desfaz o envio (volta a "Currículo pronto") e
// "note" só anota, sem mudar a etapa.
var applicationStages = map[string]bool{
	"ready": true, "applied": true, "interview": true, "offer": true,
	"hired": true, "rejected": true, "withdrawn": true, "note": true,
}

const applicationNoteMax = 1000

// O Brasil não tem horário de verão desde 2019: um fuso fixo evita depender do tzdata
// da imagem. Data informada vira meio-dia desse dia (não muda de dia em nenhum fuso
// do país).
var brt = time.FixedZone("BRT", -3*60*60)

type ApplicationEvent struct {
	ID         string    `json:"id"`
	Stage      string    `json:"stage"`
	Note       string    `json:"note,omitempty"`
	HappenedAt time.Time `json:"happened_at"`
	CreatedAt  time.Time `json:"created_at"`
}

type Application struct {
	Stage   string             `json:"stage"` // "" = ainda não enviado
	StageAt *time.Time         `json:"stage_at,omitempty"`
	Events  []ApplicationEvent `json:"events"`
	// EventID: o evento recém-criado (POST), para o "Desfazer".
	EventID string `json:"event_id,omitempty"`
}

type ApplicationBoardItem struct {
	ID               string     `json:"id"`
	JobTitle         string     `json:"job_title,omitempty"`
	CompanyName      string     `json:"company_name,omitempty"`
	Url              string     `json:"url,omitempty"`
	Quality          string     `json:"quality,omitempty"`
	Status           string     `json:"status"`
	Stage            string     `json:"stage"`
	StageAt          *time.Time `json:"stage_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	ResumeAt         *time.Time `json:"resume_at,omitempty"`
	AppliedAt        *time.Time `json:"applied_at,omitempty"`
	ReachedInterview bool       `json:"reached_interview"`
	LastNote         string     `json:"last_note,omitempty"`
}

// parseApplicationDate lê "AAAA-MM-DD" (vazio = agora). Hoje vira o instante atual,
// para a ordem do histórico seguir a dos cliques; outro dia vira meio-dia dele. Não
// aceita data futura nem com mais de 3 anos.
func parseApplicationDate(s string, now time.Time) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return now, true
	}
	d, err := time.ParseInLocation("2006-01-02", s, brt)
	if err != nil {
		return time.Time{}, false
	}
	today := now.In(brt)
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, brt)
	if d.After(todayDate) || d.Before(todayDate.AddDate(-3, 0, 0)) {
		return time.Time{}, false
	}
	if d.Equal(todayDate) {
		return now, true
	}
	return d.Add(12 * time.Hour), true
}

// ApplicationBoard lista as vagas do quadro: concluídas e as que já têm etapa.
func (h *JobHandler) ApplicationBoard(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	rows, errR := h.Jobs.ApplicationBoard(r.Context(), userID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	out := make([]ApplicationBoardItem, len(rows))
	for i, j := range rows {
		out[i] = ApplicationBoardItem{
			ID:               j.ID.String(),
			JobTitle:         j.JobTitle.String,
			CompanyName:      j.CompanyName.String,
			Url:              j.ExternalUrl,
			Quality:          string(j.Quality.JobQuality),
			Status:           string(j.Status),
			Stage:            j.ApplicationStage.String,
			StageAt:          pgTimePtr(j.ApplicationStageAt),
			CreatedAt:        j.CreatedAt.Time,
			ResumeAt:         pgTimePtr(j.ResumeAt),
			AppliedAt:        pgTimePtr(j.AppliedAt),
			ReachedInterview: j.ReachedInterview,
			LastNote:         j.LastNote,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// GetApplication devolve a etapa atual e o histórico de uma vaga.
func (h *JobHandler) GetApplication(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	jobID, errP := util.ParseUUID(r.PathValue("id"))
	if errP != nil {
		writeRepositoryError(w, errP)
		return
	}
	h.writeApplication(w, r, userID, jobID, http.StatusOK, "")
}

// AddApplicationEvent marca uma etapa (ou anotação): {stage, note, date}.
func (h *JobHandler) AddApplicationEvent(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	jobID, errP := util.ParseUUID(r.PathValue("id"))
	if errP != nil {
		writeRepositoryError(w, errP)
		return
	}
	var body struct {
		Stage string `json:"stage"`
		Note  string `json:"note"`
		Date  string `json:"date"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if !applicationStages[body.Stage] {
		writeError(w, http.StatusBadRequest, "etapa inválida")
		return
	}
	note := strings.TrimSpace(body.Note)
	if utf8.RuneCountInString(note) > applicationNoteMax {
		writeError(w, http.StatusBadRequest, "anotação longa demais (máx. 1000 caracteres)")
		return
	}
	if body.Stage == "note" && note == "" {
		writeError(w, http.StatusBadRequest, "anotação vazia")
		return
	}
	at, okD := parseApplicationDate(body.Date, time.Now())
	if !okD {
		writeError(w, http.StatusBadRequest, "data inválida: use uma data de hoje ou do passado")
		return
	}
	eventID, errR := h.Jobs.ApplicationAddEvent(r.Context(), db.QueryInsertApplicationEventParams{
		JobID:      jobID,
		UserID:     userID,
		Stage:      body.Stage,
		Note:       note,
		HappenedAt: pgtype.Timestamptz{Time: at, Valid: true},
	})
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	h.writeApplication(w, r, userID, jobID, http.StatusCreated, eventID)
}

// DeleteApplicationEvent apaga um item do histórico (marcação errada).
func (h *JobHandler) DeleteApplicationEvent(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	jobID, errP := util.ParseUUID(r.PathValue("id"))
	if errP != nil {
		writeRepositoryError(w, errP)
		return
	}
	eventID, errE := util.ParseUUID(r.PathValue("event_id"))
	if errE != nil {
		writeRepositoryError(w, errE)
		return
	}
	if errR := h.Jobs.ApplicationDeleteEvent(r.Context(), userID, jobID, eventID); errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	h.writeApplication(w, r, userID, jobID, http.StatusOK, "")
}

func (h *JobHandler) writeApplication(w http.ResponseWriter, r *http.Request, userID, jobID pgtype.UUID, status int, eventID string) {
	j, errR := h.Jobs.QuerySelectJobForUser(r.Context(), db.QuerySelectJobForUserParams{ID: jobID, UserID: userID})
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	rows, errR := h.Jobs.ApplicationEvents(r.Context(), userID, jobID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	out := Application{
		Stage:   j.ApplicationStage.String,
		StageAt: pgTimePtr(j.ApplicationStageAt),
		Events:  make([]ApplicationEvent, len(rows)),
		EventID: eventID,
	}
	for i, e := range rows {
		out.Events[i] = ApplicationEvent{
			ID:         e.ID.String(),
			Stage:      e.Stage,
			Note:       e.Note,
			HappenedAt: e.HappenedAt.Time,
			CreatedAt:  e.CreatedAt.Time,
		}
	}
	writeJSON(w, status, out)
}

func pgTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
