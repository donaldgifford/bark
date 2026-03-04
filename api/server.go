// Package api provides the bark API HTTP server.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/donaldgifford/bark/api/handlers"
	"github.com/donaldgifford/bark/api/middleware"
)

const (
	readTimeout  = 10 * time.Second
	writeTimeout = 30 * time.Second
	idleTimeout  = 120 * time.Second
)

// Config holds all configuration required to start the API server.
type Config struct {
	// Addr is the address the server listens on (e.g. ":8080").
	Addr string
	// JWTConfig configures JWT validation middleware.
	JWTConfig middleware.AuthConfig
}

// Server is the bark API HTTP server.
type Server struct {
	cfg    Config
	pool   *pgxpool.Pool
	logger *slog.Logger
	http   *http.Server
}

// New creates and configures a Server but does not start it.
// Call ListenAndServe to start accepting connections.
func New(cfg Config, pool *pgxpool.Pool, logger *slog.Logger) (*Server, error) {
	s := &Server{
		cfg:    cfg,
		pool:   pool,
		logger: logger,
	}

	mux, err := s.routes()
	if err != nil {
		return nil, fmt.Errorf("build routes: %w", err)
	}

	s.http = &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	return s, nil
}

// ListenAndServe starts the HTTP server. It blocks until the context is
// cancelled, at which point it initiates a graceful shutdown.
func (s *Server) ListenAndServe(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("api server starting", "addr", s.cfg.Addr)
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("listen: %w", err)
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("api server shutting down")
		// Parent context is cancelled; we need a fresh context for graceful shutdown.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.http.Shutdown(shutdownCtx); err != nil { //nolint:contextcheck // parent ctx is done; shutdown needs fresh ctx
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

// routes configures all routes and middleware and returns the root handler.
func (s *Server) routes() (http.Handler, error) {
	mux := http.NewServeMux()

	// Public routes (no auth).
	mux.Handle("GET /healthz", handlers.Health(s.pool))

	// Authenticated routes — wrapped below.
	authed := http.NewServeMux()

	// Wrap the authenticated mux with JWT middleware.
	authMiddleware, err := middleware.JWTAuth(s.cfg.JWTConfig, s.logger)
	if err != nil {
		return nil, fmt.Errorf("initialize jwt auth: %w", err)
	}

	// Chain middleware: request ID → logger → auth (for protected routes).
	chain := middleware.RequestID(
		middleware.Logger(s.logger)(
			authMiddleware(authed),
		),
	)

	// Mount authenticated routes under /v1/.
	mux.Handle("/v1/", chain)

	// Apply global middleware (request ID + logger) to all routes including /healthz.
	return middleware.RequestID(middleware.Logger(s.logger)(mux)), nil
}
