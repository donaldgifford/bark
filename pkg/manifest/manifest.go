// Package manifest defines the package manifest schema — the central contract
// between the API, the CI pipeline, and the CLI.
//
// Every component in bark converges on this schema. Changes here affect all
// three components and must be reviewed across the full system.
package manifest

import "time"

// Tier represents the trust tier of a package.
// Tiers determine the scan depth, build process, and approval mechanism.
type Tier string

// Tier values.
const (
	// TierInternal represents packages produced by internal teams via GoReleaser.
	// Approval is automatic if the scan passes.
	TierInternal Tier = "internal"

	// TierExternalBuilt represents third-party packages built from source
	// in a controlled environment. Approval is automatic if scans pass policy.
	TierExternalBuilt Tier = "external-built"

	// TierExternalBinary represents pre-built binaries from third parties.
	// Approval is always manual regardless of scan outcome.
	TierExternalBinary Tier = "external-binary"
)

// ApprovalStatus is the current approval state of a package version.
type ApprovalStatus string

// ApprovalStatus values.
const (
	// ApprovalStatusPending means the version is awaiting a manual approval decision.
	ApprovalStatusPending ApprovalStatus = "pending"
	// ApprovalStatusApproved means an authorized reviewer approved the version for publishing.
	ApprovalStatusApproved ApprovalStatus = "approved"
	// ApprovalStatusDenied means an authorized reviewer denied the version; it will not be published.
	ApprovalStatusDenied ApprovalStatus = "denied"
)

// ScanStatus is the result of the vulnerability and license scan pipeline.
type ScanStatus string

// ScanStatus values.
const (
	// ScanStatusPending means scanning has not yet completed.
	ScanStatusPending ScanStatus = "pending"
	// ScanStatusPassed means all configured scan gates were satisfied.
	ScanStatusPassed ScanStatus = "passed"
	// ScanStatusFailed means at least one scan gate was not satisfied.
	ScanStatusFailed ScanStatus = "failed"
)

// LicensePolicyStatus is the outcome of the license policy evaluation.
type LicensePolicyStatus string

// LicensePolicyStatus values.
const (
	// LicensePolicyPassed means all detected licenses are on the allow list.
	LicensePolicyPassed LicensePolicyStatus = "passed"
	// LicensePolicyWarned means one or more detected licenses triggered a warning but did not block.
	LicensePolicyWarned LicensePolicyStatus = "warned"
	// LicensePolicyFailed means one or more detected licenses are on the deny list.
	LicensePolicyFailed LicensePolicyStatus = "failed"
)

// ScanSummary holds a normalized vulnerability and license scan result
// stored inside the manifest. It is tool-agnostic — the scanner field
// identifies which tool produced the result.
type ScanSummary struct {
	Scanner             string              `json:"scanner"`
	Passed              bool                `json:"passed"`
	CriticalCount       int                 `json:"critical_count"`
	HighCount           int                 `json:"high_count"`
	LicensePolicyStatus LicensePolicyStatus `json:"license_policy_status"`
}

// Manifest is the contract between the API and the CLI.
// It describes a specific version of a package and contains everything
// needed to verify, download, and install it.
//
// The BottlePresignedURL is short-lived (see BARK_PRESIGNED_URL_TTL_MINUTES)
// and must not be cached by the CLI beyond a single install operation.
type Manifest struct {
	Name               string      `json:"name"`
	Version            string      `json:"version"`
	Tier               Tier        `json:"tier"`
	BottleS3Key        string      `json:"bottle_s3_key"`
	BottleSHA256       string      `json:"bottle_sha256"`
	BottlePresignedURL string      `json:"bottle_presigned_url"`
	CosignSigRef       string      `json:"cosign_sig_ref"`
	SBOMS3Key          string      `json:"sbom_s3_key"`
	ScanSummary        ScanSummary `json:"scan_summary"`
	PublishedAt        time.Time   `json:"published_at"`
}
