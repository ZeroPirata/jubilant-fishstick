package handler

import (
	"context"
	"encoding/json"
	"hackton-treino/internal/db"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/security"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

type fakeLinkedinRepo struct {
	count    int64
	inserted []db.LinkedInInsertSearchParams
	deleted  bool
}

func (f *fakeLinkedinRepo) List(context.Context, pgtype.UUID) ([]db.UserLinkedinSearch, *repository.RepositoryError) {
	return []db.UserLinkedinSearch{{Name: "Go", Keywords: "go", Workplace: []string{"2"}}}, nil
}
func (f *fakeLinkedinRepo) Count(context.Context, pgtype.UUID) (int64, *repository.RepositoryError) {
	return f.count, nil
}
func (f *fakeLinkedinRepo) Insert(_ context.Context, a db.LinkedInInsertSearchParams) (db.UserLinkedinSearch, *repository.RepositoryError) {
	f.inserted = append(f.inserted, a)
	return db.UserLinkedinSearch{Name: a.Name, Keywords: a.Keywords, Workplace: a.Workplace, Experience: a.Experience, JobTypes: a.JobTypes}, nil
}
func (f *fakeLinkedinRepo) Update(context.Context, db.LinkedInUpdateSearchParams) (db.UserLinkedinSearch, *repository.RepositoryError) {
	return db.UserLinkedinSearch{}, &repository.RepositoryError{StatusCode: http.StatusNotFound, Message: "resource not found"}
}
func (f *fakeLinkedinRepo) Delete(context.Context, db.LinkedInDeleteSearchParams) (bool, *repository.RepositoryError) {
	return f.deleted, nil
}

func linkedinReq(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	return req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID,
		security.ValidatedClaims{UserID: "11111111-1111-1111-1111-111111111111"}))
}

func linkedinData(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out struct {
		Data  map[string]any `json:"data"`
		Error string         `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v (%s)", err, w.Body.String())
	}
	return out.Data
}

func TestLinkedInCreate(t *testing.T) {
	f := &fakeLinkedinRepo{}
	h := &LinkedInSearchHandler{Repo: f}
	w := httptest.NewRecorder()
	h.Create(w, linkedinReq("POST", "/linkedin/searches", `{"name":"  Back-end  Go ","keywords":"golang","workplace":["3","2"]}`))
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if len(f.inserted) != 1 || f.inserted[0].Name != "Back-end Go" || strings.Join(f.inserted[0].Workplace, ",") != "2,3" {
		t.Errorf("gravou %+v", f.inserted)
	}
	// Sem filtro nenhum: listas vazias, não nulas (o banco não aceita NULL).
	if f.inserted[0].Experience == nil || f.inserted[0].JobTypes == nil {
		t.Errorf("lista nula: %+v", f.inserted[0])
	}
	if url, _ := linkedinData(t, w)["url"].(string); !strings.Contains(url, "f_WT=2%2C3") {
		t.Errorf("url = %q", url)
	}
}

func TestLinkedInCreateRejects(t *testing.T) {
	for name, tc := range map[string]struct {
		body  string
		count int64
		code  int
	}{
		"sem nome":     {`{"keywords":"go"}`, 0, http.StatusBadRequest},
		"filtro ruim":  {`{"name":"x","experience":["9"]}`, 0, http.StatusBadRequest},
		"json ruim":    {`{`, 0, http.StatusBadRequest},
		"limite de 20": {`{"name":"x"}`, 20, http.StatusConflict},
	} {
		f := &fakeLinkedinRepo{count: tc.count}
		w := httptest.NewRecorder()
		(&LinkedInSearchHandler{Repo: f}).Create(w, linkedinReq("POST", "/linkedin/searches", tc.body))
		if w.Code != tc.code || len(f.inserted) != 0 {
			t.Errorf("%s: status %d, gravou %d", name, w.Code, len(f.inserted))
		}
	}
}

func TestLinkedInFromURLAndPreview(t *testing.T) {
	h := &LinkedInSearchHandler{Repo: &fakeLinkedinRepo{}}
	w := httptest.NewRecorder()
	h.FromURL(w, linkedinReq("POST", "/x", `{"url":"https://www.linkedin.com/jobs/search-results/?currentJobId=1&keywords=Desenvolvedor%20Back-end&geoId=105104646"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	d := linkedinData(t, w)
	if d["keywords"] != "Desenvolvedor Back-end" || d["geo_id"] != "105104646" {
		t.Errorf("leu %+v", d)
	}

	w = httptest.NewRecorder()
	h.FromURL(w, linkedinReq("POST", "/x", `{"url":"https://evil.com/jobs/search/"}`))
	if w.Code != http.StatusBadRequest {
		t.Errorf("url de fora: status %d", w.Code)
	}

	w = httptest.NewRecorder()
	h.Preview(w, linkedinReq("POST", "/x", `{"keywords":"go","sort":"DD"}`))
	if url, _ := linkedinData(t, w)["url"].(string); url != "https://www.linkedin.com/jobs/search/?keywords=go&sortBy=DD" {
		t.Errorf("preview = %q", url)
	}
}

func TestLinkedInDeleteAndUpdateNotFound(t *testing.T) {
	h := &LinkedInSearchHandler{Repo: &fakeLinkedinRepo{deleted: false}}
	req := linkedinReq("DELETE", "/linkedin/searches/22222222-2222-2222-2222-222222222222", "")
	req.SetPathValue("id", "22222222-2222-2222-2222-222222222222")
	w := httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("delete de outro usuário: status %d", w.Code)
	}

	req = linkedinReq("PUT", "/x", `{"name":"x"}`)
	req.SetPathValue("id", "22222222-2222-2222-2222-222222222222")
	w = httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("update de outro usuário: status %d", w.Code)
	}

	req = linkedinReq("DELETE", "/x", "")
	req.SetPathValue("id", "não-é-uuid")
	w = httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code < 400 {
		t.Errorf("id inválido: status %d", w.Code)
	}
}
