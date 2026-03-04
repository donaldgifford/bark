package signing

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSignBottle(t *testing.T) {
	// Test data paths
	keyPath := filepath.Join("..", "..", "infra", "keys", "cosign-dev.key")
	bottlePath := filepath.Join("..", "..", "testdata", "test-bottle.tar.gz")

	// Test the convenience function
	sig, err := SignBottle(bottlePath, keyPath)
	if err != nil {
		t.Fatalf("SignBottle failed: %v", err)
	}

	if sig == "" {
		t.Fatal("SignBottle returned empty signature")
	}

	// Base64 signatures should be reasonably long
	if len(sig) < 50 {
		t.Fatalf("Signature too short: got %d characters", len(sig))
	}

	t.Logf("Generated signature: %s", sig[:50]+"...")
}

func TestSignSBOM(t *testing.T) {
	// Test data paths
	keyPath := filepath.Join("..", "..", "infra", "keys", "cosign-dev.key")
	sbomPath := filepath.Join("..", "..", "testdata", "test-package.txt") // Use text file as SBOM fixture

	// Test the convenience function
	sig, err := SignSBOM(sbomPath, keyPath)
	if err != nil {
		t.Fatalf("SignSBOM failed: %v", err)
	}

	if sig == "" {
		t.Fatal("SignSBOM returned empty signature")
	}

	// Base64 signatures should be reasonably long
	if len(sig) < 50 {
		t.Fatalf("Signature too short: got %d characters", len(sig))
	}

	t.Logf("Generated SBOM signature: %s", sig[:50]+"...")
}

func TestSignerRoundTrip(t *testing.T) {
	// Test data paths
	keyPath := filepath.Join("..", "..", "infra", "keys", "cosign-dev.key")
	bottlePath := filepath.Join("..", "..", "testdata", "test-bottle.tar.gz")

	// Create signer
	signer := NewSigner(keyPath)

	// Sign the bottle
	ctx := context.Background()
	sig, err := signer.SignBottle(ctx, bottlePath)
	if err != nil {
		t.Fatalf("SignBottle failed: %v", err)
	}

	// Verify we get consistent signatures for the same file
	sig2, err := signer.SignBottle(ctx, bottlePath)
	if err != nil {
		t.Fatalf("Second SignBottle failed: %v", err)
	}

	// Cosign signatures are not deterministic due to ECDSA randomness,
	// so we just verify both are valid non-empty strings.
	if sig == "" || sig2 == "" {
		t.Fatal("One of the signatures is empty")
	}

	t.Logf("First signature:  %s", sig[:50]+"...")
	t.Logf("Second signature: %s", sig2[:50]+"...")
}

func TestSignerErrors(t *testing.T) {
	tests := []struct {
		name        string
		keyPath     string
		filePath    string
		expectError string
	}{
		{
			name:        "nonexistent key",
			keyPath:     "/nonexistent/key.pem",
			filePath:    filepath.Join("..", "..", "testdata", "test-bottle.tar.gz"),
			expectError: "failed to load signer",
		},
		{
			name:        "nonexistent file",
			keyPath:     filepath.Join("..", "..", "infra", "keys", "cosign-dev.key"),
			filePath:    "/nonexistent/file.tar.gz",
			expectError: "failed to read file to sign",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer := NewSigner(tt.keyPath)
			_, err := signer.SignBottle(context.Background(), tt.filePath)
			if err == nil {
				t.Fatal("Expected error but got none")
			}

			if tt.expectError != "" && !contains(err.Error(), tt.expectError) {
				t.Fatalf("Expected error containing %q, got: %v", tt.expectError, err)
			}
		})
	}
}

// contains reports whether substr appears within s.
func contains(s, substr string) bool {
	return len(substr) <= len(s) && (substr == s || (substr != "" && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
