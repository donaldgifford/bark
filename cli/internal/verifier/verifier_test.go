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

func TestVerifySignature(t *testing.T) {
	// Test data paths
	keyPath := filepath.Join("..", "..", "..", "infra", "keys", "cosign-dev.key")
	pubKeyPath := filepath.Join("..", "..", "..", "infra", "keys", "cosign-dev.pub")
	bottlePath := filepath.Join("..", "..", "..", "testdata", "test-bottle.tar.gz")

	// Read the public key
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		t.Fatalf("Failed to read public key: %v", err)
	}
	pubKey := string(pubKeyBytes)

	// Sign the bottle
	sig, err := signing.SignBottle(bottlePath, keyPath)
	if err != nil {
		t.Fatalf("Failed to sign bottle: %v", err)
	}

	// Verify the signature using explicit public key
	verifier := NewVerifier(Config{})
	err = verifier.Verify(context.Background(), bottlePath, sig, pubKey)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	t.Log("Signature verification succeeded")
}

func TestVerifyWithAPIKey(t *testing.T) {
	// Test data paths
	keyPath := filepath.Join("..", "..", "..", "infra", "keys", "cosign-dev.key")
	pubKeyPath := filepath.Join("..", "..", "..", "infra", "keys", "cosign-dev.pub")
	bottlePath := filepath.Join("..", "..", "..", "testdata", "test-bottle.tar.gz")

	// Read the public key
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		t.Fatalf("Failed to read public key: %v", err)
	}
	pubKey := string(pubKeyBytes)

	// Create a mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/signing-keys/current" {
			http.NotFound(w, r)
			return
		}

		response := signingKeyResponse{
			KeyID:     "test-key-1",
			PublicKey: pubKey,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Sign the bottle
	sig, err := signing.SignBottle(bottlePath, keyPath)
	if err != nil {
		t.Fatalf("Failed to sign bottle: %v", err)
	}

	// Verify using API (empty publicKey triggers API fetch)
	verifier := NewVerifier(Config{
		APIURL: server.URL,
	})
	err = verifier.Verify(context.Background(), bottlePath, sig, "")
	if err != nil {
		t.Fatalf("Verify with API failed: %v", err)
	}

	t.Log("Signature verification with API succeeded")
}

func TestVerifyTamperedFile(t *testing.T) {
	// Test data paths
	keyPath := filepath.Join("..", "..", "..", "infra", "keys", "cosign-dev.key")
	pubKeyPath := filepath.Join("..", "..", "..", "infra", "keys", "cosign-dev.pub")
	bottlePath := filepath.Join("..", "..", "..", "testdata", "test-bottle.tar.gz")

	// Create a temporary tampered file
	tempDir := t.TempDir()
	tamperedPath := filepath.Join(tempDir, "tampered-bottle.tar.gz")

	// Copy original file
	originalData, err := os.ReadFile(bottlePath)
	if err != nil {
		t.Fatalf("Failed to read original file: %v", err)
	}

	// Write tampered data (append some bytes)
	tamperedData := make([]byte, len(originalData), len(originalData)+len("TAMPERED"))
	copy(tamperedData, originalData)
	tamperedData = append(tamperedData, []byte("TAMPERED")...)
	err = os.WriteFile(tamperedPath, tamperedData, 0o644)
	if err != nil {
		t.Fatalf("Failed to write tampered file: %v", err)
	}

	// Read the public key
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		t.Fatalf("Failed to read public key: %v", err)
	}
	pubKey := string(pubKeyBytes)

	// Sign the original bottle
	sig, err := signing.SignBottle(bottlePath, keyPath)
	if err != nil {
		t.Fatalf("Failed to sign bottle: %v", err)
	}

	// Try to verify tampered file with original signature - should fail
	verifier := NewVerifier(Config{})
	err = verifier.Verify(context.Background(), tamperedPath, sig, pubKey)
	if err == nil {
		t.Fatal("Expected verification to fail for tampered file, but it succeeded")
	}

	t.Logf("Correctly rejected tampered file: %v", err)
}

