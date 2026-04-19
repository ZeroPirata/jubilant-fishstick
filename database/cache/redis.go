package cache

import (
	"context"
	"fmt"
	"hackton-treino/config"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	client *redis.Client
	once   sync.Once
)

func GetClient(cfg *config.Config, logger *zap.Logger) (*redis.Client, error) {
	var err error
	once.Do(func() {
		client, err = createClient(cfg, logger)
	})
	return client, err
}

func createClient(cfg *config.Config, logger *zap.Logger) (*redis.Client, error) {
	c := cfg.Cache

	rdb := redis.NewClient(&redis.Options{
		Addr:     c.Addr,
		Password: c.Password,
		DB:       c.DB,

		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,

		PoolSize:        10,
		MinIdleConns:    2,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("redis: ping falhou: %w", err)
	}

	logger.Info("redis: conexão estabelecida",
		zap.String("addr", c.Addr),
		zap.Int("db", c.DB),
	)

	return rdb, nil
}

func Close(logger *zap.Logger) {
	if client != nil {
		if err := client.Close(); err != nil {
			logger.Error("redis: erro ao fechar conexão", zap.Error(err))
			return
		}
		logger.Info("redis: conexão fechada")
	}
}
