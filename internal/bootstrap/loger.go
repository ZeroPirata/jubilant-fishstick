package bootstrap

import (
	"fmt"
	"hackton-treino/config"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitZapLog(cfg *config.Config) (*zap.Logger, func(), error) {
	if cfg.Project.LoggerFolder != "" {
		if err := os.MkdirAll(cfg.Project.LoggerFolder, 0755); err != nil {
			return nil, nil, fmt.Errorf("could not create log directory: %w", err)
		}
	}

	loggerConfig := zapConfigFromProjectConfig(*cfg)
	logger, err := loggerConfig.Build()
	if err != nil {
		return nil, nil, fmt.Errorf("could not create a build for log: %w", err)
	}

	logger = logger.With(
		zap.String("service", cfg.Project.Name),
		zap.String("version", cfg.Project.Version),
	)

	flush := func() {
		if err := logger.Sync(); err != nil {
			if !strings.Contains(err.Error(), "stdout") && !strings.Contains(err.Error(), "stderr") {
				fmt.Printf("Error syncing logger: %v\n", err)
			}
		}
	}

	return logger, flush, nil
}

func zapConfigFromProjectConfig(cfg config.Config) zap.Config {
	var zapConfig zap.Config

	if cfg.Project.Debug {
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		zapConfig = zap.NewProductionConfig()
	}

	zapConfig.OutputPaths = []string{"stdout"}
	zapConfig.ErrorOutputPaths = []string{"stderr"}

	if !cfg.Project.Debug && cfg.Project.LoggerFolder != "" {
		zapConfig.OutputPaths = append(
			zapConfig.OutputPaths,
			cfg.Project.LoggerFolder+"/"+cfg.Project.Name+".log",
		)
		zapConfig.ErrorOutputPaths = append(
			zapConfig.ErrorOutputPaths,
			cfg.Project.LoggerFolder+"/"+cfg.Project.Name+"_error.log",
		)
	}

	enc := zapConfig.EncoderConfig

	enc.EncodeLevel = zapcore.LowercaseLevelEncoder
	enc.EncodeTime = zapcore.ISO8601TimeEncoder
	enc.EncodeCaller = zapcore.ShortCallerEncoder
	enc.EncodeDuration = zapcore.StringDurationEncoder

	zapConfig.EncoderConfig = enc

	zapConfig.DisableStacktrace = true

	if cfg.Project.Debug {
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	} else {
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	return zapConfig
}