func TestVerifyWrongSignature(t *testing.T) {
	// Test data paths
	pubKeyPath := filepath.Join("..", "..", "..", "infra", "keys", "cosign-dev.pub")
	bottlePath := filepath.Join("..", "..", "..", "testdata", "test-bottle.tar.gz")

	// Read the public key
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		t.Fatalf("Failed to read public key: %v", err)
	}
	pubKey := string(pubKeyBytes)

	// Use a fake signature
	fakeSignature := "dGhpcyBpcyBhIGZha2Ugc2lnbmF0dXJl" // "this is a fake signature" in base64

	// Try to verify with wrong signature - should fail
	verifier := NewVerifier(Config{})
	err = verifier.Verify(context.Background(), bottlePath, fakeSignature, pubKey)
	if err == nil {
		t.Fatal("Expected verification to fail for wrong signature, but it succeeded")
	}

	t.Logf("Correctly rejected wrong signature: %v", err)
}

func TestVerifyAPIFailure(t *testing.T) {
	// Test data paths
	bottlePath := filepath.Join("..", "..", "..", "testdata", "test-bottle.tar.gz")

	// Create a mock API server that always returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "Internal Server Error")
	}))
	defer server.Close()

	// Verify without public key should trigger API call and fail
	verifier := NewVerifier(Config{
		APIURL: server.URL,
	})
	err := verifier.Verify(context.Background(), bottlePath, "some-signature", "")
	if err == nil {
		t.Fatal("Expected verification to fail when API is down, but it succeeded")
	}

	t.Logf("Correctly failed when API is unavailable: %v", err)
}

func TestKeyCache(t *testing.T) {
	// Test data paths
	pubKeyPath := filepath.Join("..", "..", "..", "infra", "keys", "cosign-dev.pub")

	// Read the public key
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		t.Fatalf("Failed to read public key: %v", err)
	}
	pubKey := string(pubKeyBytes)

	requestCount := 0
	// Create a mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/v1/signing-keys/current" {
			http.NotFound(w, r)
			return
		}

		response := signingKeyResponse{
			KeyID:     "test-key-1",
			PublicKey: pubKey,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Use temporary cache directory
	tempDir := t.TempDir()

	// Create verifier with short cache TTL for testing
	verifier := NewVerifier(Config{
		APIURL:   server.URL,
		CacheDir: tempDir,
		CacheTTL: 1 * time.Hour, // Long enough for this test
	})

	// First call should hit the API
	key1, err := verifier.getPublicKey(context.Background())
	if err != nil {
		t.Fatalf("First getPublicKey failed: %v", err)
	}

	// Second call should use cache (request count should not increment)
	key2, err := verifier.getPublicKey(context.Background())
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
	// Test data paths
	pubKeyPath := filepath.Join("..", "..", "..", "infra", "keys", "cosign-dev.pub")

	// Read the public key
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		t.Fatalf("Failed to read public key: %v", err)
	}
	pubKey := string(pubKeyBytes)

	requestCount := 0
	// Create a mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/v1/signing-keys/current" {
			http.NotFound(w, r)
			return
		}

		response := signingKeyResponse{
			KeyID:     "test-key-1",
			PublicKey: pubKey,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Use temporary cache directory
	tempDir := t.TempDir()

	// Create verifier with very short cache TTL
	verifier := NewVerifier(Config{
		APIURL:   server.URL,
		CacheDir: tempDir,
		CacheTTL: 1 * time.Millisecond, // Very short for testing
	})

	// First call should hit the API
	_, err = verifier.getPublicKey(context.Background())
	if err != nil {
		t.Fatalf("First getPublicKey failed: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(10 * time.Millisecond)

	// Second call should hit API again due to expiry
	_, err = verifier.getPublicKey(context.Background())
	if err != nil {
		t.Fatalf("Second getPublicKey failed: %v", err)
	}

	if requestCount != 2 {
		t.Fatalf("Expected 2 API requests due to cache expiry, got %d", requestCount)
	}

	t.Log("Cache expiry works correctly")
}

func TestConvenienceFunction(t *testing.T) {
	// Test data paths
	keyPath := filepath.Join("..", "..", "..", "infra", "keys", "cosign-dev.key")
	pubKeyPath := filepath.Join("..", "..", "..", "infra", "keys", "cosign-dev.pub")
	bottlePath := filepath.Join("..", "..", "..", "testdata", "test-bottle.tar.gz")

	// Read the public key
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		t.Fatalf("Failed to read public key: %v", err)
	}
	pubKey := string(pubKeyBytes)

	// Sign the bottle
	sig, err := signing.SignBottle(bottlePath, keyPath)
	if err != nil {
		t.Fatalf("Failed to sign bottle: %v", err)
	}

	// Test the convenience function
	err = Verify(bottlePath, sig, pubKey)
	if err != nil {
		t.Fatalf("Convenience Verify failed: %v", err)
	}

	t.Log("Convenience function works correctly")
}
