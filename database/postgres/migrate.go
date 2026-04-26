package postgres

import (
	"database/sql"
	"embed"
	"fmt"
	"hackton-treino/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

func RunMigrations(fs embed.FS, cfg *config.Config, logr *zap.Logger) error {
	dsn := getDatabaseConfig(cfg)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db for migrations: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(fs)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	logr.Info("migrations aplicadas com sucesso")
	return nil
}
