package security

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// TokenRevoker is the interface implemented by Revoker; use it in tests to avoid Redis.
type TokenRevoker interface {
	Revoke(ctx context.Context, jti string, exp time.Time)
	IsRevoked(ctx context.Context, jti string) bool
}

const revokedPrefix = "jwt:revoked:"

type Revoker struct {
	rds *redis.Client
}

func NewRevoker(rds *redis.Client) *Revoker {
	return &Revoker{rds: rds}
}

// Revoke adds the JTI to the Redis blacklist until the token expires naturally.
func (r *Revoker) Revoke(ctx context.Context, jti string, exp time.Time) {
	ttl := time.Until(exp)
	if ttl <= 0 {
		return
	}
	if err := r.rds.Set(ctx, revokedPrefix+jti, "1", ttl).Err(); err != nil {
		zap.L().Warn("revocation: falha ao revogar token", zap.String("jti", jti), zap.Error(err))
	}
}

// IsRevoked returns true if the JTI is in the blacklist.
func (r *Revoker) IsRevoked(ctx context.Context, jti string) bool {
	if jti == "" {
		return false
	}
	n, err := r.rds.Exists(ctx, revokedPrefix+jti).Result()
	if err != nil {
		zap.L().Warn("revocation: falha ao checar revogação", zap.Error(err))
		return false
	}
	return n > 0
}
