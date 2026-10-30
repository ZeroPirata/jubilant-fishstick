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
	TopicLanguages    = "languages"

	keyLLMRateLimit = "worker:llm_rate_limited"

	// Respostas da IA guardadas por hash do prompt (worker.generationKey).
	generationTTL = 30 * 24 * time.Hour
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
	// RateLimitRemaining devolve quanto falta para a pausa do worker acabar (0 = não pausado).
	RateLimitRemaining(ctx context.Context) time.Duration
	ClearRateLimit(ctx context.Context) error
	// GetGeneration/SetGeneration guardam a resposta da IA de uma geração, por usuário
	// e hash do prompt, por generationTTL. Get devolve ErrCacheMiss quando não há.
	GetGeneration(ctx context.Context, userId, key string) ([]byte, error)
	SetGeneration(ctx context.Context, userId, key string, body []byte) error
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

func (c *cache) RateLimitRemaining(ctx context.Context) time.Duration {
	ttl, err := c.r.TTL(ctx, keyLLMRateLimit).Result()
	if err != nil || ttl < 0 {
		return 0
	}
	return ttl
}

func (c *cache) ClearRateLimit(ctx context.Context) error {
	return c.r.Del(ctx, keyLLMRateLimit).Err()
}

func generationKey(userId, key string) string {
	return fmt.Sprintf("user:%s:generation:%s", userId, key)
}

func (c *cache) GetGeneration(ctx context.Context, userId, key string) ([]byte, error) {
	b, err := c.r.Get(ctx, generationKey(userId, key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheMiss
	}
	return b, err
}

func (c *cache) SetGeneration(ctx context.Context, userId, key string, body []byte) error {
	return c.r.Set(ctx, generationKey(userId, key), body, generationTTL).Err()
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
