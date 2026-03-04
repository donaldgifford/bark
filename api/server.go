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
	"github.com/donaldgifford/bark/api/s3"
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
	// JWTConfig configures JWT validation middleware for user requests.
	JWTConfig middleware.AuthConfig
	// PipelineToken is the bearer token expected from CI pipeline calls.
	PipelineToken string
	// PresignTTL is how long presigned S3 download URLs remain valid.
	PresignTTL time.Duration
}

// Server is the bark API HTTP server.
type Server struct {
	cfg    Config
	pool   *pgxpool.Pool
	s3     *s3.Client
	logger *slog.Logger
	http   *http.Server
}

// New creates and configures a Server but does not start it.
// Call ListenAndServe to start accepting connections.
func New(cfg Config, pool *pgxpool.Pool, s3Client *s3.Client, logger *slog.Logger) (*Server, error) {
	s := &Server{
		cfg:    cfg,
		pool:   pool,
		s3:     s3Client,
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
	h := handlers.New(s.pool, s.s3, s.logger, s.cfg.PresignTTL)

	mux := http.NewServeMux()

	// Public route (no auth).
	mux.Handle("GET /healthz", handlers.Health(s.pool))

	// JWT-authenticated routes (user-facing).
	authMW, err := middleware.JWTAuth(s.cfg.JWTConfig, s.logger)
	if err != nil {
		return nil, fmt.Errorf("initialize jwt auth: %w", err)
	}

	authed := http.NewServeMux()
	authed.HandleFunc("GET /v1/packages", h.ListPackages)
	authed.HandleFunc("GET /v1/packages/search", h.SearchPackages)
	authed.HandleFunc("GET /v1/packages/{name}", h.ResolveLatest)
	authed.HandleFunc("GET /v1/packages/{name}/{version}", h.ResolveVersion)
	authed.HandleFunc("GET /v1/signing-keys/current", h.CurrentSigningKey)

	// Admin approval endpoints — JWT-authenticated; caller identity logged in approval records.
	authed.HandleFunc("POST /v1/packages/{name}/versions/{version}/approve", h.ApproveVersion)
	authed.HandleFunc("POST /v1/packages/{name}/versions/{version}/deny", h.DenyVersion)
	authed.HandleFunc("GET /v1/admin/pending", h.ListPending)

	mux.Handle("/v1/packages", authMW(authed))
	mux.Handle("/v1/packages/", authMW(authed))
	mux.Handle("/v1/signing-keys/", authMW(authed))
	mux.Handle("/v1/admin/", authMW(authed))

	// Pipeline-authenticated routes (CI service token).
	pipeline := http.NewServeMux()
	pipeline.HandleFunc("POST /v1/packages/{name}/versions", h.RegisterVersion)
	pipelineMW := middleware.PipelineAuth(s.cfg.PipelineToken)
	mux.Handle("/v1/pipeline/", pipelineMW(pipeline))

	// POST registration and publish endpoints — pipeline token auth.
	mux.Handle("POST /v1/packages/{name}/versions", pipelineMW(http.HandlerFunc(h.RegisterVersion)))
	mux.Handle(
		"POST /v1/packages/{name}/versions/{version}/publish",
		pipelineMW(http.HandlerFunc(h.PublishVersion)),
	)

	// Apply global middleware to all routes.
	return middleware.RequestID(middleware.Logger(s.logger)(mux)), nil
}
