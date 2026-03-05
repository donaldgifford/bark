// Package database provides the PostgreSQL connection pool, migration runner,
// and repository functions for all bark data models.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/donaldgifford/bark/pkg/manifest"
)

// =============================================================================
// Domain types
// =============================================================================

// Package represents a row in the packages table.
type Package struct {
	ID          uuid.UUID
	Name        string
	Description string
	Tier        manifest.Tier
	CreatedAt   time.Time
}

// PackageVersion represents a row in the package_versions table, joined with
// the parent package name.
type PackageVersion struct {
	ID             uuid.UUID
	PackageID      uuid.UUID
	PackageName    string
	Version        string
	BottleS3Key    string
	SBOMS3Key      string
	SHA256         string
	CosignSigRef   string
	Tier           manifest.Tier
	ScanStatus     manifest.ScanStatus
	ApprovalStatus manifest.ApprovalStatus
	ApprovedBy     *string
	ApprovedAt     *time.Time
	PublishedAt    *time.Time
	CreatedAt      time.Time
}

// ScanResult represents a row in the scan_results table.
type ScanResult struct {
	ID               uuid.UUID
	PackageVersionID uuid.UUID
	Scanner          string
	ResultS3Key      string
	Passed           bool
	SummaryJSON      []byte
	ScannedAt        time.Time
}

// SigningKey represents a row in the signing_keys table.
type SigningKey struct {
	ID        uuid.UUID
	KeyID     string
	PublicKey string
	Active    bool
	CreatedAt time.Time
	RotatedAt *time.Time
}

// =============================================================================
// Package queries
// =============================================================================

// ListPublishedPackages returns all packages that have at least one published,
// approved version, ordered alphabetically by name.
func ListPublishedPackages(ctx context.Context, pool *pgxpool.Pool) ([]Package, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT ON (p.name) p.id, p.name, p.description, p.tier, p.created_at
		FROM packages p
		JOIN package_versions pv ON pv.package_id = p.id
		WHERE pv.approval_status = 'approved' AND pv.published_at IS NOT NULL
		ORDER BY p.name
	`)
	if err != nil {
		return nil, fmt.Errorf("list packages: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Package, error) {
		var p Package
		return p, row.Scan(&p.ID, &p.Name, &p.Description, &p.Tier, &p.CreatedAt)
	})
}

// SearchPackages searches packages by name prefix or description substring.
func SearchPackages(ctx context.Context, pool *pgxpool.Pool, query string) ([]Package, error) {
	pattern := "%" + query + "%"
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT ON (p.name) p.id, p.name, p.description, p.tier, p.created_at
		FROM packages p
		JOIN package_versions pv ON pv.package_id = p.id
		WHERE pv.approval_status = 'approved' AND pv.published_at IS NOT NULL
		  AND (p.name ILIKE $1 OR p.description ILIKE $1)
		ORDER BY p.name
	`, pattern)
	if err != nil {
		return nil, fmt.Errorf("search packages: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Package, error) {
		var p Package
		return p, row.Scan(&p.ID, &p.Name, &p.Description, &p.Tier, &p.CreatedAt)
	})
}

// =============================================================================
// Package version queries
// =============================================================================

// ResolveLatestVersion returns the most recently published, approved version
// of the named package.
func ResolveLatestVersion(ctx context.Context, pool *pgxpool.Pool, name string) (*PackageVersion, error) {
	row := pool.QueryRow(ctx, `
		SELECT pv.id, pv.package_id, p.name, pv.version,
		       pv.bottle_s3_key, pv.sbom_s3_key, pv.sha256, pv.cosign_sig_ref,
		       pv.tier, pv.scan_status, pv.approval_status,
		       pv.approved_by, pv.approved_at, pv.published_at, pv.created_at
		FROM package_versions pv
		JOIN packages p ON p.id = pv.package_id
		WHERE p.name = $1
		  AND pv.approval_status = 'approved'
		  AND pv.published_at IS NOT NULL
		ORDER BY pv.published_at DESC
		LIMIT 1
	`, name)

	return scanPackageVersion(row)
}

// ResolveVersion returns a specific version of the named package.
func ResolveVersion(ctx context.Context, pool *pgxpool.Pool, name, version string) (*PackageVersion, error) {
	row := pool.QueryRow(ctx, `
		SELECT pv.id, pv.package_id, p.name, pv.version,
		       pv.bottle_s3_key, pv.sbom_s3_key, pv.sha256, pv.cosign_sig_ref,
		       pv.tier, pv.scan_status, pv.approval_status,
		       pv.approved_by, pv.approved_at, pv.published_at, pv.created_at
		FROM package_versions pv
		JOIN packages p ON p.id = pv.package_id
		WHERE p.name = $1 AND pv.version = $2
		  AND pv.approval_status = 'approved'
		  AND pv.published_at IS NOT NULL
		LIMIT 1
	`, name, version)

	return scanPackageVersion(row)
}

