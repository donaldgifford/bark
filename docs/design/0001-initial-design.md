---
id: DESIGN-0001
title: "Initial Design"
status: Draft
author: Donald Gifford
created: 2026-02-27
---

# DESIGN 0001: Initial Design

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-02-27

## Overview

A self-hosted package distribution platform for internal macOS developer tooling consisting of three parts: a developer CLI that replaces Homebrew for internal packages, a backend API that enforces authentication and serves signed package manifests, and a CI pipeline that builds, scans, signs, and publishes packages to S3.

## Goals and Non-Goals

### Goals

- Provide a developer experience as simple as Homebrew — a single CLI command to install a package
- Enforce authentication on all package operations using existing SSO infrastructure
- Cryptographically sign all distributed binaries and verify signatures at install time
- Scan all packages for vulnerabilities and license compliance before they are made available
- Maintain a clear audit trail of what was approved, by whom, and why
- Self-host all infrastructure with no external SaaS dependencies in the critical path
- Support ongoing vulnerability monitoring against the published package inventory

### Non-Goals

- Fleet management or remote device control
- Package installation telemetry or usage tracking
- Replacing Homebrew for public package distribution
- Supporting Linux or Windows developer machines in this iteration

## Background

Engineering organizations running macOS developer fleets have no good middle ground between raw Homebrew and a fully managed SaaS product like Workbrew. Raw Homebrew gives developers flexibility but provides no controls: there is no authentication, no signature verification, no vulnerability scanning, no license compliance, and no audit trail. SaaS products solve these problems but introduce external data dependencies, pricing at scale, and loss of control over the pipeline and infrastructure.

Specific problems being solved:

- Developers can install arbitrary binaries with no organizational oversight
- Internal tooling has no secure, authenticated distribution mechanism
- There is no visibility into what is installed across the developer fleet
- Third-party packages are pulled from the public Homebrew CDN with no verification beyond Homebrew's own checksums
- License compliance for distributed tooling is untracked
- Vulnerability exposure in distributed packages is unknown until something goes wrong

## Detailed Design

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Developer Machine                        │
│                                                                 │
│   pkgtool install my-tool                                       │
│         │                                                       │
│         ▼                                                       │
│   ┌───────────┐    auth token    ┌─────────────────────────┐   │
│   │    CLI    │ ───────────────► │     Internal API        │   │
│   │           │ ◄─────────────── │                         │   │
│   └───────────┘    manifest +    └────────────┬────────────┘   │
│         │          presigned URL              │                 │
│         │                                     │ reads           │
│         ▼                                     ▼                 │
│   verify signature                    ┌──────────────┐         │
│   download bottle  ◄──────────────── │      S3      │         │
│   install + link                      └──────────────┘         │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                        CI Pipeline                              │
│                                                                 │
│   source / binary artifact                                      │
│         │                                                       │
│         ▼                                                       │
│   ┌────────────┐   ┌─────────────┐   ┌──────────────────────┐ │
│   │   Build    │──►│ Wiz Scan +  │──►│  License Check       │ │
│   │ (if needed)│   │ SBOM Export │   │  (ScanCode)          │ │
│   └────────────┘   └─────────────┘   └──────────┬───────────┘ │
│                                                  │             │
│                                                  ▼             │
│                                      ┌──────────────────────┐ │
│                                      │ Policy Gate          │ │
│                                      │ auto / manual        │ │
│                                      └──────────┬───────────┘ │
│                                                  │             │
│                                                  ▼             │
│                                      ┌──────────────────────┐ │
│                                      │ cosign sign          │ │
│                                      │ publish to S3        │ │
│                                      │ register with API    │ │
│                                      └──────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### Package Tiers

All packages are not equal. The system defines three tiers with different trust assumptions, build processes, and approval mechanisms.

```
Tier              Build Control    Scan Depth    Approval
─────────────────────────────────────────────────────────
internal          team CI          lightweight   automatic
external-built    our CI           full          automatic (policy)
external-binary   none             best-effort   manual
```

**Internal** — Packages produced by internal teams via GoReleaser. The pipeline performs a lightweight scan and license check before re-signing with the organization distribution key. Approval is automatic if the scan passes.

**External-Built** — Third-party packages built from source in a controlled environment. Source URL and checksum defined in a formula. Full SBOM generated, full scan run, license check evaluated against policy. Approval is automatic if the scan passes all policy thresholds.

**External-Binary** — Third-party packages where building from source is impractical. Binary downloaded, scanned best-effort, SBOM generated. Approval is always manual — a designated reviewer sees the scan results and makes an explicit decision before signing and publishing.

### Components

**CLI (`pkgtool`)** — Developer-facing binary written in Go. Handles authentication via OIDC device flow, stores tokens in the macOS keychain, resolves packages via the API, verifies cosign signatures, and manages a local content-addressable store to avoid redundant downloads. Business logic lives in the API, not here.

**API** — Go HTTP service. Authoritative source for package state. Enforces authentication on all requests. Serves package manifests containing the bottle URL, SHA256 checksum, cosign signature reference, scan summary, and tier metadata. Issues presigned S3 URLs for bottle downloads so binaries are never accessible without a valid token. Stores package state in a relational database.

**S3** — Object storage for bottle tarballs, SBOM documents, and raw scan output. LocalStack used for local development with the same AWS SDK code running against both.

