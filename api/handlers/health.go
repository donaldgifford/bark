// Package handlers provides HTTP handlers for the bark API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// healthResponse is the response body for GET /healthz.
type healthResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

// Health returns an http.Handler that pings the database and reports
// overall service health.
//
// Returns 200 if the database is reachable, 503 otherwise.
func Health(pool *pgxpool.Pool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		dbStatus := "ok"
		if err := pool.Ping(ctx); err != nil {
			dbStatus = "unreachable"
		}

		status := http.StatusOK
		resp := healthResponse{Status: "ok", DB: dbStatus}
		if dbStatus != "ok" {
			status = http.StatusServiceUnavailable
			resp.Status = "degraded"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		//nolint:errcheck // response write error after headers are sent; not actionable
		_ = json.NewEncoder(w).Encode(resp)
	})
}
