---
id: IMPL-0002
title: "Wiz Scanner Integration"
status: Draft
author: Donald Gifford
created: 2026-02-27
---

# IMPL 0002: Wiz Scanner Integration

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-02-27

## Objective

Introduce a scanner provider abstraction to the pipeline that allows organizations to choose their vulnerability scanner. The default provider uses `syft` for SBOM generation and `grype` for vulnerability scanning — both open-source and requiring no external account. The Wiz provider wraps `wizcli` and is enabled by configuration when a Wiz account is available. Both providers produce the same normalized scan result consumed by the rest of the pipeline, so the choice of scanner requires no code changes in the ingestion logic.

**Implements:** DESIGN-0001, DESIGN-0002

**Prerequisite:** IMPL-0001 (MVP pipeline must exist before this integration)

## Scope

### In Scope

- `pipeline/scanner` package with a `Scanner` interface and normalized `ScanResult` type
- Grype provider (syft + grype) as the default implementation
- Wiz provider (wizcli) as an opt-in implementation
- Factory function that selects the provider based on `BARK_SCANNER` environment variable
- Migration to add `scanner_external_result_id` column for Wiz result IDs
- Update pipeline workflows to use the Scanner interface instead of direct tool invocations
- Unit tests for both providers using fixture JSON outputs
- Integration tests for the grype provider (no external account required)
- Integration tests for the Wiz provider guarded by `t.Skip` when credentials are absent

### Out of Scope

- Wiz continuous monitoring framework and custom Wiz controls (operational hardening, Phase 9)
- Any UI or API surface exposing scanner selection to end users
- Mixing providers within a single pipeline run (one scanner per run)
- Providers beyond grype and Wiz

## Implementation Steps

### Step 1: Define the Scanner Interface and Normalized Types

Create the `pipeline/scanner` package with the interface and shared types that both providers will implement. This must be done before either provider so both compile against the same contract.

- [ ] Create `pipeline/scanner/scanner.go` with the `Scanner` interface:
  ```go
  type Scanner interface {
      // Scan runs vulnerability scanning against the artifact at artifactPath.
      // It writes the raw tool output to rawOutputPath and returns a normalized result.
      Scan(ctx context.Context, artifactPath string, rawOutputPath string) (*ScanResult, error)

      // GenerateSBOM writes a CycloneDX JSON SBOM for the artifact to sbomOutputPath.
      // For providers that generate the SBOM as part of scanning, this may be a no-op
      // if called after Scan.
      GenerateSBOM(ctx context.Context, artifactPath string, sbomOutputPath string) error

      // Name returns the canonical scanner identifier stored in scan_results.scanner.
      Name() string
  }
  ```
- [ ] Create `pipeline/scanner/types.go` with normalized result types:
  ```go
  type ScanResult struct {
      Scanner            string
      Passed             bool
      ExternalResultID   string    // populated by Wiz only; empty for grype
      FindingsBySeverity map[Severity]int
      PolicyThreshold    Severity  // severity at which the scan fails
      SummaryJSON        []byte    // normalized summary stored in DB
  }

  type Severity string

  const (
      SeverityCritical Severity = "critical"
      SeverityHigh     Severity = "high"
      SeverityMedium   Severity = "medium"
      SeverityLow      Severity = "low"
      SeverityNone     Severity = "none"
  )
  ```
- [ ] Write unit tests for the types package (marshaling, severity ordering)

**Success Criteria:** `pipeline/scanner` compiles with no implementations. The interface and types are reviewed and agreed as the contract between the pipeline and scanner providers.

### Step 2: Implement the Grype Provider (Default)

The grype provider runs `syft` to generate the SBOM then `grype` against that SBOM. Both tools are installed in the pipeline environment via mise.

- [ ] Add `syft` and `grype` to `mise.toml` with pinned versions
- [ ] Create `pipeline/scanner/grype/provider.go` implementing `Scanner`:
  - `GenerateSBOM`: runs `syft packages <artifactPath> -o cyclonedx-json=<sbomOutputPath>`
  - `Scan`: runs `GenerateSBOM` internally if SBOM does not yet exist, then runs `grype sbom:<sbomPath> -o json --file <rawOutputPath>`; parses the output into `ScanResult`
  - `Name`: returns `"grype"`
