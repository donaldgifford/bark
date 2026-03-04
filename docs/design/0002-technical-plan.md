---
id: DESIGN-0002
title: "Technical Plan"
status: Draft
author: Donald Gifford
created: 2026-02-27
---

# DESIGN 0002: Technical Plan

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-02-27

## Overview

This document describes the order and approach for building the internal package distribution platform at the component and integration level, not the code level. It gives the team a shared mental model of what gets built in what order and why, covering five major areas: infrastructure, backend API, CI pipeline, developer CLI, and the admin approval workflow.

## Goals and Non-Goals

### Goals

- Define a clear build order with explicit dependencies between areas
- Establish guiding principles that prevent common architectural mistakes
- Identify parallel workstreams to enable concurrent development once integration contracts are stable
- Ensure real infrastructure (LocalStack) is in place from the first commit, not introduced late

### Non-Goals

- Code-level implementation detail (covered in the implementation plan)
- Deployment or operational runbooks
- Feature prioritization beyond MVP scope

## Background

The system has five major areas of work that are not strictly sequential — some can be parallelized — but there are hard dependencies that dictate a general order. The manifest schema is the central contract: everything (the API, the CLI, the pipeline) converges on it. It must be defined before building any of the consumers.

## Detailed Design

### Phase Order and Dependencies

```
Infrastructure  ──────────────────────────────────────────────────►
                      │
                      ▼
              API (core) ──────────────────────────────────────────►
                      │
              ┌───────┴────────┐
              ▼                ▼
         CLI (core)      Pipeline (internal tier)
              │                │
              └───────┬────────┘
                      ▼
              Integration (CLI + API + S3 + signing)
                      │
              ┌───────┴────────┐
              ▼                ▼
        Pipeline          Approval
      (ext-built,        Workflow
       ext-binary)            │
              └───────┬────────┘
                      ▼
              Wiz + ScanCode Integration
                      │
                      ▼
              Hardening + Operationalization
```

### Area 1: Infrastructure

The foundation everything else runs on. Should be the first thing stood up and the first thing a new team member can run locally.

**Local Development Environment** — Docker Compose brings up LocalStack with S3. An initialization script creates the bottle bucket with versioning enabled. Environment variable conventions are established here and carried through the entire project — the same vars drive local and production behavior, with production simply omitting the LocalStack endpoint override. A Makefile provides standard targets: `infra-up`, `infra-down`, `infra-seed`, `api-dev`, `cli-build`, `test`, `lint`.

**Production Infrastructure** — S3 bucket with versioning enabled, encryption at rest, no public access, and a lifecycle policy for scan artifacts. IAM roles for the API service and the CI pipeline — neither uses long-lived access keys. The API reads from S3 (presigned URL generation) and the pipeline writes to S3 (bottle and SBOM uploads). A relational database for package state covers packages, versions, scan results, approval records, and signing key metadata.

**Signing Key Management** — cosign key pairs generated and stored in a secrets manager. The private key is accessible only to the CI pipeline. The public key is embedded in the CLI at build time or fetched from a well-known API endpoint on first run. A key rotation procedure is documented and scheduled.

### Area 2: Backend API

The authority for all package state. Built before the CLI so the CLI has something real to talk to.

**Authentication Middleware** — JWT validation against the organization identity provider. All routes except a health check require a valid token. The middleware extracts the caller identity and makes it available to handlers for audit logging.

**Package Manifest Endpoint** — The core endpoint. Returns a manifest: bottle S3 key, SHA256 checksum, cosign signature reference, tier, scan summary, and policy status. The response schema is the contract between the API and the CLI — define it explicitly and version it.

**Presigned URL Generation** — Bottle downloads never go through the API directly. The API generates a short-lived presigned S3 URL returned in the manifest. The presigned URL includes the S3 server-side checksum so the download is verified at the S3 layer before the CLI does its own verification.

**Package Registration Endpoint** — Used by the CI pipeline to register a new package version after it has been built, scanned, and signed. Accepts bottle S3 key, SHA256, cosign signature reference, SBOM S3 key, scan result references, and tier. Writes a new version record marked as pending or available depending on tier and scan outcome. Requires a pipeline service token separate from user JWTs.

**Search and List Endpoints** — Searchable package catalog powering `pkgtool search` and `pkgtool list`. No auth-sensitive data in these responses.

### Area 3: CI Pipeline

The pipeline has different shapes for each tier. Build them in order of complexity: internal first, then external-built, then external-binary.

**Internal Tier** — A GoReleaser-based team cuts a release. Their GoReleaser config references the internal tap repo. A webhook or polling job detects the new formula and bottle archive. The pipeline downloads the bottle, runs a lightweight Wiz scan, runs a license check, and if both pass, signs the bottle with cosign and calls the API registration endpoint. This establishes the core pipeline mechanics — download, scan, sign, upload, register — without the complexity of building from source.

**External-Built Tier** — Adds the build step before scanning. A formula file defines the source URL, checksum, and build instructions. A macOS arm64 runner fetches the source, verifies the checksum, builds the bottle, then runs the same scan-sign-upload-register sequence as the internal tier. The build environment is locked down: no network access after source fetch, pinned tool versions, clean VM per build via Tart or equivalent.

**External-Binary Tier** — Adds the manual approval gate. The pipeline runs automatically on submission — download, scan, generate SBOM, post results — but does not sign or publish. Instead it creates a pending approval record in the API and notifies reviewers. The sign-upload-register sequence only executes after an approval action is recorded.

**Scan and SBOM Steps** — Wiz CLI invocation is a shared step across all tiers. The scan result ID from Wiz is stored in the API alongside the package version. The SBOM is exported from Wiz in CycloneDX format and uploaded to S3. ScanCode runs against the source tree for external-built packages and against the tarball contents for the other tiers. License results are evaluated against the policy file.

