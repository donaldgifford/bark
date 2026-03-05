// Package database provides the PostgreSQL connection pool and migration runner.
package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // pgx v5 driver for migrate
	"github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds the configuration for the database connection pool.
type Config struct {
	// DSN is the PostgreSQL connection string.
	DSN string
	// MigrationsPath is the filesystem path to the SQL migrations directory.
	MigrationsPath string
}

// Open creates and validates a new connection pool. The caller is responsible
// for calling Close when done.
func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// Migrate runs all pending up migrations from the configured migrations path.
// It is idempotent and safe to call on every startup.
func Migrate(cfg Config, logger *slog.Logger) error {
	src, err := (&file.File{}).Open("file://" + cfg.MigrationsPath)
	if err != nil {
		return fmt.Errorf("open migrations source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("file", src, cfg.DSN)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			logger.Error("close migration source", "error", srcErr)
		}
		if dbErr != nil {
			logger.Error("close migration db connection", "error", dbErr)
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	logger.Info("database migrations applied")
	return nil
}
