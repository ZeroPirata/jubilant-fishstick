package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"hackton-treino/internal/db"
	"hackton-treino/internal/metrics"
	ucache "hackton-treino/internal/repository/cache"
	"hackton-treino/internal/resume"
	"hackton-treino/internal/services"

	"go.uber.org/zap"
)

// generationCacheRevision muda quando o tratamento da resposta antes de guardá-la
// (generateWithCorrection) muda: as respostas antigas deixam de valer.
const generationCacheRevision = 1

// generationKey identifica um pedido à IA: o prompt de sistema (idioma e versão do
// prompt) e o do usuário (vaga, perfil, feedbacks e modo). Mudou qualquer um, a chave
// muda e a IA é chamada de novo.
func generationKey(systemPrompt, userPrompt string) string {
	h := sha256.New()
	h.Write([]byte{generationCacheRevision})
	h.Write([]byte(systemPrompt))
	h.Write([]byte{0})
	h.Write([]byte(userPrompt))
	return hex.EncodeToString(h.Sum(nil))
}

// generateCached reaproveita a resposta guardada para o mesmo pedido, a não ser que a
// vaga peça uma versão nova (fresh_generation, "Nova versão" no Refazer). Respostas
// novas são guardadas. O cache é só economia: se o Redis falhar, a IA é chamada.
func (w *Worker) generateCached(ctx context.Context, job *db.Job, systemPrompt, userPrompt string, src resume.Source) (*resume.LLMOutput, error) {
	jobID, userID := job.ID.String(), job.UserID.String()
	key := generationKey(systemPrompt, userPrompt)
	result := "fresh"
	if !job.FreshGeneration {
		result = "miss"
		if body, err := w.Cache.GetGeneration(ctx, userID, key); err == nil {
			var out resume.LLMOutput
			if err := json.Unmarshal(body, &out); err == nil {
				metrics.LLMCache.WithLabelValues("hit").Inc()
				w.Logger.Info("Currículo: resposta da IA reaproveitada do cache", zap.String("job_id", jobID))
				return &out, nil
			}
		} else if !errors.Is(err, ucache.ErrCacheMiss) {
			w.Logger.Warn("Cache da IA indisponível", zap.String("job_id", jobID), zap.Error(err))
		}
	}
	metrics.LLMCache.WithLabelValues(result).Inc()

	start := time.Now()
	out, err := w.generateWithCorrection(ctx, jobID, systemPrompt, userPrompt, src)
	status := "ok"
	if err != nil {
		status = "error"
		var errRL *services.ErrRateLimit
		if errors.As(err, &errRL) {
			status = "rate_limit"
		}
	}
	metrics.LLMDuration.WithLabelValues(status).Observe(time.Since(start).Seconds())
	if err != nil {
		return nil, err
	}

	if body, errM := json.Marshal(out); errM == nil {
		if errS := w.Cache.SetGeneration(ctx, userID, key, body); errS != nil {
			w.Logger.Warn("Erro ao guardar resposta da IA no cache", zap.String("job_id", jobID), zap.Error(errS))
		}
	}
	return out, nil
}