func scanPackageVersion(row pgx.Row) (*PackageVersion, error) {
	var pv PackageVersion
	err := row.Scan(
		&pv.ID, &pv.PackageID, &pv.PackageName, &pv.Version,
		&pv.BottleS3Key, &pv.SBOMS3Key, &pv.SHA256, &pv.CosignSigRef,
		&pv.Tier, &pv.ScanStatus, &pv.ApprovalStatus,
		&pv.ApprovedBy, &pv.ApprovedAt, &pv.PublishedAt, &pv.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan package version: %w", err)
	}
	return &pv, nil
}

// RegisterVersionParams holds the inputs for creating a new package version.
type RegisterVersionParams struct {
	PackageName  string
	Description  string
	Version      string
	BottleS3Key  string
	SBOMS3Key    string
	SHA256       string
	CosignSigRef string
	Tier         manifest.Tier
	ScanResults  []ScanResultParams
	// ApprovalStatus is set to 'approved' for automatic tiers and 'pending' for manual tiers.
	ApprovalStatus manifest.ApprovalStatus
}

// ScanResultParams holds scan result data for a single scanner.
type ScanResultParams struct {
	Scanner     string
	ResultS3Key string
	Passed      bool
	SummaryJSON []byte
}

// RegisterVersion creates or updates the package record and inserts a new
// package version with the provided scan results. Returns the new version ID
// and its approval status.
func RegisterVersion(
	ctx context.Context,
	pool *pgxpool.Pool,
	p *RegisterVersionParams,
) (versionID uuid.UUID, approvalStatus manifest.ApprovalStatus, err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return uuid.UUID{}, "", fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx) //nolint:errcheck // rollback on error path; original error takes precedence
		}
	}()

	// Upsert the package record.
	var pkgID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO packages (name, description, tier)
		VALUES ($1, $2, $3)
		ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description
		RETURNING id
	`, p.PackageName, p.Description, p.Tier).Scan(&pkgID)
	if err != nil {
		return uuid.UUID{}, "", fmt.Errorf("upsert package: %w", err)
	}

	// Determine published_at: immediate for auto-approved, null for pending.
	var publishedAt *time.Time
	if p.ApprovalStatus == manifest.ApprovalStatusApproved {
		now := time.Now()
		publishedAt = &now
	}

	// Insert the package version.
	err = tx.QueryRow(ctx, `
		INSERT INTO package_versions
		  (package_id, version, bottle_s3_key, sbom_s3_key, sha256,
		   cosign_sig_ref, tier, scan_status, approval_status, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`,
		pkgID, p.Version, p.BottleS3Key, p.SBOMS3Key, p.SHA256,
		p.CosignSigRef, p.Tier, manifest.ScanStatusPassed, p.ApprovalStatus, publishedAt,
	).Scan(&versionID)
	if err != nil {
		return uuid.UUID{}, "", fmt.Errorf("insert package version: %w", err)
	}

	// Insert scan results.
	for _, sr := range p.ScanResults {
		if _, err = tx.Exec(ctx, `
			INSERT INTO scan_results (package_version_id, scanner, result_s3_key, passed, summary_json)
			VALUES ($1, $2, $3, $4, $5)
		`, versionID, sr.Scanner, sr.ResultS3Key, sr.Passed, sr.SummaryJSON); err != nil {
			return uuid.UUID{}, "", fmt.Errorf("insert scan result for %s: %w", sr.Scanner, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.UUID{}, "", fmt.Errorf("commit transaction: %w", err)
	}

	return versionID, p.ApprovalStatus, nil
}

// =============================================================================
// Approval workflow queries
// =============================================================================

// ApproveVersion transitions a pending version to approved, sets published_at
// to now, and inserts an approval record.
func ApproveVersion(
	ctx context.Context,
	pool *pgxpool.Pool,
	name, version, actor, reason string,
) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx) //nolint:errcheck // rollback on error path; original error takes precedence
		}
	}()

	now := time.Now()

	var versionID uuid.UUID

	// Approve records the decision; published_at remains NULL until the
	// post-approval pipeline signs and uploads the bottle (PublishVersion).
	err = tx.QueryRow(ctx, `
		UPDATE package_versions pv
		SET approval_status = 'approved',
		    approved_by     = $3,
		    approved_at     = $4
		FROM packages p
		WHERE pv.package_id = p.id
		  AND p.name = $1
		  AND pv.version = $2
		  AND pv.approval_status = 'pending'
		RETURNING pv.id
	`, name, version, actor, now).Scan(&versionID)
	if err != nil {
		return fmt.Errorf("approve version: %w", err)
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO approval_records (package_version_id, action, actor, reason)
		VALUES ($1, 'approved', $2, $3)
	`, versionID, actor, reason); err != nil {
		return fmt.Errorf("insert approval record: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit approve: %w", err)
	}

	return nil
}

