---
id: IMPL-0001
title: "MVP Implementation Plan"
status: Draft
author: Donald Gifford
created: 2026-02-27
---

# IMPL 0001: MVP Implementation Plan

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-02-27

## Objective

Implement the internal package distribution platform from initial project setup through a fully functional MVP covering all three package tiers. The MVP is complete when developers can authenticate and install packages, internal team packages flow through the pipeline automatically, and external-binary packages can be requested, scanned, manually approved, and installed.

**Implements:** DESIGN-0001, DESIGN-0002

## Scope

### In Scope

- Phases 0–6: project setup through external-binary tier and approval workflow
- All three package tiers: internal, external-built (partial), external-binary
- Complete install flow: OIDC auth, manifest resolution, cosign verification, download, install
- CI pipeline for internal tier and external-binary tier
- Manual approval workflow for external-binary packages

### Out of Scope

- Phases 7–9: external-built tier pipeline, Pkgfile sync polish, and full operational hardening
- Wiz scanner integration (covered in IMPL-0002)
- Linux and Windows support
- Fleet management or telemetry

## Implementation Steps

### Phase 0: Project Setup and Conventions

Establish the repository structure, tooling, and conventions before any feature work begins.

- [x] Create monorepo with top-level directories: `cli/`, `api/`, `pipeline/`, `infra/`, `formulas/`, `docs/`
- [x] Initialize Go modules in `cli/` and `api/` with consistent Go version pinned in `.go-version`
- [x] Set up `docker-compose.yml` with LocalStack service
- [x] Write LocalStack init script that creates the `homebrew-bottles` bucket with versioning enabled
- [x] Write `Makefile` with targets: `infra-up`, `infra-down`, `infra-seed`, `api-dev`, `cli-build`, `test`, `lint`
- [x] Configure `.env.local.example` with all required environment variables documented
- [x] Set up `golangci-lint` with agreed linting rules for both modules
- [ ] Set up pre-commit hooks: lint, format, no committed secrets
- [x] Create the package manifest schema as a Go struct in a shared `pkg/manifest` module and document it as the contract
- [x] Define S3 key structure: `bottles/{name}/{version}/{name}-{version}.arm64_sonoma.bottle.tar.gz`, `sboms/{name}/{version}/sbom.cdx.json`, `scans/{name}/{version}/grype.json`, `scans/{name}/{version}/scancode.json`
- [x] Define API request/response types in a shared `pkg/types` module
- [x] Document environment variable conventions in `docs/env-conventions.md`
- [x] Set up GitHub Actions CI: lint and test on PR, runs against LocalStack for integration tests

**Success Criteria:** Running `make infra-up` starts LocalStack and creates the S3 bucket. Running `make infra-down` tears it down cleanly. The manifest schema, S3 key structure, and API types are committed and reviewed by all contributors. The CI pipeline runs lint and tests on a hello-world Go file in each module.

### Phase 1: Database and API Foundation

The API is the authority for all package state. Build the data layer and core server structure before any endpoints.

- [x] Choose and configure database migration tool (goose or migrate)
- [x] Write initial migration: `packages` table (id, name, description, tier, created_at)
- [x] Write migration: `package_versions` table (id, package_id, version, bottle_s3_key, sbom_s3_key, sha256, cosign_sig_ref, tier, scan_status, approval_status, approved_by, approved_at, published_at, created_at)
- [x] Write migration: `scan_results` table (id, package_version_id, scanner, result_s3_key, passed, summary_json, scanned_at)
- [x] Write migration: `approval_records` table (id, package_version_id, action, actor, reason, created_at)
- [x] Write migration: `signing_keys` table (id, key_id, public_key, active, created_at, rotated_at)
- [x] Implement database connection pool with health check and graceful shutdown
- [x] Set up API server with graceful shutdown, request logging middleware, and request ID middleware
- [x] Implement health check endpoint `GET /healthz` returning 200 with db ping
- [x] Write JWT validation middleware that extracts and validates tokens against the OIDC provider's JWKS endpoint
- [x] Write middleware that attaches the caller identity to the request context
- [x] Write integration tests for middleware using real JWT fixtures
- [x] Set up structured logging (zerolog or slog) with consistent field conventions
- [x] Configure LocalStack-backed S3 client with path-style addressing and endpoint override from env
- [x] Write `S3Client` with methods: `PutObject`, `GetPresignedURL`, `ObjectExists`
- [ ] Write integration tests for `S3Client` against LocalStack

