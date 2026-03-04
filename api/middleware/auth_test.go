package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/donaldgifford/bark/api/middleware"
)

func TestGetCaller_Empty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	caller := middleware.GetCaller(ctx)
	if caller.Subject != "" {
		t.Errorf("expected empty subject, got %q", caller.Subject)
	}
}

func TestGetRequestID_Empty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	id := middleware.GetRequestID(ctx)
	if id != "" {
		t.Errorf("expected empty request ID, got %q", id)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		incomingID    string
		wantGenerated bool
	}{
		{
			name:          "generates ID when none present",
			incomingID:    "",
			wantGenerated: true,
		},
		{
			name:          "preserves existing ID",
			incomingID:    "my-existing-id",
			wantGenerated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedID string
			inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				capturedID = middleware.GetRequestID(r.Context())
			})

			handler := middleware.RequestID(inner)
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if tt.incomingID != "" {
				req.Header.Set(middleware.RequestIDHeader, tt.incomingID)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if capturedID == "" {
				t.Error("request ID should be non-empty in context")
			}
			if !tt.wantGenerated && capturedID != tt.incomingID {
				t.Errorf("expected preserved ID %q, got %q", tt.incomingID, capturedID)
			}
			if rr.Header().Get(middleware.RequestIDHeader) == "" {
				t.Error("X-Request-ID header should be set in response")
			}
		})
	}
}

// TestJWTAuth_MissingToken verifies that requests without Authorization headers
// receive a 401 response. (Does not require a running JWKS server.)
func TestJWTAuth_MissingToken(t *testing.T) {
	t.Parallel()

	// Use a non-existent JWKS URL; the middleware should reject before fetching.
	// This test only verifies the missing-token branch.
	cfg := middleware.AuthConfig{
		JWKSURL:  "http://localhost:0/jwks.json",
		Audience: "test",
	}

	authMW, err := middleware.JWTAuth(cfg, newNopLogger(t))
	if err != nil {
		t.Fatalf("JWTAuth init: %v", err)
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/packages", http.NoBody)
	rr := httptest.NewRecorder()
	authMW(inner).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] == "" {
		t.Error("expected error field in response body")
	}
}
