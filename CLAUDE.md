# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`bark` is a self-hosted internal package manager for distributing and managing internal tooling across macOS developer fleets. It has three components: a developer CLI (`pkgtool`), a backend API, and a CI pipeline. It enforces authentication (OIDC), cryptographic signing (cosign), vulnerability scanning (Wiz CLI), and license compliance for all distributed packages.

Go module: `github.com/donaldgifford/bark`

## Commands

```bash
# Build
make build              # Build to build/bin/bark

# Test
make test               # Run all tests with race detector (-v -race)
make test-pkg PKG=./path/to/pkg  # Test a specific package
make test-coverage      # Generate coverage report to coverage.out
make test-report        # Run tests with coverage and open HTML report

# Lint & Format
make lint               # Run golangci-lint
make lint-fix           # Auto-fix linting issues
make fmt                # Run gofmt + goimports

# Pre-commit
make check              # lint + test
make ci                 # Full pipeline: lint, test, build, license-check

# License
make license-check      # Check deps against allowed SPDX licenses
make license-report     # Generate CSV of dependency licenses

# Release
make release TAG=v1.0.0 # Create and push git tag
make release-check      # Validate goreleaser config
make release-local      # Test goreleaser build without publishing

# Documentation
docz create adr "Title"     # New Architecture Decision Record in docs/adr/
docz create design "Title"  # New design doc in docs/design/
docz create impl "Title"    # New implementation plan in docs/impl/
docz create rfc "Title"     # New RFC in docs/rfc/
```

## Architecture

The project is in early development (Phase 0/1). The planned package structure:

```
cmd/bark/          - Entry point
cli/               - Developer-facing CLI tool
  internal/auth/   - OIDC device flow authentication
  internal/store/  - Content-addressable binary store (~/.pkgtool/store, SHA256)
  internal/verifier/ - cosign signature verification
api/               - Backend HTTP service (JWT middleware, presigned S3 URLs)
  handlers/        - HTTP endpoint handlers
  database/        - DB layer and migrations
  s3/              - S3 client wrapper
pipeline/          - CI automation
  signing/         - cosign signing integration
  ingest/          - Package ingestion and registration
  policy/          - License policy evaluation
pkg/manifest/      - Shared package manifest schema (central contract)
pkg/types/         - Shared API types
```

**Key architectural decisions:**
- Business logic lives in the API, not the CLI (keep CLI dumb)
- Binaries are never served through the API—use presigned S3 URLs
- Content-addressable store uses SHA256 deduplication
- Three package tiers with different trust levels: Internal (auto-approve), External-Built (auto-approve after full scan), External-Binary (manual approval required)
- LocalStack from day one for S3 testing—no mocking S3

## Code Style

This project follows the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) enforced via golangci-lint.

- Max function length: 100 lines; cyclomatic complexity: 15; nesting depth: 4
- All errors must be checked; wrap with `%w` for context
- Imports ordered: stdlib → external → `github.com/donaldgifford` (internal)
- Line length: 150 characters (golines)
- Use `gofumpt` (stricter than `gofmt`)
- Table-driven tests expected

## PR Requirements

Every PR **must** have exactly one semver label before it can merge: `major`, `minor`, `patch`, or `dont-release`. The release workflow uses this label to auto-bump the version and cut a GitHub release via GoReleaser.

## Known Issues

- `.goreleaser.yml` references binary `forge` and `./cmd/forge`—needs to be updated to `bark` and `./cmd/bark`.
- `go.mod` has not been initialized yet (project is pre-Phase 1).

## Implementation Roadmap

Detailed phase plans live in `docs/impl/`. The 9-phase MVP:

| Phase | Focus |
|-------|-------|
| 0 | Project setup and conventions |
| 1 | Database and API foundation |
| 2 | Core API endpoints |
| 3 | Signing infrastructure (cosign) |
| 4 | CLI core—auth and install |
| 5 | Internal tier pipeline |
| 6 | External-binary tier + approval workflow |
| 7 | External-built tier pipeline |
| 8 | Pkgfile sync and CLI polish |
| 9 | Wiz framework and operational readiness |
