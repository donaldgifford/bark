---
id: IMPL-0003
title: "Phase 3: Signing Infrastructure Implementation"
status: Completed
author: Claude Code
created: 2026-03-04
implements: IMPL-0001
---

# IMPL-0003: Phase 3 Signing Infrastructure Implementation

**Status:** Completed
**Author:** Claude Code
**Date:** 2026-03-04
**Implements:** IMPL-0001 Phase 3

## Objective

Implement cosign-based signing infrastructure for both the pipeline (signing) and CLI (verification) components. This establishes the cryptographic foundation for package integrity verification before any real packages are distributed.

## Implementation Summary

### Package Structure

```
pipeline/signing/           # Pipeline signing functionality
├── signer.go              # Core signing implementation
├── convenience.go         # Convenience functions matching requirements
└── signer_test.go         # Comprehensive tests

cli/internal/verifier/      # CLI verification functionality
├── verifier.go            # Core verification implementation
└── verifier_test.go       # Comprehensive tests with API mocking

infra/keys/                # Development signing keys
├── cosign-dev.key         # Private key (encrypted, committed for dev)
└── cosign-dev.pub         # Public key (safe to commit)

testdata/                  # Test fixtures
├── test-bottle.tar.gz     # Sample bottle for round-trip tests
└── test-package.txt       # Sample file content
```

### Key Architectural Decisions

#### 1. **cosign SDK Approach**
**Decision:** Use `github.com/sigstore/cosign/v2/pkg/signature` SDK directly

**Rationale:**
- Better control over signing/verification process
- No external binary dependency
- More robust error handling
- Better testability with mocked interfaces
- Direct integration with Go's context system

#### 2. **Key Format**
**Decision:** Encrypted Sigstore format (cosign's default)

**Details:**
- Private keys generated with `cosign generate-key-pair`
- Uses empty password for dev environment (passwords provided via function parameter)
- PEM-encoded ECDSA keys with P-256 curve
- Public keys in standard PKIX format

#### 3. **Verifier Design**
**Decision:** File-based cache with configurable TTL

**Implementation:**
- Cache directory: `~/.pkgtool/cache/signing-key.json` (default)
- Default TTL: 24 hours
- Falls back to API when cache is stale or missing
- Fails closed on any error (security-first)

#### 4. **Signature Format**
**Decision:** Base64-encoded detached signatures

**Details:**
- Signs the entire file content (cosign handles SHA256 hashing internally)
- Returns base64-encoded ASN.1 signature bytes
- Compatible with standard ECDSA verification

## Implementation Details

### Pipeline Signing (`pipeline/signing`)

#### Core Functions
- `NewSigner(keyPath string) *Signer` - Creates signer instance
- `SignBottle(ctx, bottlePath string) (string, error)` - Signs bottle tarball
- `SignSBOM(ctx, sbomPath string) (string, error)` - Signs SBOM file

#### Convenience Functions (Requirements Compliance)
- `SignBottle(bottlePath, keyPath string) (string, error)` - Matches spec exactly
- `SignSBOM(sbomPath, keyPath string) (string, error)` - Matches spec exactly

#### Key Features
- Uses `signature.SignerFromKeyRef()` for cosign compatibility
- Handles encrypted Sigstore private key format
- Context-aware signing operations
- Comprehensive error handling

### CLI Verification (`cli/internal/verifier`)

#### Core Functions
- `NewVerifier(cfg Config) *Verifier` - Creates verifier with config
- `Verify(ctx, filePath, sigB64, publicKey string) error` - Verifies signature

#### Configuration Options
- `APIURL` - API endpoint for public key fetching
- `CacheDir` - Local cache directory (defaults to `~/.pkgtool/cache`)
- `CacheTTL` - Cache lifetime (defaults to 24h)
- `Client` - HTTP client for API calls

#### Key Features
- Public key caching with TTL
- Automatic API fallback when publicKey parameter is empty
- `LoadPublicKeyRaw()` for PEM parsing
- Context-aware verification
- Fail-closed security model

### Testing Strategy

#### Unit Tests
- **Signing Package:** 4 test cases covering happy path, error conditions, round-trip consistency
- **Verifier Package:** 8 test cases covering verification, API integration, caching, error handling

#### Integration Tests
- **Round-trip testing:** Sign with pipeline package, verify with CLI package
- **SBOM testing:** Separate tests for SBOM file signing/verification
- **Tampered file rejection:** Ensures verification fails for modified files
- **Wrong key rejection:** Ensures signatures from different keys fail

#### Test Fixtures
- `test-bottle.tar.gz` - Realistic tarball for bottle testing
- `test-package.txt` - Simple text file for basic functionality
- Cosign development key pair for all tests

## Security Considerations

### Development vs Production
- **Development:** Uses empty password for convenience, key committed to repo
- **Production:** Will use proper password management and external key storage
- **Key Rotation:** Public key fetching supports key rotation via API

### Verification Security
- **Fail Closed:** Any error in verification aborts install
- **Content Integrity:** Signs entire file content, not just metadata
- **Cache Security:** Cached keys have TTL to limit stale key exposure

## Success Criteria (Met)

✅ **Bottle Signing:** Pipeline can sign bottle tarballs using development key pair
✅ **SBOM Signing:** Pipeline can sign SBOM files with same pattern
✅ **Signature Verification:** CLI verifier correctly validates signatures
✅ **Public Key Fetching:** Verifier fetches and caches public keys from API
✅ **Round-trip Testing:** Real bottle can be signed and verified successfully
✅ **Tampered File Rejection:** Verification fails for modified bottles
✅ **Wrong Key Rejection:** Verification fails for signatures from different keys
✅ **Requirements Compliance:** Exact function signatures match implementation plan

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `pipeline/signing/signer.go` | Create | Core signing implementation using cosign v2 SDK |
| `pipeline/signing/convenience.go` | Create | Convenience functions matching requirements spec |
| `pipeline/signing/signer_test.go` | Create | Comprehensive unit tests with error cases |
| `cli/internal/verifier/verifier.go` | Create | Core verification with API integration and caching |
| `cli/internal/verifier/verifier_test.go` | Create | Unit tests with mock API server |
| `cli/integration_test.go` | Create | End-to-end signing and verification tests |
| `infra/keys/cosign-dev.key` | Create | Development private key (encrypted) |
| `infra/keys/cosign-dev.pub` | Create | Development public key (safe to commit) |
| `testdata/test-bottle.tar.gz` | Create | Test bottle fixture |
| `testdata/test-package.txt` | Create | Test file content |

## Dependencies Added

- `github.com/sigstore/cosign/v2@latest` - Core cosign functionality

## Next Steps (Phase 4)

The signing infrastructure is now ready for integration with:

1. **CLI Authentication** - OIDC device flow and token management
2. **Install Command** - Package resolution, signature verification, and installation
3. **Content-Addressable Store** - Efficient bottle caching and deduplication

The verification step will be integrated into the install flow:
```go
// In install command
err := verifier.Verify(ctx, bottlePath, manifest.CosignSigRef, "")
if err != nil {
    return fmt.Errorf("signature verification failed: %w", err)
}
```

## Notes

- ECDSA signatures are non-deterministic due to randomness, which is expected and secure
- The implementation uses cosign's internal libraries for maximum compatibility
- Public key format is standard PEM PKIX, ensuring broad tool compatibility
- All error messages are actionable and include sufficient context for debugging