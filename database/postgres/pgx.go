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
	logr.Info("conexão: ", zap.Any("url", database))
	poolConfig, err := setupPoolConfig(database, c, logr)
	if err != nil {
		return nil, err
	}

	var pool *pgxpool.Pool
	maxRetries := 5

	for i := 1; i <= maxRetries; i++ {
		logr.Info("Tentando conectar ao banco de dados...",
			zap.Int("tentativa", i),
			zap.Int("max_tentativas", maxRetries))

		pool, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err = pool.Ping(ctx)
			cancel()

			if err == nil {
				// Se o ping passou, testamos a query SELECT 1
				ctxTest, cancelTest := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancelTest()
				if errTest := testConnection(ctxTest, pool); errTest == nil {
					logr.Info("Conexão com o banco estabelecida com sucesso!")
					return pool, nil
				} else {
					err = errTest
				}
			}
		}

		if pool != nil {
			pool.Close()
		}

		logr.Warn("Falha na conexão, agendando nova tentativa",
			zap.Error(err),
			zap.Duration("espera", time.Duration(i*2)*time.Second))

		time.Sleep(time.Duration(i*2) * time.Second)
	}

	return nil, fmt.Errorf("após %d tentativas, não foi possível conectar ao banco: %w", maxRetries, err)
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