- [ ] Create `pipeline/scanner/grype/parser.go`:
  - Parses grype's JSON output (matches, vulnerabilities, severity) into `ScanResult`
  - Applies `GRYPE_FAIL_ON_SEVERITY` env var (default: `critical`) to set `Passed`
  - Maps grype severity strings to the canonical `Severity` type
- [ ] Create `pipeline/scanner/testdata/grype/` with fixture grype JSON outputs:
  - `pass.json` — output with no critical findings
  - `fail-critical.json` — output with at least one critical CVE
  - `empty.json` — output for an artifact with no vulnerabilities
- [ ] Write unit tests for `grype/parser.go` using the fixture files
- [ ] Write integration tests for `grype/provider.go` that run real syft and grype against a minimal test tarball fixture (a small static binary with no deps for speed)

**Success Criteria:** `GrypeProvider.Scan` correctly parses fixture outputs into `ScanResult` with the right pass/fail decision. The integration test runs real syft and grype binaries and produces a valid `ScanResult`.

### Step 3: Implement the Wiz Provider

The Wiz provider wraps `wizcli artifact scan`, which performs SBOM generation and vulnerability scanning in a single invocation. The SBOM is exported as a side-effect of the scan.

- [ ] Create `pipeline/scanner/wiz/provider.go` implementing `Scanner`:
  - `Scan`: runs `wizcli artifact scan --path <artifactPath> --policy-name <WIZCLI_POLICY_NAME> --format json --output <rawOutputPath> --export-sbom cyclonedx --sbom-output <sbomOutputPath>`; captures stdout/stderr for logging; parses output into `ScanResult`
  - `GenerateSBOM`: if called after `Scan`, the SBOM already exists at the configured path — return nil; if called before, run a scan with `--sbom-only` flag
  - `Name`: returns `"wiz"`
- [ ] Create `pipeline/scanner/wiz/parser.go`:
  - Parses wizcli JSON output into `ScanResult`
  - Extracts `ExternalResultID` from the wizcli result (the Wiz platform scan ID)
  - Maps Wiz severity levels to the canonical `Severity` type
  - Sets `Passed` based on the `result.passed` field in wizcli output (Wiz evaluates the policy server-side)
- [ ] Create `pipeline/scanner/testdata/wiz/` with fixture wizcli JSON outputs:
  - `pass.json` — output with `result.passed: true`
  - `fail.json` — output with `result.passed: false` and critical findings
- [ ] Write unit tests for `wiz/parser.go` using fixture files
- [ ] Write integration tests for `wiz/provider.go` guarded with:
  ```go
  if os.Getenv("WIZCLI_TOKEN") == "" {
      t.Skip("WIZCLI_TOKEN not set; skipping Wiz integration tests")
  }
  ```
- [ ] Document required environment variables for Wiz provider in `.env.local.example`:
  - `WIZCLI_TOKEN` — service account token
  - `WIZCLI_API_URL` — Wiz API endpoint (default: `https://api.wiz.io`)
  - `WIZCLI_POLICY_NAME` — the Wiz vulnerability policy to evaluate against

**Success Criteria:** `WizProvider` unit tests pass using fixture outputs. Integration tests are correctly skipped when `WIZCLI_TOKEN` is absent. `ExternalResultID` is populated in `ScanResult` for Wiz results and empty for grype results.

### Step 4: Provider Factory and Configuration

A single factory function reads the environment and returns the configured provider. The rest of the pipeline never imports a provider package directly — only the factory.

- [ ] Create `pipeline/scanner/factory.go`:
  ```go
  // NewScanner returns a Scanner for the provider named by BARK_SCANNER.
  // Valid values: "grype" (default), "wiz".
  // Returns an error if BARK_SCANNER=wiz but required Wiz env vars are missing.
  func NewScanner(cfg Config) (Scanner, error)
  ```