// DenyVersion transitions a pending version to denied and inserts a denial record.
func DenyVersion(
	ctx context.Context,
	pool *pgxpool.Pool,
	name, version, actor, reason string,
) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx) //nolint:errcheck // rollback on error path; original error takes precedence
		}
	}()

	var versionID uuid.UUID

	err = tx.QueryRow(ctx, `
		UPDATE package_versions pv
		SET approval_status = 'denied'
		FROM packages p
		WHERE pv.package_id = p.id
		  AND p.name = $1
		  AND pv.version = $2
		  AND pv.approval_status = 'pending'
		RETURNING pv.id
	`, name, version).Scan(&versionID)
	if err != nil {
		return fmt.Errorf("deny version: %w", err)
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO approval_records (package_version_id, action, actor, reason)
		VALUES ($1, 'denied', $2, $3)
	`, versionID, actor, reason); err != nil {
		return fmt.Errorf("insert denial record: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit deny: %w", err)
	}

	return nil
}

// PublishVersion updates the bottle_s3_key, cosign_sig_ref, and published_at
// fields for a version that has already been approved. Called by the
// post-approval pipeline after the bottle has been signed and uploaded.
func PublishVersion(
	ctx context.Context,
	pool *pgxpool.Pool,
	name, version, bottleS3Key, cosignSigRef string,
) error {
	tag, err := pool.Exec(ctx, `
		UPDATE package_versions pv
		SET bottle_s3_key  = $3,
		    cosign_sig_ref = $4,
		    published_at   = NOW()
		FROM packages p
		WHERE pv.package_id = p.id
		  AND p.name = $1
		  AND pv.version = $2
		  AND pv.approval_status = 'approved'
		  AND pv.published_at IS NULL
	`, name, version, bottleS3Key, cosignSigRef)
	if err != nil {
		return fmt.Errorf("publish version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("publish version: %w", pgx.ErrNoRows)
	}
	return nil
}

// ListPendingVersions returns all package versions with approval_status = 'pending',
// ordered oldest first.
func ListPendingVersions(ctx context.Context, pool *pgxpool.Pool) ([]PackageVersion, error) {
	rows, err := pool.Query(ctx, `
		SELECT pv.id, pv.package_id, p.name, pv.version,
		       pv.bottle_s3_key, pv.sbom_s3_key, pv.sha256, pv.cosign_sig_ref,
		       pv.tier, pv.scan_status, pv.approval_status,
		       pv.approved_by, pv.approved_at, pv.published_at, pv.created_at
		FROM package_versions pv
		JOIN packages p ON p.id = pv.package_id
		WHERE pv.approval_status = 'pending'
		ORDER BY pv.created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list pending versions: %w", err)
	}

	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (PackageVersion, error) {
		var pv PackageVersion
		return pv, row.Scan(
			&pv.ID, &pv.PackageID, &pv.PackageName, &pv.Version,
			&pv.BottleS3Key, &pv.SBOMS3Key, &pv.SHA256, &pv.CosignSigRef,
			&pv.Tier, &pv.ScanStatus, &pv.ApprovalStatus,
			&pv.ApprovedBy, &pv.ApprovedAt, &pv.PublishedAt, &pv.CreatedAt,
		)
	})
}

// =============================================================================
// Signing key queries
// =============================================================================

// GetActiveSigningKey returns the currently active signing key.
func GetActiveSigningKey(ctx context.Context, pool *pgxpool.Pool) (*SigningKey, error) {
	row := pool.QueryRow(ctx, `
		SELECT id, key_id, public_key, active, created_at, rotated_at
		FROM signing_keys
		WHERE active = true
		ORDER BY created_at DESC
		LIMIT 1
	`)

	var k SigningKey
	if err := row.Scan(&k.ID, &k.KeyID, &k.PublicKey, &k.Active, &k.CreatedAt, &k.RotatedAt); err != nil {
		return nil, fmt.Errorf("get active signing key: %w", err)
	}
	return &k, nil
}

// =============================================================================
// Scan result queries
// =============================================================================

// ListScanResults returns all scan results for a given package version.
func ListScanResults(ctx context.Context, pool *pgxpool.Pool, versionID uuid.UUID) ([]ScanResult, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, package_version_id, scanner, result_s3_key, passed, summary_json, scanned_at
		FROM scan_results
		WHERE package_version_id = $1
	`, versionID)
	if err != nil {
		return nil, fmt.Errorf("list scan results: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ScanResult, error) {
		var sr ScanResult
		return sr, row.Scan(
			&sr.ID, &sr.PackageVersionID, &sr.Scanner, &sr.ResultS3Key,
			&sr.Passed, &sr.SummaryJSON, &sr.ScannedAt,
		)
	})
}
