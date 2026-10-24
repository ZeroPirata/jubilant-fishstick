package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ActivityToucher grava o último uso autenticado de um usuário.
type ActivityToucher interface {
	TouchLastSeen(ctx context.Context, userID string) error
}

// activityTracker limita as escritas: no máximo uma por usuário a cada `every`.
type activityTracker struct {
	toucher ActivityToucher
	every   time.Duration
	now     func() time.Time
	mu      sync.Mutex
	last    map[string]time.Time
}

func newActivityTracker(t ActivityToucher, every time.Duration, now func() time.Time) *activityTracker {
	return &activityTracker{toucher: t, every: every, now: now, last: map[string]time.Time{}}
}

// due diz se já passou o intervalo desde a última gravação desse usuário (e reserva a vez).
func (a *activityTracker) due(userID string) bool {
	now := a.now()
	a.mu.Lock()
	defer a.mu.Unlock()
	if prev, ok := a.last[userID]; ok && now.Sub(prev) < a.every {
		return false
	}
	a.last[userID] = now
	return true
}

// ActivityMiddleware registra o último acesso do usuário autenticado (deve vir depois
// do AuthMiddleware). A escrita é em segundo plano e uma falha só gera log: métrica
// nunca pode atrasar nem derrubar a requisição.
func ActivityMiddleware(t ActivityToucher, every time.Duration) func(http.Handler) http.Handler {
	tracker := newActivityTracker(t, every, time.Now)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if uid, ok := GetUserID(r.Context()); ok {
				id := uid.String()
				if tracker.due(id) {
					go func() {
						ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
						defer cancel()
						if err := t.TouchLastSeen(ctx, id); err != nil {
							zap.L().Warn("activity: falha ao registrar último acesso", zap.Error(err))
						}
					}()
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