- [ ] `Config` reads from environment: `BARK_SCANNER`, `GRYPE_FAIL_ON_SEVERITY`, `WIZCLI_TOKEN`, `WIZCLI_API_URL`, `WIZCLI_POLICY_NAME`
- [ ] Factory fails fast with a clear error if `BARK_SCANNER=wiz` and `WIZCLI_TOKEN` is not set
- [ ] Factory logs the selected provider on initialization
- [ ] Add `BARK_SCANNER` to `.env.local.example` with value `grype` and a comment explaining the `wiz` option
- [ ] Write unit tests for the factory: correct provider returned per config, error on missing Wiz credentials, default to grype when `BARK_SCANNER` is unset

**Success Criteria:** `NewScanner` returns a `GrypeProvider` by default and when `BARK_SCANNER=grype`. Returns a `WizProvider` when `BARK_SCANNER=wiz` and `WIZCLI_TOKEN` is set. Returns a descriptive error when `BARK_SCANNER=wiz` but credentials are missing.

### Step 5: Database Migration

Add a nullable column to `scan_results` for the external scanner result ID. This is populated only by the Wiz provider and used for cross-referencing results in the Wiz platform console.

- [ ] Write migration: add `scanner_external_result_id TEXT NULL` to `scan_results`
- [ ] Write corresponding down migration
- [ ] Update `S3Client` registration call to include `scanner_external_result_id` if non-empty

**Success Criteria:** Migration runs forward and backward cleanly. Grype scan records have `NULL` in `scanner_external_result_id`. Wiz scan records have the Wiz platform scan ID populated.

### Step 6: Pipeline Integration

Replace the direct `syft`/`grype`/`wizcli` invocations in the pipeline workflows and `pipeline/ingest` package with calls through the `Scanner` interface.

- [ ] Update `pipeline/ingest` to accept a `scanner.Scanner` argument (injected, not constructed internally)
- [ ] Replace inline scan logic in the internal tier workflow with: construct scanner via factory, call `scanner.Scan`, call `scanner.GenerateSBOM`, upload outputs
- [ ] Replace inline scan logic in the external-binary tier workflow the same way
- [ ] Update S3 upload step: raw scan output uploaded as `scans/{name}/{version}/{scanner.Name()}.json` so the filename is always the tool that produced it
- [ ] Update the PR comment step to include the scanner name in the report header
- [ ] Update GitHub Actions workflows to pass `BARK_SCANNER` and conditionally pass Wiz env vars when the secret is available:
  ```yaml
  env:
    BARK_SCANNER: ${{ secrets.WIZCLI_TOKEN != '' && 'wiz' || 'grype' }}
    WIZCLI_TOKEN: ${{ secrets.WIZCLI_TOKEN }}
    WIZCLI_POLICY_NAME: ${{ vars.WIZCLI_POLICY_NAME }}
  ```

**Success Criteria:** The internal tier pipeline runs end-to-end using the grype provider with no `BARK_SCANNER` set. The same pipeline runs end-to-end using the Wiz provider when `WIZCLI_TOKEN` is set in CI secrets. The scanner name appears in the PR comment and in the S3 key for scan output.

### Step 7: Document Wiz-Exclusive Benefits

