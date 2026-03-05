package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/donaldgifford/bark/pkg/manifest"
	"github.com/donaldgifford/bark/pkg/types"
)

// =============================================================================
// validateRegisterRequest
// =============================================================================

func TestValidateRegisterRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     types.RegisterVersionRequest
		wantErr string
	}{
		{
			name: "valid request",
			req: types.RegisterVersionRequest{
				Version:     "1.0.0",
				BottleS3Key: "bottles/internal/tool/1.0.0/tool.tar.gz",
				SHA256:      "abc123",
				Tier:        manifest.TierInternal,
			},
			wantErr: "",
		},
		{
			name: "missing version",
			req: types.RegisterVersionRequest{
				BottleS3Key: "bottles/internal/tool/1.0.0/tool.tar.gz",
				SHA256:      "abc123",
				Tier:        manifest.TierInternal,
			},
			wantErr: "version is required",
		},
		{
			name: "missing bottle s3 key",
			req: types.RegisterVersionRequest{
				Version: "1.0.0",
				SHA256:  "abc123",
				Tier:    manifest.TierInternal,
			},
			wantErr: "bottle_s3_key is required",
		},
		{
			name: "missing sha256",
			req: types.RegisterVersionRequest{
				Version:     "1.0.0",
				BottleS3Key: "bottles/internal/tool/1.0.0/tool.tar.gz",
				Tier:        manifest.TierInternal,
			},
			wantErr: "sha256 is required",
		},
		{
			name: "missing tier",
			req: types.RegisterVersionRequest{
				Version:     "1.0.0",
				BottleS3Key: "bottles/internal/tool/1.0.0/tool.tar.gz",
				SHA256:      "abc123",
			},
			wantErr: "tier is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateRegisterRequest(&tc.req)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %q, got nil", tc.wantErr)
			}
			if err.Error() != tc.wantErr {
				t.Errorf("error = %q, want %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// =============================================================================
// approvalStatusForTier
// =============================================================================

func TestApprovalStatusForTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tier manifest.Tier
		want manifest.ApprovalStatus
	}{
		{
			name: "internal tier auto-approves",
			tier: manifest.TierInternal,
			want: manifest.ApprovalStatusApproved,
		},
		{
			name: "external-built tier auto-approves",
			tier: manifest.TierExternalBuilt,
			want: manifest.ApprovalStatusApproved,
		},
		{
			name: "external-binary tier requires manual approval",
			tier: manifest.TierExternalBinary,
			want: manifest.ApprovalStatusPending,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := approvalStatusForTier(tc.tier)
			if got != tc.want {
				t.Errorf("approvalStatusForTier(%q) = %q, want %q", tc.tier, got, tc.want)
			}
		})
	}
}

// =============================================================================
// isNotFound
// =============================================================================

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "pgx ErrNoRows returns true",
			err:  pgx.ErrNoRows,
			want: true,
		},
		{
			name: "wrapped pgx ErrNoRows returns true",
			err:  errors.Join(errors.New("scan package version"), pgx.ErrNoRows),
			want: true,
		},
		{
			name: "other error returns false",
			err:  errors.New("connection refused"),
			want: false,
		},
		{
			name: "nil error returns false",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := isNotFound(tc.err)
			if got != tc.want {
				t.Errorf("isNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// =============================================================================
// writeJSON / writeError
// =============================================================================

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		status          int
		body            any
		wantStatus      int
		wantBody        string
		wantContentType string
	}{
		{
			name:            "200 with struct",
			status:          http.StatusOK,
			body:            map[string]string{"key": "value"},
			wantStatus:      http.StatusOK,
			wantBody:        `{"key":"value"}`,
			wantContentType: "application/json",
		},
		{
			name:            "201 created",
			status:          http.StatusCreated,
			body:            map[string]int{"id": 1},
			wantStatus:      http.StatusCreated,
			wantBody:        `{"id":1}`,
			wantContentType: "application/json",
		},
		{
			name:            "400 error response",
			status:          http.StatusBadRequest,
			body:            types.ErrorResponse{Error: "bad request"},
			wantStatus:      http.StatusBadRequest,
			wantBody:        `{"error":"bad request"}`,
			wantContentType: "application/json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			writeJSON(w, tc.status, tc.body)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}

			ct := resp.Header.Get("Content-Type")
			if ct != tc.wantContentType {
				t.Errorf("Content-Type = %q, want %q", ct, tc.wantContentType)
			}

			rawBody, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}

			// Compact the actual body for comparison.
			var buf map[string]any
			if err := json.Unmarshal(rawBody, &buf); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			compact, err := json.Marshal(buf)
			if err != nil {
				t.Fatalf("marshal body: %v", err)
			}

			if string(compact) != tc.wantBody {
				t.Errorf("body = %s, want %s", compact, tc.wantBody)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		msg        string
		wantStatus int
		wantErr    string
	}{
		{
			name:       "400 bad request",
			status:     http.StatusBadRequest,
			msg:        "invalid input",
			wantStatus: http.StatusBadRequest,
			wantErr:    "invalid input",
		},
		{
			name:       "404 not found",
			status:     http.StatusNotFound,
			msg:        "package not found",
			wantStatus: http.StatusNotFound,
			wantErr:    "package not found",
		},
		{
			name:       "500 internal server error",
			status:     http.StatusInternalServerError,
			msg:        "internal server error",
			wantStatus: http.StatusInternalServerError,
			wantErr:    "internal server error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			writeError(w, tc.status, tc.msg)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}

			var errResp types.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if errResp.Error != tc.wantErr {
				t.Errorf("error message = %q, want %q", errResp.Error, tc.wantErr)
			}
		})
	}
}
