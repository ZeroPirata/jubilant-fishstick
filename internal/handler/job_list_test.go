package handler

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/repository/jobs"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// fakeJobList: as vagas candidatas (da mais recente para a mais antiga) e a busca por id.
type fakeJobList struct {
	jobs.Repository
	cands   []db.QuerySelectJobFitRowsRow
	fetched [][]pgtype.UUID
}

func (f *fakeJobList) QuerySelectJobFitRows(context.Context, db.QuerySelectJobFitRowsParams) ([]db.QuerySelectJobFitRowsRow, *repository.RepositoryError) {
	return f.cands, nil
}

func (f *fakeJobList) QuerySelectJobsByIds(_ context.Context, p db.QuerySelectJobsByIdsParams) ([]db.QuerySelectJobsByIdsRow, *repository.RepositoryError) {
	f.fetched = append(f.fetched, p.Ids)
	out := make([]db.QuerySelectJobsByIdsRow, 0, len(p.Ids))
	for i := len(p.Ids) - 1; i >= 0; i-- { // ordem do banco não é garantida
		out = append(out, db.QuerySelectJobsByIdsRow{ID: p.Ids[i]})
	}
	return out, nil
}

func uid(b byte) pgtype.UUID { return pgtype.UUID{Bytes: [16]byte{b}, Valid: true} }

func TestListJobsByFit(t *testing.T) {
	skills := []db.UserSkill{
		{SkillName: "Go", ProficiencyLevel: db.SkillLevelExpert},
		{SkillName: "PostgreSQL", ProficiencyLevel: db.SkillLevelAdvanced},
	}
	fake := &fakeJobList{cands: []db.QuerySelectJobFitRowsRow{
		{ID: uid(1), TechStack: []string{"go", "react"}},               // 50%
		{ID: uid(2), TechStack: nil},                                   // ainda lendo: sem %
		{ID: uid(3), TechStack: []string{"go", "postgresql"}},          // 100%
		{ID: uid(4), TechStack: []string{"java"}},                      // 0%
		{ID: uid(5), TechStack: []string{"go", "postgresql", "kafka"}}, // 67%
	}}
	h := &JobHandler{Jobs: fake}
	ids := func(rows []db.QuerySelectJobsForUserRow) []byte {
		var b []byte
		for _, r := range rows {
			b = append(b, r.ID.Bytes[0])
		}
		return b
	}
	for _, tc := range []struct {
		name         string
		f            jobListFilter
		cursor, size int32
		want         []byte
		total        int32
	}{
		{"mínimo 50%, mais recentes", jobListFilter{MinPct: 50}, 0, 10, []byte{1, 3, 5}, 3},
		{"ordem por %, sem stack no fim", jobListFilter{ByFit: true}, 0, 10, []byte{3, 5, 1, 4, 2}, 5},
		{"ordem por %, 2ª página", jobListFilter{ByFit: true}, 2, 2, []byte{1, 4}, 5},
		{"página além do fim", jobListFilter{MinPct: 60}, 4, 2, nil, 2},
	} {
		rows, total, errR := h.listJobsByFit(context.Background(), uid(9), tc.f, tc.cursor, tc.size, skills, nil)
		if errR != nil {
			t.Fatalf("%s: %v", tc.name, errR)
		}
		if got := ids(rows); string(got) != string(tc.want) || total != tc.total {
			t.Errorf("%s: ids %v total %d, esperado %v total %d", tc.name, got, total, tc.want, tc.total)
		}
	}
}
