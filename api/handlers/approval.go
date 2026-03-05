package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/donaldgifford/bark/api/database"
	"github.com/donaldgifford/bark/api/middleware"
	"github.com/donaldgifford/bark/pkg/types"
)

// =============================================================================
// POST /v1/packages/{name}/versions/{version}/approve
// =============================================================================

// ApproveVersion handles POST /v1/packages/{name}/versions/{version}/approve.
// Transitions a pending external-binary version to approved and records the
// approver identity and optional reason.
func (h *Handlers) ApproveVersion(w http.ResponseWriter, r *http.Request) {
	name, version := r.PathValue("name"), r.PathValue("version")
	if name == "" || version == "" {
		writeError(w, http.StatusBadRequest, "package name and version are required")
		return
	}

	var req types.ApproveVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %s", err))
		return
	}

	caller := middleware.GetCaller(r.Context())
	actor := caller.Subject
	if actor == "" {
		actor = "unknown"
	}

	if err := database.ApproveVersion(r.Context(), h.pool, name, version, actor, req.Reason); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "pending version not found")
			return
		}
		h.internalError(w, r, "approve version", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "approved",
		"package": name,
		"version": version,
		"actor":   actor,
	})
}

// =============================================================================
// POST /v1/packages/{name}/versions/{version}/deny
// =============================================================================

// DenyVersion handles POST /v1/packages/{name}/versions/{version}/deny.
// Transitions a pending version to denied and records the denier identity and reason.
func (h *Handlers) DenyVersion(w http.ResponseWriter, r *http.Request) {
	name, version := r.PathValue("name"), r.PathValue("version")
	if name == "" || version == "" {
		writeError(w, http.StatusBadRequest, "package name and version are required")
		return
	}

	var req types.DenyVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %s", err))
		return
	}

	if req.Reason == "" {
		writeError(w, http.StatusBadRequest, "reason is required when denying a version")
		return
	}

	caller := middleware.GetCaller(r.Context())
	actor := caller.Subject
	if actor == "" {
		actor = "unknown"
	}

	if err := database.DenyVersion(r.Context(), h.pool, name, version, actor, req.Reason); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "pending version not found")
			return
		}
		h.internalError(w, r, "deny version", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "denied",
		"package": name,
		"version": version,
		"actor":   actor,
	})
}

// =============================================================================
// POST /v1/packages/{name}/versions/{version}/publish
// =============================================================================

// PublishVersion handles POST /v1/packages/{name}/versions/{version}/publish.
// Called by the post-approval pipeline after it has signed and uploaded the
// bottle to its final S3 location. Sets bottle_s3_key, cosign_sig_ref, and
// published_at, making the version installable.
func (h *Handlers) PublishVersion(w http.ResponseWriter, r *http.Request) {
	name, version := r.PathValue("name"), r.PathValue("version")
	if name == "" || version == "" {
		writeError(w, http.StatusBadRequest, "package name and version are required")
		return
	}

	var req types.PublishVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %s", err))
		return
	}

	if req.BottleS3Key == "" || req.CosignSigRef == "" {
		writeError(w, http.StatusBadRequest, "bottle_s3_key and cosign_sig_ref are required")
		return
	}

	if err := database.PublishVersion(r.Context(), h.pool, name, version, req.BottleS3Key, req.CosignSigRef); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "approved version not found")
			return
		}
		h.internalError(w, r, "publish version", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "published",
		"package": name,
		"version": version,
	})
}

// =============================================================================
// GET /v1/admin/pending
// =============================================================================

// ListPending handles GET /v1/admin/pending.
// Returns all package versions awaiting manual approval.
func (h *Handlers) ListPending(w http.ResponseWriter, r *http.Request) {
	versions, err := database.ListPendingVersions(r.Context(), h.pool)
	if err != nil {
		h.internalError(w, r, "list pending versions", err)
		return
	}

	pending := make([]types.PendingVersion, 0, len(versions))
	for i := range versions {
		pending = append(pending, types.PendingVersion{
			PackageName: versions[i].PackageName,
			Version:     versions[i].Version,
			Tier:        versions[i].Tier,
			CreatedAt:   versions[i].CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, types.ListPendingResponse{
		Pending: pending,
		Total:   len(pending),
	})
}
