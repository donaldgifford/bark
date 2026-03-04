package middleware_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/donaldgifford/bark/api/middleware"
)

// newNopLogger creates a slog.Logger that discards all output. Used in tests
// where structured log output is not under test.
func newNopLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(nopWriter{}, nil))
}

// nopWriter satisfies io.Writer, discarding all bytes.
type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestLoggerMiddleware(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	handler := middleware.Logger(newNopLogger(t))(inner)
	req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Errorf("expected status %d, got %d", http.StatusTeapot, rr.Code)
	}
}
