package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultTTL = 24 * time.Hour

const (
	TopicProfile      = "profile"
	TopicSkills       = "skills"
	TopicExperiences  = "experiences"
	TopicAcademic     = "academic"
	TopicProjects     = "projects"
	TopicCertificates = "certificates"

	keyLLMRateLimit = "worker:llm_rate_limited"
)

// ErrCacheMiss is returned by Get when the key does not exist in Redis.
var ErrCacheMiss = errors.New("cache miss")

type cache struct {
	r *redis.Client
}

type Cache interface {
	Create(ctx context.Context, userId, topic string, body []byte) error
	Get(ctx context.Context, userId, topic string) ([]byte, error)
	Delete(ctx context.Context, userId, topic string) error
	SetRateLimit(ctx context.Context, ttl time.Duration) error
	IsRateLimited(ctx context.Context) bool
}

func New(rds *redis.Client) Cache {
	return &cache{r: rds}
}

func createKey(userId, topic string) string {
	return fmt.Sprintf("user:%s:%s", userId, topic)
}

func (c *cache) Create(ctx context.Context, userId, topic string, body []byte) error {
	return c.r.Set(ctx, createKey(userId, topic), body, defaultTTL).Err()
}

func (c *cache) Get(ctx context.Context, userId, topic string) ([]byte, error) {
	result, err := c.r.Get(ctx, createKey(userId, topic)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, err
	}
	return []byte(result), nil
}

func (c *cache) Delete(ctx context.Context, userId, topic string) error {
	return c.r.Del(ctx, createKey(userId, topic)).Err()
}

func (c *cache) SetRateLimit(ctx context.Context, ttl time.Duration) error {
	return c.r.Set(ctx, keyLLMRateLimit, "1", ttl).Err()
}

func (c *cache) IsRateLimited(ctx context.Context) bool {
	n, err := c.r.Exists(ctx, keyLLMRateLimit).Result()
	return err == nil && n > 0
}

// GetTyped deserializes cached JSON into T. Returns (zero, false) on miss or unmarshal error.
func GetTyped[T any](ctx context.Context, c Cache, userId, topic string) (T, bool) {
	var zero T
	data, err := c.Get(ctx, userId, topic)
	if err != nil {
		return zero, false
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return zero, false
	}
	return v, true
}

// SetTyped serializes v as JSON and stores it with the default TTL.
func SetTyped[T any](ctx context.Context, c Cache, userId, topic string, v T) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Create(ctx, userId, topic, data)
}
