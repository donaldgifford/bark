// Package middleware provides HTTP middleware for the bark API server.
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const (
	// requestIDKey is the context key for the request ID.
	requestIDKey contextKey = "request_id"
	// RequestIDHeader is the HTTP header carrying the request ID.
	RequestIDHeader = "X-Request-ID"
)

// RequestID injects a unique request ID into each request context and sets
// the X-Request-ID response header. If the incoming request already carries
// the header, its value is preserved.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from the context.
// Returns an empty string if no ID is present.
func GetRequestID(ctx context.Context) string {
	id, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}
	return id
}