**CI Pipeline** — macOS arm64 runners responsible for the entire lifecycle: building bottles for external-built packages, running Wiz CLI scans, running ScanCode, generating SBOMs, signing artifacts with cosign, uploading to S3, and registering with the API.

**Wiz CLI Integration** — Vulnerability scan executor in the pipeline. Scan results flow into the Wiz platform for centralized CVE visibility across all published packages. Post-publish, Wiz continuously monitors published packages against updated CVE databases and alerts on newly discovered vulnerabilities.

**ScanCode Toolkit** — License compliance scanning. Runs separately against source trees and generated SBOMs. Results evaluated against a policy file defining allowed, warned, and denied license identifiers. This gate is independent from the vulnerability gate and can block publication on its own.

**cosign** — Artifact signing and verification. Pipeline signs bottle tarballs and SBOMs. CLI verifies signatures at install time before extraction. Signing keys managed separately and rotated on a defined schedule.

### Authentication Flow

```
First run:
  pkgtool auth login
       │
       ▼
  OIDC device flow (browser opens → SSO login → token issued)
       │
       ▼
  Token stored in macOS keychain
       │
       ▼
  All subsequent API calls use token from keychain
  Token refresh is transparent
```

The API validates JWTs against the organization's identity provider on every request. No custom credential system is introduced.

### Vulnerability Monitoring

Wiz provides continuous monitoring against the published package inventory after publication. When a new CVE affects a package already in the registry, Wiz alerts the security team through the existing Wiz console — eliminating the need to build a re-scanning cron job.

The Wiz framework covers infra-adjacent controls (S3 encryption, bucket policy, signing key rotation schedules) as a compliance view for the security team. The source of truth for package state remains the API database.

### License Policy

License compliance is evaluated at pipeline time against a versioned policy file checked into the pipeline configuration. The policy defines three lists: allowed licenses that pass automatically, licenses that trigger a warning but do not block, and licenses that block publication entirely. Evaluation covers both declared licenses in package metadata and licenses detected in source files by ScanCode, catching transitive dependencies that only declare their license in source headers.

### Audit Trail

Every package publication event is recorded with: package name and version, tier, scan result references (Wiz result ID, ScanCode output URL, SBOM S3 path), approval mechanism (automatic or manual), approver identity if manual, signing key used, and timestamp. This provides a complete chain of evidence for compliance without depending on any external system.

## API / Interface Changes

### CLI Commands

```
pkgtool auth login                    # OIDC device flow
pkgtool auth status                   # Print identity and token expiry
pkgtool auth logout                   # Remove token from keychain
pkgtool install <package>             # Install a package
pkgtool install <package>@<version>   # Install a specific version
pkgtool uninstall <package>           # Remove a package
pkgtool list                          # Show installed packages
pkgtool search <query>                # Search available packages
pkgtool sync [--file Pkgfile]         # Install all packages in Pkgfile
pkgtool update                        # Check for newer versions
pkgtool gc                            # Remove unused store entries
pkgtool verify                        # Re-verify signatures on installed packages
pkgtool completion bash|zsh|fish      # Shell completion
```

### API Endpoints

```
GET  /healthz
GET  /v1/packages
GET  /v1/packages/search?q=
GET  /v1/packages/{name}
GET  /v1/packages/{name}/{version}
POST /v1/packages/{name}/versions
POST /v1/packages/{name}/versions/{version}/approve
POST /v1/packages/{name}/versions/{version}/deny
GET  /v1/admin/pending
GET  /v1/signing-keys/current
```

### Package Manifest Schema

The manifest is the contract between the API and the CLI. Defined in `pkg/manifest`:

```
name, version, tier
bottle_s3_key, bottle_sha256, bottle_presigned_url (short-lived, default 5m TTL)
cosign_sig_ref
sbom_s3_key
scan_summary { wiz_result_id, license_policy_status, passed }
published_at
```

### S3 Key Structure

```
bottles/{tier}/{name}/{version}/{name}-{version}.arm64_sonoma.bottle.tar.gz
sboms/{name}/{version}/sbom.cdx.json
scans/{name}/{version}/wiz.json
scans/{name}/{version}/scancode.json
```

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

LocalStack provides a local S3-compatible endpoint from day one — no mocking or interface shims required. The same AWS SDK code runs against both LocalStack and real S3 via environment variable configuration.

Integration tests cover:
- JWT middleware using real JWT fixtures
- S3 client operations against LocalStack
- Full install flow against a locally seeded API and LocalStack
- Signing and verification round-trips with real bottle tarball fixtures
- Tamper detection: verification must fail with a modified bottle
- Wrong-key rejection: verification must fail with a signature from a different key pair

## Migration / Rollout Plan

This is a greenfield system. Rollout sequence:

1. Stand up infrastructure (LocalStack locally, real S3 and database in production)
2. Deploy API and run database migrations
3. Run internal tier pipeline on a pilot internal package
4. Install CLI on pilot developer machines
5. Expand to all internal packages via GoReleaser tap integration
6. Enable external-binary tier for requested third-party tools with manual approval
7. Enable external-built tier for packages requiring source builds

## Open Questions

<!-- Unresolved decisions or areas needing further investigation -->

## References

- [Technical Plan](0002-technical-plan.md)
- [MVP Implementation Plan](../impl/0001-mvp-implementation-plan.md)
