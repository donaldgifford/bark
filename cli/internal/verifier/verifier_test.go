package verifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/donaldgifford/bark/pipeline/signing"
)

// devKeyPath returns the path to a file in infra/keys/ and skips the test if
// the file does not exist (keys are gitignored; run 'make dev-keys').
func devKeyPath(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("..", "..", "..", "infra", "keys", name)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Skip("dev key not found; run 'make dev-keys' to generate: " + p)
	}
	return p
}

// readDevPubKey reads cosign-dev.pub from infra/keys/ and skips if missing.
func readDevPubKey(t *testing.T) string {
	t.Helper()
	path := devKeyPath(t, "cosign-dev.pub")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dev public key: %v", err)
	}
	return string(data)
}

func TestVerifySignature(t *testing.T) {
	keyPath := devKeyPath(t, "cosign-dev.key")
	pubKey := readDevPubKey(t)
	bottlePath := filepath.Join("..", "..", "..", "testdata", "test-bottle.tar.gz")

	sig, err := signing.SignBottle(bottlePath, keyPath)
	if err != nil {
		t.Fatalf("Failed to sign bottle: %v", err)
	}

	v := NewVerifier(Config{})
	if err := v.Verify(context.Background(), bottlePath, sig, pubKey); err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	t.Log("Signature verification succeeded")
}

func TestVerifyWithAPIKey(t *testing.T) {
	keyPath := devKeyPath(t, "cosign-dev.key")
	pubKey := readDevPubKey(t)
	bottlePath := filepath.Join("..", "..", "..", "testdata", "test-bottle.tar.gz")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/signing-keys/current" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(signingKeyResponse{KeyID: "test-key-1", PublicKey: pubKey}); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	sig, err := signing.SignBottle(bottlePath, keyPath)
	if err != nil {
		t.Fatalf("Failed to sign bottle: %v", err)
	}

	v := NewVerifier(Config{APIURL: server.URL})
	if err := v.Verify(context.Background(), bottlePath, sig, ""); err != nil {
		t.Fatalf("Verify with API failed: %v", err)
	}
	t.Log("Signature verification with API succeeded")
}

func TestVerifyTamperedFile(t *testing.T) {
	keyPath := devKeyPath(t, "cosign-dev.key")
	pubKey := readDevPubKey(t)
	bottlePath := filepath.Join("..", "..", "..", "testdata", "test-bottle.tar.gz")

	tempDir := t.TempDir()
	tamperedPath := filepath.Join(tempDir, "tampered-bottle.tar.gz")

	originalData, err := os.ReadFile(bottlePath)
	if err != nil {
		t.Fatalf("Failed to read original file: %v", err)
	}

	tamperedData := make([]byte, len(originalData), len(originalData)+len("TAMPERED"))
	copy(tamperedData, originalData)
	tamperedData = append(tamperedData, []byte("TAMPERED")...)
	if err := os.WriteFile(tamperedPath, tamperedData, 0o644); err != nil {
		t.Fatalf("Failed to write tampered file: %v", err)
	}

	sig, err := signing.SignBottle(bottlePath, keyPath)
	if err != nil {
		t.Fatalf("Failed to sign bottle: %v", err)
	}

	v := NewVerifier(Config{})
	if err := v.Verify(context.Background(), tamperedPath, sig, pubKey); err == nil {
		t.Fatal("Expected verification to fail for tampered file, but it succeeded")
	} else {
		t.Logf("Correctly rejected tampered file: %v", err)
	}
}

func TestVerifyWrongSignature(t *testing.T) {
	pubKey := readDevPubKey(t)
	bottlePath := filepath.Join("..", "..", "..", "testdata", "test-bottle.tar.gz")

	fakeSignature := "dGhpcyBpcyBhIGZha2Ugc2lnbmF0dXJl" // "this is a fake signature" in base64

	v := NewVerifier(Config{})
	if err := v.Verify(context.Background(), bottlePath, fakeSignature, pubKey); err == nil {
		t.Fatal("Expected verification to fail for wrong signature, but it succeeded")
	} else {
		t.Logf("Correctly rejected wrong signature: %v", err)
	}
}

func TestVerifyAPIFailure(t *testing.T) {
	bottlePath := filepath.Join("..", "..", "..", "testdata", "test-bottle.tar.gz")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "Internal Server Error")
	}))
	defer server.Close()

	v := NewVerifier(Config{APIURL: server.URL})
	if err := v.Verify(context.Background(), bottlePath, "some-signature", ""); err == nil {
		t.Fatal("Expected verification to fail when API is down, but it succeeded")
	} else {
		t.Logf("Correctly failed when API is unavailable: %v", err)
	}
}

func TestKeyCache(t *testing.T) {
	pubKey := readDevPubKey(t)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/v1/signing-keys/current" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(signingKeyResponse{KeyID: "test-key-1", PublicKey: pubKey}); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	v := NewVerifier(Config{
		APIURL:   server.URL,
		CacheDir: tempDir,
		CacheTTL: 1 * time.Hour,
	})

	key1, err := v.getPublicKey(context.Background())
	if err != nil {
		t.Fatalf("First getPublicKey failed: %v", err)
	}
	key2, err := v.getPublicKey(context.Background())
	if err != nil {
		t.Fatalf("Second getPublicKey failed: %v", err)
	}
	if key1 != key2 {
		t.Fatal("Keys from API and cache don't match")
	}
	if requestCount != 1 {
		t.Fatalf("Expected 1 API request, got %d", requestCount)
	}
	t.Log("Key caching works correctly")
}

func TestCacheExpiry(t *testing.T) {
	pubKey := readDevPubKey(t)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/v1/signing-keys/current" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(signingKeyResponse{KeyID: "test-key-1", PublicKey: pubKey}); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	v := NewVerifier(Config{
		APIURL:   server.URL,
		CacheDir: tempDir,
		CacheTTL: 1 * time.Millisecond,
	})

	if _, err := v.getPublicKey(context.Background()); err != nil {
		t.Fatalf("First getPublicKey failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := v.getPublicKey(context.Background()); err != nil {
		t.Fatalf("Second getPublicKey failed: %v", err)
	}

	if requestCount != 2 {
		t.Fatalf("Expected 2 API requests due to cache expiry, got %d", requestCount)
	}
	t.Log("Cache expiry works correctly")
}

func TestConvenienceFunction(t *testing.T) {
	keyPath := devKeyPath(t, "cosign-dev.key")
	pubKey := readDevPubKey(t)
	bottlePath := filepath.Join("..", "..", "..", "testdata", "test-bottle.tar.gz")

	sig, err := signing.SignBottle(bottlePath, keyPath)
	if err != nil {
		t.Fatalf("Failed to sign bottle: %v", err)
	}
	if err := Verify(bottlePath, sig, pubKey); err != nil {
		t.Fatalf("Convenience Verify failed: %v", err)
	}
	t.Log("Convenience function works correctly")
}
