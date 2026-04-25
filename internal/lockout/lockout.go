package lockout

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	maxAttempts = 5
	lockWindow  = 15 * time.Minute
	keyPrefix   = "auth:lockout:"
)

type Locker interface {
	IsLocked(ctx context.Context, email string) bool
	RecordFailure(ctx context.Context, email string)
	Reset(ctx context.Context, email string)
}

type redisLocker struct {
	rds *redis.Client
}

func New(rds *redis.Client) Locker {
	return &redisLocker{rds: rds}
}

func (l *redisLocker) IsLocked(ctx context.Context, email string) bool {
	val, err := l.rds.Get(ctx, keyPrefix+email).Int()
	if err != nil {
		return false
	}
	return val >= maxAttempts
}

func (l *redisLocker) RecordFailure(ctx context.Context, email string) {
	key := keyPrefix + email
	pipe := l.rds.TxPipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, lockWindow)
	if _, err := pipe.Exec(ctx); err != nil {
		zap.L().Warn("lockout: falha ao registrar tentativa", zap.Error(err))
	}
}

func (l *redisLocker) Reset(ctx context.Context, email string) {
	if err := l.rds.Del(ctx, keyPrefix+email).Err(); err != nil {
		zap.L().Warn("lockout: falha ao limpar lockout", zap.Error(err))
	}
}