Wiz provides capabilities that have no grype equivalent. Document these clearly so teams understand what they get (and don't get) from each provider.

- [ ] Document in `docs/design/` or operator guide:
  - **Continuous monitoring**: Wiz continuously re-evaluates published packages against new CVE disclosures. Grype scans only at publish time — new CVEs disclosed after publication are not automatically detected without re-scanning
  - **Wiz framework controls**: S3 bucket encryption posture, bucket access policy, signing key rotation schedule — Wiz-platform-managed compliance view. No grype equivalent
  - **Wiz result ID**: Wiz scans are tracked in the Wiz console with a stable ID. Grype results exist only as JSON in S3
- [ ] Add a note in `CLAUDE.md` listing `BARK_SCANNER` as a configurable env var

**Success Criteria:** The distinction between provider capabilities is documented. An operator reading the docs can understand what they gain or lose by switching providers.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `pipeline/scanner/scanner.go` | Create | `Scanner` interface and `Config` type |
| `pipeline/scanner/types.go` | Create | `ScanResult`, `Severity`, normalized output types |
| `pipeline/scanner/factory.go` | Create | `NewScanner` factory, reads `BARK_SCANNER` env var |
| `pipeline/scanner/grype/provider.go` | Create | Grype provider: syft SBOM + grype vuln scan |
| `pipeline/scanner/grype/parser.go` | Create | Parse grype JSON into `ScanResult` |
| `pipeline/scanner/wiz/provider.go` | Create | Wiz provider: wraps `wizcli artifact scan` |
| `pipeline/scanner/wiz/parser.go` | Create | Parse wizcli JSON into `ScanResult` |
| `pipeline/scanner/testdata/grype/` | Create | Fixture grype JSON outputs (pass, fail, empty) |
| `pipeline/scanner/testdata/wiz/` | Create | Fixture wizcli JSON outputs (pass, fail) |
| `pipeline/ingest/*.go` | Modify | Accept `scanner.Scanner` interface; remove direct tool calls |
| `.github/workflows/pipeline-internal.yml` | Modify | Add `BARK_SCANNER` and conditional Wiz env vars |
| `.github/workflows/pipeline-external-binary.yml` | Modify | Same |
| `.env.local.example` | Modify | Add `BARK_SCANNER`, `WIZCLI_TOKEN`, `WIZCLI_API_URL`, `WIZCLI_POLICY_NAME` |
| `mise.toml` | Modify | Add `syft` and `grype` with pinned versions |
| DB migration | Create | Add `scanner_external_result_id TEXT NULL` to `scan_results` |

## Testing Plan

- [ ] Unit: `GrypeParser` correctly maps `pass.json` to `ScanResult{Passed: true}`
- [ ] Unit: `GrypeParser` correctly maps `fail-critical.json` to `ScanResult{Passed: false}` with critical count populated
- [ ] Unit: `WizParser` correctly maps `pass.json` to `ScanResult{Passed: true, ExternalResultID: "<id>"}`
- [ ] Unit: `WizParser` correctly maps `fail.json` to `ScanResult{Passed: false}`
- [ ] Unit: factory returns `GrypeProvider` when `BARK_SCANNER` is unset or `"grype"`
- [ ] Unit: factory returns `WizProvider` when `BARK_SCANNER=wiz` and `WIZCLI_TOKEN` is set
- [ ] Unit: factory returns error when `BARK_SCANNER=wiz` and `WIZCLI_TOKEN` is missing
- [ ] Contract: both providers return `ScanResult` with no nil pointer fields populated
- [ ] Integration: `GrypeProvider.Scan` runs real syft and grype against a minimal test tarball and returns a valid `ScanResult`
- [ ] Integration: `WizProvider.Scan` skipped when `WIZCLI_TOKEN` is unset (`t.Skip`)
- [ ] Integration: pipeline internal tier workflow runs end-to-end with grype provider in CI (no secrets required)

## Rollback Plan

The `Scanner` interface decouples the provider from the pipeline. To revert from Wiz to grype:

- Remove or unset `BARK_SCANNER` (defaults to grype)
- Remove `WIZCLI_TOKEN` from CI secrets
- No code changes required

Existing scan result records in the database and S3 are not affected — the `scanner` column already records which tool produced each result. Historical Wiz results remain accessible in S3 at `scans/{name}/{version}/wiz.json`.

## Dependencies

- IMPL-0001 must be complete (`pipeline/ingest` must exist before it can be refactored)
- `syft` and `grype` — added to `mise.toml`; no external account required
- `wizcli` — optional; only required when `BARK_SCANNER=wiz`
- Wiz platform account with a configured vulnerability policy (for Wiz provider)
- Wiz service account credentials stored as a CI secret (`WIZCLI_TOKEN`)

## References

- [Initial Design](../design/0001-initial-design.md)
- [Technical Plan](../design/0002-technical-plan.md)
- [MVP Implementation Plan](0001-mvp-implementation-plan.md)
