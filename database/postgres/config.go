package postgres

import (
	"context"
	"fmt"
	"hackton-treino/config"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func getDatabaseConfig(cfg *config.Config) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
	)
}

func setupPoolConfig(databaseUrl string, c *config.Config, logr *zap.Logger) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(databaseUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing database URL: %w", err)
	}

	cfg.ConnConfig.OnNotice = func(c *pgconn.PgConn, n *pgconn.Notice) {
		logr.Info("PostgreSQL NOTICE", zap.String("message", n.Message), zap.String("code", n.Code))
	}

	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET statement_timeout = '30s'")
		if err != nil {
			logr.Error("Failed to set statement_timeout", zap.Error(err))
		}
		return nil
	}

	cfg.BeforeClose = func(conn *pgx.Conn) {
		logr.Debug("Closing connection")
	}

	numCPU := runtime.NumCPU()

	cfg.MaxConns = int32(config.GetConfigIntOrDefault(c.Database.MaxConnections, numCPU*4))
	cfg.MinConns = int32(config.GetConfigIntOrDefault(c.Database.MinConnections, max(2, numCPU/2)))

	cfg.MaxConnLifetime = config.GetConfigDurationOrDefault(c.Database.MaxConnLifetime, 2*time.Hour)
	cfg.MaxConnIdleTime = config.GetConfigDurationOrDefault(c.Database.MaxConnIdleTime, 15*time.Minute)
	cfg.HealthCheckPeriod = config.GetConfigDurationOrDefault(c.Database.HealthCheckPeriod, 30*time.Second)

	cfg.ConnConfig.ConnectTimeout = config.GetConfigDurationOrDefault(c.Database.ConnectTimeout, 10*time.Second)

	logr.Info("Database pool configuration",
		zap.String("max_conns", fmt.Sprintf("%d", cfg.MaxConns)),
		zap.String("min_conns", fmt.Sprintf("%d", cfg.MinConns)),
		zap.String("max_conn_lifetime", cfg.MaxConnLifetime.String()),
		zap.String("max_conn_idle_time", cfg.MaxConnIdleTime.String()),
		zap.String("health_check_period", cfg.HealthCheckPeriod.String()),
		zap.String("connect_timeout", cfg.ConnConfig.ConnectTimeout.String()),
	)
	return cfg, nil
}
