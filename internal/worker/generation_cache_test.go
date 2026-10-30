package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"hackton-treino/internal/db"
	ucache "hackton-treino/internal/repository/cache"
	"hackton-treino/internal/resume"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// fakeCache guarda as gerações em memória; o resto da interface não é usado aqui.
type fakeCache struct {
	ucache.Cache
	gens map[string][]byte
	gets int
}

func (f *fakeCache) GetGeneration(_ context.Context, userID, key string) ([]byte, error) {
	f.gets++
	if b, ok := f.gens[userID+"|"+key]; ok {
		return b, nil
	}
	return nil, ucache.ErrCacheMiss
}

func (f *fakeCache) SetGeneration(_ context.Context, userID, key string, body []byte) error {
	f.gens[userID+"|"+key] = body
	return nil
}

func TestGenerationKey(t *testing.T) {
	k := generationKey("sys", "user")
	if k != generationKey("sys", "user") {
		t.Fatal("mesma entrada, chaves diferentes")
	}
	for _, other := range []string{generationKey("sys2", "user"), generationKey("sys", "user2"), generationKey("sysu", "ser")} {
		if other == k {
			t.Fatal("entradas diferentes com a mesma chave")
		}
	}
}

func TestGenerateCached_ReaproveitaResposta(t *testing.T) {
	job := &db.Job{ID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}, UserID: pgtype.UUID{Bytes: [16]byte{2}, Valid: true}}
	want := resume.LLMOutput{Resumo: "resumo guardado", CoverLetter: "carta"}
	body, _ := json.Marshal(want)
	c := &fakeCache{gens: map[string][]byte{job.UserID.String() + "|" + generationKey("sys", "user"): body}}
	// Sem LLM configurado: se o cache não for usado, a chamada falha.
	w := &Worker{Logger: zap.NewNop(), Cache: c}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := w.generateCached(ctx, job, "sys", "user", resume.Source{})
	if err != nil || got == nil || got.Resumo != want.Resumo || got.CoverLetter != want.CoverLetter {
		t.Fatalf("generateCached = %+v, %v; quero a resposta guardada", got, err)
	}

}
