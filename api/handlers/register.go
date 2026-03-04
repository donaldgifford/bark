package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/donaldgifford/bark/api/database"
	"github.com/donaldgifford/bark/pkg/manifest"
	"github.com/donaldgifford/bark/pkg/types"
)

// =============================================================================
// POST /v1/packages/{name}/versions
// =============================================================================

// RegisterVersion handles POST /v1/packages/{name}/versions.
// Requires a valid pipeline service token in the Authorization header
// (validated upstream by PipelineAuth middleware).
func (h *Handlers) RegisterVersion(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "package name is required")
		return
	}

	var req types.RegisterVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %s", err))
		return
	}

	if err := validateRegisterRequest(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Determine approval status based on tier.
	approvalStatus := approvalStatusForTier(req.Tier)

	scanResults := make([]database.ScanResultParams, 0, len(req.ScanResults))
	for _, sr := range req.ScanResults {
		scanResults = append(scanResults, database.ScanResultParams{
			Scanner:     sr.Scanner,
			ResultS3Key: sr.ResultS3Key,
			Passed:      sr.Passed,
			SummaryJSON: sr.SummaryJSON,
		})
	}

	versionID, status, err := database.RegisterVersion(r.Context(), h.pool, &database.RegisterVersionParams{
		PackageName:    name,
		Version:        req.Version,
		BottleS3Key:    req.BottleS3Key,
		SBOMS3Key:      req.SBOMS3Key,
		SHA256:         req.SHA256,
		CosignSigRef:   req.CosignSigRef,
		Tier:           req.Tier,
		ScanResults:    scanResults,
		ApprovalStatus: approvalStatus,
	})
	if err != nil {
		h.internalError(w, r, "register version", err)
		return
	}

	writeJSON(w, http.StatusCreated, types.RegisterVersionResponse{
		VersionID:      versionID.String(),
		ApprovalStatus: status,
	})
}

func validateRegisterRequest(req *types.RegisterVersionRequest) error {
	switch {
	case req.Version == "":
		return errors.New("version is required")
	case req.BottleS3Key == "":
		return errors.New("bottle_s3_key is required")
	case req.SHA256 == "":
		return errors.New("sha256 is required")
	case req.Tier == "":
		return errors.New("tier is required")
	}
	return nil
}

// approvalStatusForTier returns the initial approval status for a package based
// on its tier. Internal and external-built packages are auto-approved when scans
// pass; external-binary packages require manual approval.
func approvalStatusForTier(tier manifest.Tier) manifest.ApprovalStatus {
	if tier == manifest.TierExternalBinary {
		return manifest.ApprovalStatusPending
	}
	return manifest.ApprovalStatusApproved
}

// isNotFound returns true if err represents a "no rows" not-found condition.
func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
