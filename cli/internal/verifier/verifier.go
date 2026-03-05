// Package verifier provides cosign-based signature verification for the bark CLI.
package verifier

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/donaldgifford/bark/pipeline/signing"
)

// Verifier verifies cosign signatures against artifact files.
type Verifier struct {
	apiURL   string
	cacheDir string
	cacheTTL time.Duration
	client   *http.Client
}

// Config holds verifier configuration.
type Config struct {
	// APIURL is the base URL of the bark API (e.g. "https://bark.example.com").
	APIURL string
	// CacheDir overrides the default public-key cache directory (~/.pkgtool/cache).
	CacheDir string
	// CacheTTL overrides the default 24-hour key cache TTL.
	CacheTTL time.Duration
	// Client overrides the default HTTP client.
	Client *http.Client
}

// NewVerifier creates a new Verifier with the given configuration.
func NewVerifier(cfg Config) *Verifier {
	if cfg.CacheDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = os.TempDir()
		}
		cfg.CacheDir = filepath.Join(homeDir, ".pkgtool", "cache")
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 24 * time.Hour
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 30 * time.Second}
	}

	return &Verifier{
		apiURL:   cfg.APIURL,
		cacheDir: cfg.CacheDir,
		cacheTTL: cfg.CacheTTL,
		client:   cfg.Client,
	}
}

// Verify verifies filePath against the base64-encoded sigB64 signature.
// If publicKey is empty, the current signing key is fetched from the API and cached.
// Fails closed: any error (key fetch, decode, verification) aborts the install.
func (v *Verifier) Verify(ctx context.Context, filePath, sigB64, publicKey string) error {
	pubKeyPEM := publicKey
	if pubKeyPEM == "" {
		var err error
		pubKeyPEM, err = v.getPublicKey(ctx)
		if err != nil {
			return fmt.Errorf("get public key: %w", err)
		}
	}

	verifier, err := signing.LoadPublicKeyPEM([]byte(pubKeyPEM))
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	if err := verifier.VerifySignature(bytes.NewReader(sig), bytes.NewReader(fileBytes)); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// =============================================================================
// Public-key cache
// =============================================================================

// cachedKey is the on-disk representation of a cached signing key.
type cachedKey struct {
	KeyID     string    `json:"key_id"`
	PublicKey string    `json:"public_key"`
	CachedAt  time.Time `json:"cached_at"`
}

// getPublicKey returns the current public key, using the on-disk cache when valid.
func (v *Verifier) getPublicKey(ctx context.Context) (string, error) {
	if cached, err := v.loadCachedKey(); err == nil {
		if time.Since(cached.CachedAt) < v.cacheTTL {
			return cached.PublicKey, nil
		}
	}

	key, err := v.fetchPublicKey(ctx)
	if err != nil {
		return "", err
	}

	// Best-effort cache write; don't fail verification if cache is unavailable.
	//nolint:errcheck // intentionally ignored; cache write failure is non-fatal
	_ = v.saveCachedKey(cachedKey{KeyID: key.KeyID, PublicKey: key.PublicKey, CachedAt: time.Now()})

	return key.PublicKey, nil
}

// signingKeyResponse mirrors types.SigningKeyResponse for decoding API responses
// without importing the types package into the CLI.
type signingKeyResponse struct {
	KeyID     string `json:"key_id"`
	PublicKey string `json:"public_key"`
}

func (v *Verifier) fetchPublicKey(ctx context.Context) (signingKeyResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.apiURL+"/v1/signing-keys/current", http.NoBody)
	if err != nil {
		return signingKeyResponse{}, fmt.Errorf("build request: %w", err)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return signingKeyResponse{}, fmt.Errorf("fetch signing key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return signingKeyResponse{}, fmt.Errorf("signing key API returned %d", resp.StatusCode)
	}

	var keyResp signingKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&keyResp); err != nil {
		return signingKeyResponse{}, fmt.Errorf("decode signing key response: %w", err)
	}

	return keyResp, nil
}

func (v *Verifier) loadCachedKey() (cachedKey, error) {
	data, err := os.ReadFile(filepath.Join(v.cacheDir, "signing-key.json"))
	if err != nil {
		return cachedKey{}, err
	}

	var k cachedKey
	if err := json.Unmarshal(data, &k); err != nil {
		return cachedKey{}, err
	}
	return k, nil
}

func (v *Verifier) saveCachedKey(k cachedKey) error {
	if err := os.MkdirAll(v.cacheDir, 0o750); err != nil {
		return err
	}

	data, err := json.MarshalIndent(k, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(v.cacheDir, "signing-key.json"), data, 0o600)
}

// =============================================================================
// Package-level convenience function
// =============================================================================

// Verify is a convenience wrapper that creates a default Verifier and verifies the
// given file. publicKey may be a PEM string; if empty the API is not called (fails).
func Verify(bottlePath, sigB64, publicKey string) error {
	return NewVerifier(Config{}).Verify(context.Background(), bottlePath, sigB64, publicKey)
}