### Area 4: Developer CLI

Built in parallel with the API once the manifest schema is stable. The CLI is the only thing developers interact with directly so its UX matters more than its internal complexity.

**Authentication** — OIDC device flow on first run. Token stored in the macOS keychain using the system keychain API. Transparent refresh — the developer authenticates once and the token is maintained without prompting unless it expires beyond the refresh window or is explicitly revoked.

**Install Command** — Calls the resolve endpoint, verifies the cosign signature against the embedded or fetched public key, checks the local content-addressable store for a cache hit, downloads the bottle via the presigned URL if not cached, verifies the SHA256, extracts the tarball, and symlinks binaries into the prefix. Errors at any verification step abort the install with a clear message.

**Content-Addressable Store** — Bottles stored locally by SHA256 hash. Reinstalls and cache hits skip the download entirely. APFS clonefile used where available for zero-copy materialization from the store. Default location: `~/.pkgtool/store`.

**Sync Command (Pkgfile)** — Teams check a `Pkgfile` into their repo specifying required packages and versions. `pkgtool sync` installs everything in the file, skipping packages already at the correct version. New team members run this once after authentication to get a consistent environment.

**Additional Commands** — Search, list installed, uninstall, update, gc, verify, shell completions. Thin wrappers over the API and the local store — no business logic in the CLI beyond what is needed to drive the install flow.

### Area 5: Approval Workflow

The mechanism for external-binary packages to move from scanned to published.

The starting implementation is PR-based: a YAML request file is submitted to the formulas repository, the pipeline runs automatically and posts scan results as a PR comment, and a reviewer merges to approve. The merge event triggers the sign-publish sequence. The PR-based approach is lower investment and provides a natural audit trail through git history.

A more integrated implementation adds API endpoints for approval actions and a minimal admin interface or Slack bot. This can be layered on later if the PR flow becomes a bottleneck.

### Area 6: Wiz and ScanCode Integration

Wiz CLI is invoked as a subprocess in the pipeline. Results are captured as structured JSON, the result ID extracted and stored in the API, and the pass/fail decision gates the pipeline. The Wiz policy governing what constitutes a pass is owned by the security team and managed in the Wiz console — pipeline code only needs to know the policy name to evaluate against.

A custom Wiz framework groups infra-adjacent controls into a named compliance view: S3 encryption posture, bucket access policy, signing key rotation status. This is a secondary deliverable for the security team and does not affect pipeline operation.

ScanCode is installed in the pipeline runner environment. A policy evaluator in the pipeline reads structured output against a versioned policy YAML file checked alongside the pipeline configuration.

### Area 7: Hardening and Operationalization

The final area before the system is considered production-ready:

- **Key rotation runbook** — documented procedure for rotating cosign signing keys, including how the new public key is distributed to CLI installations
- **Pipeline failure handling** — dead letter queue or alerting for jobs that fail silently; a stuck external-built job should not silently block a package from ever being published
- **Dependency updates** — process for re-scanning and re-publishing packages when upstream source has a new release
- **CLI distribution** — distributed as an internal package through the same system (bootstrapping requires a one-time manual install) or via a separate bootstrap script
- **Signing key availability** — private signing key must be accessible to CI but never to developers or the API; key access audited and limited to the pipeline execution role only
- **Documentation** — developer onboarding guide, package maintainer guide (internal and external), and operator guide (approvals, key rotation, scan failures)

## API / Interface Changes

Integration contracts to define before parallel workstreams begin. These are owned by the API team and consumed by both the CLI and pipeline teams:

- **Package manifest schema** — the API↔CLI contract, defined in `pkg/manifest`
- **API request/response types** — defined in `pkg/types`
- **S3 key structure** — `bottles/{tier}/{name}/{version}/...`, `sboms/...`, `scans/...`
- **Environment variable conventions** — shared naming across API, pipeline, and local development, documented in `docs/env-conventions.md`

## Data Model

```
packages          (id, name, description, tier, created_at)
package_versions  (id, package_id, version, bottle_s3_key, sbom_s3_key, sha256,
                   cosign_sig_ref, tier, scan_status, approval_status,
                   approved_by, approved_at, published_at, created_at)
scan_results      (id, package_version_id, scanner, result_s3_key, wiz_result_id,
                   passed, summary_json, scanned_at)
approval_records  (id, package_version_id, action, actor, reason, created_at)
signing_keys      (id, key_id, public_key, active, created_at, rotated_at)
```

## Testing Strategy

Use real infrastructure from day one. LocalStack from the first commit means the S3 integration is real code, not mocked, preventing an entire class of production surprises.

- Integration tests run against LocalStack for all S3 operations
- JWT middleware tested with real JWT fixtures
- Pipeline tests use fixture bottles and mocked Wiz CLI output
- CLI install flow tested against a locally seeded API and LocalStack

## Migration / Rollout Plan

Once the API manifest schema and S3 structure are defined, these workstreams can proceed in parallel:

- CLI core (auth + install) alongside the internal tier pipeline
- External-built pipeline alongside CLI search and sync commands
- Approval workflow alongside Wiz/ScanCode integration
- Wiz framework definition alongside operationalization work

The integration points — manifest schema, S3 key structure, API request/response shapes — should be defined as contracts up front so parallel workstreams are not blocked on each other.

## Open Questions

<!-- Unresolved decisions or areas needing further investigation -->

## References

- [Initial Design](0001-initial-design.md)
- [MVP Implementation Plan](../impl/0001-mvp-implementation-plan.md)
