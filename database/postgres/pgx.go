package postgres

import (
	"context"
	"fmt"
	"hackton-treino/config"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var (
	dbPool *pgxpool.Pool
	once   sync.Once
)

func GetDataBasePool(c *config.Config, logr *zap.Logger) (*pgxpool.Pool, error) {
	var err error
	once.Do(func() {
		dbPool, err = CreateConnection(c, logr)
	})
	return dbPool, err
}

func CreateConnection(c *config.Config, logr *zap.Logger) (*pgxpool.Pool, error) {
	database := getDatabaseConfig(c)
	print(database)
	config, err := setupPoolConfig(database, c, logr)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar pool de conexões: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := testConnection(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	logr.Info("Pool de conexões criado com sucesso")
	return pool, nil
}

func ClosePool(logr *zap.Logger) {
	if dbPool != nil {
		dbPool.Close()
		logr.Info("Pool de conexões fechado")
	}
}

func testConnection(ctx context.Context, pool *pgxpool.Pool) error {
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("falha no ping básico: %w", err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("falha ao adquirir conexão: %w", err)
	}
	defer conn.Release()
	var result int
	if err := conn.QueryRow(ctx, "SELECT 1").Scan(&result); err != nil {
		return fmt.Errorf("falha na query de teste: %w", err)
	}
	return nil
}
