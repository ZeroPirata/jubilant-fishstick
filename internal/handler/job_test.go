package handler

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/repository/jobs"
	"hackton-treino/internal/security"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeJobs implementa só o que o teste usa; o resto entra em pânico pela interface nula.
type fakeJobs struct {
	jobs.Repository
	deleted []db.QueryDeleteResumeParams
	exists  bool
}

func (f *fakeJobs) QueryDeleteResume(_ context.Context, p db.QueryDeleteResumeParams) *repository.RepositoryError {
	f.deleted = append(f.deleted, p)
	if !f.exists {
		return &repository.RepositoryError{StatusCode: http.StatusNotFound, Message: "resource not found"}
	}
	return nil
}

func TestDeleteResume_UsaIDDoCurriculo(t *testing.T) {
	const (
		jobID    = "11111111-1111-1111-1111-111111111111"
		resumeID = "22222222-2222-2222-2222-222222222222"
		userID   = "33333333-3333-3333-3333-333333333333"
	)
	for _, tc := range []struct {
		exists bool
		want   int
	}{{true, http.StatusNoContent}, {false, http.StatusNotFound}} {
		fake := &fakeJobs{exists: tc.exists}
		h := &JobHandler{Jobs: fake}
		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /jobs/{id}/resumes/{resume_id}", h.DeleteResume)

		req := httptest.NewRequest(http.MethodDelete, "/jobs/"+jobID+"/resumes/"+resumeID, nil)
		req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, security.ValidatedClaims{UserID: userID}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != tc.want {
			t.Fatalf("exists=%v: status %d, want %d", tc.exists, rec.Code, tc.want)
		}
		if len(fake.deleted) != 1 {
			t.Fatalf("QueryDeleteResume chamado %d vezes", len(fake.deleted))
		}
		p := fake.deleted[0]
		if p.ID.String() != resumeID || p.JobID.String() != jobID || p.UserID.String() != userID {
			t.Errorf("params = id %s job %s user %s", p.ID.String(), p.JobID.String(), p.UserID.String())
		}
	}
}
