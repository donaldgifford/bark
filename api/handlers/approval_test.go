package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/donaldgifford/bark/pkg/types"
)

// =============================================================================
// ApproveVersion — input validation
// =============================================================================

func TestApproveVersion_MissingPathValues(t *testing.T) {
	t.Parallel()

	// No path values set → name and version are empty.
	h := &Handlers{}
	r := httptest.NewRequest(http.MethodPost, "/v1/packages//versions//approve", http.NoBody)
	w := httptest.NewRecorder()

	h.ApproveVersion(w, r)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp types.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if errResp.Error != "package name and version are required" {
		t.Errorf("error = %q, want %q", errResp.Error, "package name and version are required")
	}
}

func TestApproveVersion_InvalidJSON(t *testing.T) {
	t.Parallel()

	h := &Handlers{}
	r := httptest.NewRequest(http.MethodPost, "/v1/packages/mytool/versions/1.0.0/approve",
		bytes.NewBufferString("not-json"))
	r.SetPathValue("name", "mytool")
	r.SetPathValue("version", "1.0.0")
	w := httptest.NewRecorder()

	h.ApproveVersion(w, r)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp types.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if errResp.Error == "" {
		t.Error("expected non-empty error message for invalid JSON")
	}
}

// =============================================================================
// DenyVersion — input validation
// =============================================================================

func TestDenyVersion_MissingPathValues(t *testing.T) {
	t.Parallel()

	h := &Handlers{}
	r := httptest.NewRequest(http.MethodPost, "/v1/packages//versions//deny", http.NoBody)
	w := httptest.NewRecorder()

	h.DenyVersion(w, r)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp types.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if errResp.Error != "package name and version are required" {
		t.Errorf("error = %q, want %q", errResp.Error, "package name and version are required")
	}
}

func TestDenyVersion_InvalidJSON(t *testing.T) {
	t.Parallel()

	h := &Handlers{}
	r := httptest.NewRequest(http.MethodPost, "/v1/packages/mytool/versions/1.0.0/deny",
		bytes.NewBufferString("not-json"))
	r.SetPathValue("name", "mytool")
	r.SetPathValue("version", "1.0.0")
	w := httptest.NewRecorder()

	h.DenyVersion(w, r)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp types.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if errResp.Error == "" {
		t.Error("expected non-empty error message for invalid JSON")
	}
}

// =============================================================================
// PublishVersion — input validation
// =============================================================================

func TestPublishVersion_MissingPathValues(t *testing.T) {
	t.Parallel()

	h := &Handlers{}
	r := httptest.NewRequest(http.MethodPost, "/v1/packages//versions//publish", http.NoBody)
	w := httptest.NewRecorder()

	h.PublishVersion(w, r)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPublishVersion_MissingFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload types.PublishVersionRequest
		wantMsg string
	}{
		{
			name:    "missing bottle_s3_key",
			payload: types.PublishVersionRequest{CosignSigRef: "abc123"},
			wantMsg: "bottle_s3_key and cosign_sig_ref are required",
		},
		{
			name:    "missing cosign_sig_ref",
			payload: types.PublishVersionRequest{BottleS3Key: "bottles/foo/1.0.0/foo.tar.gz"},
			wantMsg: "bottle_s3_key and cosign_sig_ref are required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := &Handlers{}
			body, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			r := httptest.NewRequest(http.MethodPost, "/v1/packages/mytool/versions/1.0.0/publish",
				bytes.NewReader(body))
			r.SetPathValue("name", "mytool")
			r.SetPathValue("version", "1.0.0")
			w := httptest.NewRecorder()

			h.PublishVersion(w, r)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}

			var errResp types.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if errResp.Error != tc.wantMsg {
				t.Errorf("error = %q, want %q", errResp.Error, tc.wantMsg)
			}
		})
	}
}

func TestDenyVersion_MissingReason(t *testing.T) {
	t.Parallel()

	h := &Handlers{}

	body, err := json.Marshal(types.DenyVersionRequest{Reason: ""})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/packages/mytool/versions/1.0.0/deny",
		bytes.NewReader(body))
	r.SetPathValue("name", "mytool")
	r.SetPathValue("version", "1.0.0")
	w := httptest.NewRecorder()

	h.DenyVersion(w, r)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp types.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if errResp.Error != "reason is required when denying a version" {
		t.Errorf("error = %q, want %q", errResp.Error, "reason is required when denying a version")
	}
}
