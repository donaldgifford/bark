// Package types defines the shared API request and response types for bark.
// These types form the HTTP contract between the API server, the CI pipeline,
// and the CLI client. All fields use JSON snake_case tags.
package types

import (
	"encoding/json"
	"time"

	"github.com/donaldgifford/bark/pkg/manifest"
)

// =============================================================================
// Package listing and search
// =============================================================================

// PackageSummary is a lightweight package descriptor used in list and search
// responses. It does not include the full manifest or presigned URLs.
type PackageSummary struct {
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Description string        `json:"description"`
	Tier        manifest.Tier `json:"tier"`
	PublishedAt time.Time     `json:"published_at"`
}

// ListPackagesResponse is the response body for GET /v1/packages.
type ListPackagesResponse struct {
	Packages []PackageSummary `json:"packages"`
	Total    int              `json:"total"`
}

// SearchResponse is the response body for GET /v1/packages/search?q=.
type SearchResponse struct {
	Results []PackageSummary `json:"results"`
	Total   int              `json:"total"`
}

// =============================================================================
// Package resolution (manifest delivery)
// =============================================================================

// ResolveResponse is the response body for GET /v1/packages/{name} and
// GET /v1/packages/{name}/{version}. It contains the full manifest including
// the short-lived presigned download URL.
type ResolveResponse struct {
	Manifest manifest.Manifest `json:"manifest"`
}

// =============================================================================
// Pipeline: version registration
// =============================================================================

// ScanResultRef references a scan result artifact stored in S3.
type ScanResultRef struct {
	Scanner     string `json:"scanner"`
	ResultS3Key string `json:"result_s3_key"`
	Passed      bool   `json:"passed"`
	// SummaryJSON is the normalized scan summary stored in the database.
	// It must be a valid JSON object.
	SummaryJSON json.RawMessage `json:"summary_json,omitempty"`
}

// RegisterVersionRequest is the request body for
// POST /v1/packages/{name}/versions.
// Sent by the CI pipeline after a bottle has been scanned and signed.
type RegisterVersionRequest struct {
	Version      string          `json:"version"`
	BottleS3Key  string          `json:"bottle_s3_key"`
	SHA256       string          `json:"sha256"`
	CosignSigRef string          `json:"cosign_sig_ref"`
	SBOMS3Key    string          `json:"sbom_s3_key"`
	ScanResults  []ScanResultRef `json:"scan_results"`
	Tier         manifest.Tier   `json:"tier"`
}

// RegisterVersionResponse is the response body for
// POST /v1/packages/{name}/versions.
type RegisterVersionResponse struct {
	VersionID      string                  `json:"version_id"`
	ApprovalStatus manifest.ApprovalStatus `json:"approval_status"`
}

// =============================================================================
// Admin: approval workflow
// =============================================================================

// ApproveVersionRequest is the request body for
// POST /v1/packages/{name}/versions/{version}/approve.
type ApproveVersionRequest struct {
	Reason string `json:"reason,omitempty"`
}

// DenyVersionRequest is the request body for
// POST /v1/packages/{name}/versions/{version}/deny.
type DenyVersionRequest struct {
	Reason string `json:"reason"`
}

// PendingVersion is a version awaiting manual approval, returned by
// GET /v1/admin/pending.
type PendingVersion struct {
	PackageName string          `json:"package_name"`
	Version     string          `json:"version"`
	Tier        manifest.Tier   `json:"tier"`
	ScanResults []ScanResultRef `json:"scan_results"`
	RequestedBy string          `json:"requested_by"`
	CreatedAt   time.Time       `json:"created_at"`
}

// ListPendingResponse is the response body for GET /v1/admin/pending.
type ListPendingResponse struct {
	Pending []PendingVersion `json:"pending"`
	Total   int              `json:"total"`
}

// =============================================================================
// Signing keys
// =============================================================================

// SigningKeyResponse is the response body for GET /v1/signing-keys/current.
type SigningKeyResponse struct {
	KeyID     string `json:"key_id"`
	PublicKey string `json:"public_key"`
}

// =============================================================================
// Errors
// =============================================================================

// ErrorResponse is the standard error response body for all API endpoints.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}