**Success Criteria:** The API server starts, connects to the database, and serves the health check endpoint. JWT middleware correctly rejects requests with missing or invalid tokens. The S3 client can put and presign objects against LocalStack. All migrations run cleanly forward and rollback.

### Phase 2: Core API Endpoints

The minimum endpoints the CLI needs to function.

- [x] Implement `GET /v1/packages` — list all published packages with name, version, tier, description
- [x] Implement `GET /v1/packages/search?q=` — search by name prefix and description
- [x] Implement `GET /v1/packages/{name}` — resolve latest published version, return full manifest including bottle presigned URL
- [x] Implement `GET /v1/packages/{name}/{version}` — resolve specific version
- [x] Presigned URL TTL configurable via environment variable, default 5 minutes
- [x] Implement `POST /v1/packages/{name}/versions` — pipeline registration endpoint, accepts bottle S3 key, SHA256, cosign sig ref, SBOM S3 key, scan result references, tier; creates version record
- [x] Registration endpoint requires a pipeline service token (separate from user JWTs), validated via a shared secret or dedicated OIDC client
- [x] Implement `GET /v1/signing-keys/current` — returns the active public key for signature verification
- [x] Write unit tests for all handlers with table-driven test cases
- [ ] Write integration tests for the full resolve flow: seed a version record, call resolve, verify presigned URL is valid and points to the correct S3 key

**Success Criteria:** All endpoints return correct responses for happy path and error cases (not found, unauthorized, bad request). The registration endpoint correctly creates a version record and the resolve endpoint returns it with a valid presigned URL. Integration tests pass against LocalStack.

### Phase 3: Signing Infrastructure

cosign integration for both the pipeline (signing) and the CLI (verification). Must be in place before any real packages are distributed.

- [ ] Generate cosign key pair for development: `cosign generate-key-pair`
- [ ] Document the production key generation process: generate on a secured machine, store private key in secrets manager, never commit
- [ ] Store development public key in `infra/keys/cosign-dev.pub` — safe to commit
- [ ] Write `pipeline/signing` package: `SignBottle(bottlePath, keyPath string) (sigB64 string, error)` using cosign SDK
- [ ] Write `pipeline/signing` package: `SignSBOM` — same pattern
- [ ] The sig reference stored in the API is the base64-encoded detached signature, not a Rekor entry for internal key signing
- [ ] Write `cli/internal/verifier` package: `Verify(bottlePath string, sigB64 string, publicKey string) error`
- [ ] Verifier fetches public key from keychain cache or calls `GET /v1/signing-keys/current` and caches the result
- [ ] Verifier fails closed: any error in verification aborts the install
- [ ] Write tests for signing and verification round-trip with a real bottle tarball fixture
- [ ] Write test that confirms verification fails with a tampered bottle
- [ ] Write test that confirms verification fails with a signature from a different key

**Success Criteria:** A bottle can be signed using the development key pair and the signature can be verified by the verifier package. Tampered bottles and wrong-key signatures are rejected. The public key fetch and cache flow works correctly.

### Phase 4: CLI Core — Authentication and Install

The minimum CLI for a developer to authenticate and install a package.

- [ ] Set up Cobra CLI structure with root command and persistent flags: `--api-url`, `--log-level`
- [ ] Implement `pkgtool auth login` — OIDC device flow, opens browser, waits for callback, stores token in macOS keychain under a namespaced service name
- [ ] Implement `pkgtool auth status` — prints current identity and token expiry
- [ ] Implement `pkgtool auth logout` — removes token from keychain
- [ ] Write token refresh logic: check expiry before each API call, refresh if within 5 minutes of expiry, prompt re-login if refresh fails
- [ ] Implement content-addressable store at `~/.pkgtool/store/{sha256}/`
- [ ] Store has methods: `Has(sha256 string) bool`, `Put(sha256 string, bottlePath string) error`, `Get(sha256 string) (path string, error)`
- [ ] APFS clonefile support with fallback to regular copy when not available
- [ ] Implement `pkgtool install <package>` command:
  - [ ] Authenticate and get token
  - [ ] Call resolve endpoint, receive manifest
  - [ ] Verify cosign signature
  - [ ] Check local store for SHA256 cache hit
  - [ ] On cache miss: download bottle from presigned URL, verify SHA256, store in content-addressable store
  - [ ] Extract bottle tarball to prefix `~/.pkgtool/prefix/Cellar/{name}/{version}/`
  - [ ] Symlink binaries into `~/.pkgtool/prefix/bin/`
  - [ ] Print clean status output with each step
