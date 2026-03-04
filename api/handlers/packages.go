package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/donaldgifford/bark/api/database"
	"github.com/donaldgifford/bark/api/s3"
	"github.com/donaldgifford/bark/pkg/manifest"
	"github.com/donaldgifford/bark/pkg/types"
)

// =============================================================================
// Handler wiring
// =============================================================================

// Handlers bundles all dependencies required by the package API handlers.
type Handlers struct {
	pool       *pgxpool.Pool
	s3         *s3.Client
	logger     *slog.Logger
	presignTTL time.Duration
}

// New creates a new Handlers instance.
func New(pool *pgxpool.Pool, s3Client *s3.Client, logger *slog.Logger, presignTTL time.Duration) *Handlers {
	return &Handlers{
		pool:       pool,
		s3:         s3Client,
		logger:     logger,
		presignTTL: presignTTL,
	}
}

// =============================================================================
// GET /v1/packages
// =============================================================================

// ListPackages handles GET /v1/packages.
// Returns all packages that have at least one published approved version.
func (h *Handlers) ListPackages(w http.ResponseWriter, r *http.Request) {
	pkgs, err := database.ListPublishedPackages(r.Context(), h.pool)
	if err != nil {
		h.internalError(w, r, "list packages", err)
		return
	}

	summaries := make([]types.PackageSummary, 0, len(pkgs))
	for _, p := range pkgs {
		summaries = append(summaries, types.PackageSummary{
			Name:        p.Name,
			Description: p.Description,
			Tier:        p.Tier,
		})
	}

	writeJSON(w, http.StatusOK, types.ListPackagesResponse{
		Packages: summaries,
		Total:    len(summaries),
	})
}

// =============================================================================
// GET /v1/packages/search?q=
// =============================================================================

// SearchPackages handles GET /v1/packages/search?q=.
func (h *Handlers) SearchPackages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	pkgs, err := database.SearchPackages(r.Context(), h.pool, q)
	if err != nil {
		h.internalError(w, r, "search packages", err)
		return
	}

	results := make([]types.PackageSummary, 0, len(pkgs))
	for _, p := range pkgs {
		results = append(results, types.PackageSummary{
			Name:        p.Name,
			Description: p.Description,
			Tier:        p.Tier,
		})
	}

	writeJSON(w, http.StatusOK, types.SearchResponse{
		Results: results,
		Total:   len(results),
	})
}

// =============================================================================
// GET /v1/packages/{name} and GET /v1/packages/{name}/{version}
// =============================================================================

// ResolveLatest handles GET /v1/packages/{name}.
// Returns the full manifest for the latest published version.
func (h *Handlers) ResolveLatest(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "package name is required")
		return
	}

	pv, err := database.ResolveLatestVersion(r.Context(), h.pool, name)
	if err != nil {
		h.resolveError(w, r, name, err)
		return
	}

	h.writeManifest(w, r, pv)
}

// ResolveVersion handles GET /v1/packages/{name}/{version}.
// Returns the full manifest for the requested specific version.
func (h *Handlers) ResolveVersion(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")

	if name == "" || version == "" {
		writeError(w, http.StatusBadRequest, "package name and version are required")
		return
	}

	pv, err := database.ResolveVersion(r.Context(), h.pool, name, version)
	if err != nil {
		h.resolveError(w, r, name, err)
		return
	}

	h.writeManifest(w, r, pv)
}

// writeManifest generates a presigned URL for the bottle and writes the manifest
// response. Shared by ResolveLatest and ResolveVersion.
func (h *Handlers) writeManifest(w http.ResponseWriter, r *http.Request, pv *database.PackageVersion) {
	presignedURL, err := h.s3.GetPresignedURL(r.Context(), pv.BottleS3Key)
	if err != nil {
		h.internalError(w, r, "generate presigned URL", err)
		return
	}

	var publishedAt time.Time
	if pv.PublishedAt != nil {
		publishedAt = *pv.PublishedAt
	}

	m := manifest.Manifest{
		Name:               pv.PackageName,
		Version:            pv.Version,
		Tier:               pv.Tier,
		BottleS3Key:        pv.BottleS3Key,
		BottleSHA256:       pv.SHA256,
		BottlePresignedURL: presignedURL,
		CosignSigRef:       pv.CosignSigRef,
		SBOMS3Key:          pv.SBOMS3Key,
		PublishedAt:        publishedAt,
	}

	writeJSON(w, http.StatusOK, types.ResolveResponse{Manifest: m})
}

// =============================================================================
// GET /v1/signing-keys/current
// =============================================================================

// CurrentSigningKey handles GET /v1/signing-keys/current.
// Returns the active public signing key.
func (h *Handlers) CurrentSigningKey(w http.ResponseWriter, r *http.Request) {
	key, err := database.GetActiveSigningKey(r.Context(), h.pool)
	if err != nil {
		h.resolveError(w, r, "signing key", err)
		return
	}

	writeJSON(w, http.StatusOK, types.SigningKeyResponse{
		KeyID:     key.KeyID,
		PublicKey: key.PublicKey,
	})
}

// =============================================================================
// Helpers
// =============================================================================

func (h *Handlers) internalError(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.Error(op, "error", err, "path", r.URL.Path)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func (h *Handlers) resolveError(w http.ResponseWriter, r *http.Request, resource string, err error) {
	if isNotFound(err) {
		writeError(w, http.StatusNotFound, resource+" not found")
		return
	}
	h.internalError(w, r, "resolve "+resource, err)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	//nolint:errcheck,errchkjson // response write error after headers sent; not actionable
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, types.ErrorResponse{Error: msg})
}
