// Package main is the entry point for the bark API server.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	bark "github.com/donaldgifford/bark/api"
	"github.com/donaldgifford/bark/api/database"
	"github.com/donaldgifford/bark/api/middleware"
	"github.com/donaldgifford/bark/api/s3"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbCfg := database.Config{
		DSN:            getEnv("BARK_DB_DSN", "postgres://bark:bark@localhost:5432/bark?sslmode=disable"),
		MigrationsPath: getEnv("BARK_MIGRATIONS_PATH", "api/database/migrations"),
	}

	pool, err := database.Open(ctx, dbCfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("database connected")

	if err := database.Migrate(dbCfg, logger); err != nil {
		return err
	}

	presignTTL := getEnvDuration("BARK_PRESIGNED_URL_TTL_MINUTES", 5) * time.Minute

	s3Client, err := s3.New(ctx, s3.Config{
		Bucket:     getEnv("BARK_S3_BUCKET", "homebrew-bottles"),
		Endpoint:   getEnv("BARK_S3_ENDPOINT", ""),
		Region:     getEnv("AWS_REGION", "us-east-1"),
		PresignTTL: presignTTL,
	})
	if err != nil {
		return err
	}
	logger.Info("s3 client initialized")

	svrCfg := bark.Config{
		Addr: getEnv("BARK_API_ADDR", ":8080"),
		JWTConfig: middleware.AuthConfig{
			JWKSURL:  getEnv("BARK_OIDC_JWKS_URL", ""),
			Audience: getEnv("BARK_OIDC_AUDIENCE", "bark-api"),
		},
		PipelineToken: getEnv("BARK_PIPELINE_TOKEN", ""),
		PresignTTL:    presignTTL,
	}

	svr, err := bark.New(svrCfg, pool, s3Client, logger)
	if err != nil {
		return err
	}

	return svr.ListenAndServe(ctx)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvDuration(key string, fallbackMinutes int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n)
		}
	}
	return time.Duration(fallbackMinutes)
}