- [ ] Implement `pkgtool uninstall <package>` — removes symlinks and Cellar directory, does not remove store entry
- [ ] Implement `pkgtool list` — shows installed packages and versions
- [ ] Implement `pkgtool search <query>` — calls search endpoint, prints results
- [ ] Print user-friendly error messages for: auth failure, package not found, signature verification failure, download failure, checksum mismatch
- [ ] Write integration tests for install flow against a locally seeded API and LocalStack

**Success Criteria:** A developer can run `pkgtool auth login`, complete the OIDC flow, run `pkgtool install <package>` for a seeded package, and find the binary available on their PATH. Running install again uses the cache and skips the download. Signature verification failure prevents installation.

### Phase 5: Internal Tier Pipeline

The pipeline for ingesting packages from internal teams via GoReleaser. This is the first real end-to-end flow.

- [ ] Write `pipeline/ingest` package that accepts a bottle tarball path and registers it with the API
- [ ] Write a GitHub Actions workflow triggered by new formula PRs to the internal tap repo
- [ ] Workflow steps:
  - [ ] Detect new or updated formula in PR
  - [ ] Download the bottle archive referenced in the formula
  - [ ] Verify the bottle URL is from an allowed internal origin (configurable allowlist)
  - [ ] Generate SBOM from bottle: `syft packages <bottle> -o cyclonedx-json=sbom.cdx.json`
  - [ ] Run vulnerability scan against SBOM: `grype sbom:sbom.cdx.json -o json --file grype-result.json`
  - [ ] Evaluate grype result against policy thresholds: fail pipeline if critical CVEs found
  - [ ] Run ScanCode license check against the tarball contents
  - [ ] Evaluate license results against `pipeline/policy/license-policy.yaml`
  - [ ] Upload bottle to S3: `bottles/internal/{name}/{version}/...`
  - [ ] Upload SBOM to S3: `sboms/{name}/{version}/sbom.cdx.json`
  - [ ] Upload raw grype result to S3: `scans/{name}/{version}/grype.json`
  - [ ] Upload raw ScanCode result to S3: `scans/{name}/{version}/scancode.json`
  - [ ] Sign bottle with cosign using private key from secrets manager
  - [ ] Call API registration endpoint with all metadata
  - [ ] Post pipeline result as PR comment (pass/fail with scan summary)
- [ ] Write `pipeline/policy/license-policy.yaml` with initial allow/warn/deny lists
- [ ] Write license policy evaluator that reads the YAML and evaluates ScanCode output against it
- [ ] Write pipeline tests with fixture bottles and mocked scanner output

**Success Criteria:** A GoReleaser-produced bottle submitted via a PR to the tap repo goes through the pipeline automatically. On pass, it is signed, uploaded to S3, and registered with the API. On scan failure, the PR is blocked with a clear summary. A developer can install the package using the CLI immediately after the pipeline succeeds.

### Phase 6: External-Binary Tier Pipeline and Approval Workflow

Adds support for the third tier and the manual approval gate.

- [ ] Define `formulas/external-binary/requests/` directory structure with a YAML schema for package requests
- [ ] Request schema fields: name, version, source_url, sha256, requested_by, reason, license_expected
- [ ] Write GitHub Actions workflow triggered by new files in `external-binary/requests/`
- [ ] Workflow steps (automatic, no approval yet):
  - [ ] Download binary from source_url
  - [ ] Verify SHA256 against request file value
  - [ ] Run syft + grype scan (same as internal tier)
  - [ ] Run ScanCode against tarball contents (best-effort, not a gate)
  - [ ] Upload scan artifacts to S3 under `scans/external-binary/{name}/{version}/`
  - [ ] Call API to create a pending version record with `approval_status: pending`
  - [ ] Post scan summary as PR comment including CVE counts, detected licenses, and policy evaluation
- [ ] Implement `POST /v1/packages/{name}/versions/{version}/approve` API endpoint
  - [ ] Requires elevated role claim in JWT or a separate admin token
  - [ ] Records approver identity, timestamp, and optional reason in `approval_records`
  - [ ] Updates version `approval_status` to `approved`
  - [ ] Triggers the sign-upload-register sequence (or a webhook that triggers the pipeline to continue)
- [ ] Implement `POST /v1/packages/{name}/versions/{version}/deny` API endpoint
  - [ ] Records denier identity and reason
  - [ ] Updates version `approval_status` to `denied`
- [ ] Write the sign-upload-register sequence that runs after approval:
  - [ ] Retrieve bottle from pending S3 location
  - [ ] Sign with cosign
  - [ ] Move to published S3 location
  - [ ] Update version record to `published`
- [ ] Write API endpoint `GET /v1/admin/pending` — lists all versions awaiting approval with scan summaries
- [ ] Notify reviewers on new pending version (Slack webhook or email, configurable)
- [ ] Write integration tests for the full approval flow: submit request → pending → approve → published → installable

**Success Criteria:** Submitting a package request YAML to the repo triggers the pipeline, which scans the binary and creates a pending record. The pending package is not installable. An admin calls the approve endpoint, the binary is signed and published, and the package becomes installable. Deny correctly blocks publication.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `docker-compose.yml` | Create | LocalStack service with S3 initialization |
| `pkg/manifest/manifest.go` | Create | Package manifest schema — the API↔CLI contract |
| `pkg/types/types.go` | Create | Shared API request/response types |
| `.env.local.example` | Create | All environment variables documented |
| `docs/env-conventions.md` | Create | Environment variable naming conventions |
| `api/` | Create | Backend HTTP service (handlers, DB, S3, middleware) |
| `cli/` | Create | Developer CLI (`pkgtool`) |
| `cli/internal/verifier/` | Create | cosign signature verification package |
| `cli/internal/store/` | Create | Content-addressable bottle store |
| `pipeline/ingest/` | Create | Package ingestion and API registration |
| `pipeline/signing/` | Create | cosign signing package |
| `pipeline/policy/license-policy.yaml` | Create | License allow/warn/deny policy |
| `infra/keys/cosign-dev.pub` | Create | Development cosign public key (safe to commit) |
| `formulas/external-binary/requests/` | Create | External-binary package request YAML schema |

## Testing Plan

- [ ] Unit tests for all API handlers with table-driven cases covering happy path and error conditions
- [ ] Integration tests for JWT middleware using real JWT fixtures
- [ ] Integration tests for `S3Client` against LocalStack (`PutObject`, `GetPresignedURL`, `ObjectExists`)
- [ ] Integration tests for all database migrations (forward and rollback)
- [ ] Integration tests for the full resolve flow: seed version record → resolve → verify presigned URL
- [ ] Round-trip tests for cosign signing and verification with a real bottle tarball fixture
- [ ] Negative tests: tampered bottle rejected, wrong-key signature rejected
- [ ] Integration tests for the full CLI install flow against a locally seeded API and LocalStack
- [ ] Pipeline tests with fixture bottles and mocked scanner output
- [ ] Integration tests for the full approval flow: submit → pending → approve → published → installable

## Rollback Plan

Each phase produces a deployable state. Rollback by reverting to the previous phase's artifact:

- Database migrations are written with `down` migrations; run rollback to revert schema changes
- API is stateless beyond the database; redeploying a previous image is sufficient
- Pipeline changes are in GitHub Actions workflows and can be reverted via git revert
- CLI is distributed as a versioned binary; users can pin to the previous version

## Dependencies

- `cosign` CLI and SDK — artifact signing and verification
- `syft` — SBOM generation from tarballs and container images
- `grype` — vulnerability scanning against SBOMs and OCI artifacts
- ScanCode Toolkit — license compliance scanning
- LocalStack — local S3-compatible endpoint for development and integration tests
- Organization OIDC provider — JWKS endpoint for JWT validation
- Secrets manager — storage for cosign private signing key
- macOS arm64 runner — required for external-built tier (Phase 7+, out of MVP scope)
- GoReleaser — used by internal teams to produce bottle archives (not a direct dependency)

## References

- [Initial Design](../design/0001-initial-design.md)
- [Technical Plan](../design/0002-technical-plan.md)
- [Wiz Scanner Integration](0002-wiz-scanner-integration.md)
