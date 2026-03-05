package signing

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// requireSigningKey returns the path to the dev signing key and skips the
// test if the file does not exist (keys are gitignored; run 'make dev-keys').
func requireSigningKey(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "infra", "keys", "cosign-dev.key")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Skip("dev signing key not found; run 'make dev-keys' to generate: " + p)
	}
	return p
}

func TestSignBottle(t *testing.T) {
	keyPath := requireSigningKey(t)
	bottlePath := filepath.Join("..", "..", "testdata", "test-bottle.tar.gz")

	sig, err := SignBottle(bottlePath, keyPath)
	if err != nil {
		t.Fatalf("SignBottle failed: %v", err)
	}
	if len(sig) < 50 {
		t.Fatalf("Signature too short: got %d characters", len(sig))
	}
	t.Logf("Generated signature: %s", sig[:50]+"...")
}

func TestSignSBOM(t *testing.T) {
	keyPath := requireSigningKey(t)
	sbomPath := filepath.Join("..", "..", "testdata", "test-package.txt")

	sig, err := SignSBOM(sbomPath, keyPath)
	if err != nil {
		t.Fatalf("SignSBOM failed: %v", err)
	}
	if len(sig) < 50 {
		t.Fatalf("Signature too short: got %d characters", len(sig))
	}
	t.Logf("Generated SBOM signature: %s", sig[:50]+"...")
}

func TestSignerRoundTrip(t *testing.T) {
	keyPath := requireSigningKey(t)
	bottlePath := filepath.Join("..", "..", "testdata", "test-bottle.tar.gz")

	signer := NewSigner(keyPath)
	ctx := context.Background()

	sig, err := signer.SignBottle(ctx, bottlePath)
	if err != nil {
		t.Fatalf("SignBottle failed: %v", err)
	}
	sig2, err := signer.SignBottle(ctx, bottlePath)
	if err != nil {
		t.Fatalf("Second SignBottle failed: %v", err)
	}
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
		needsDevKey bool
	}{
		{
			name:        "nonexistent key",
			keyPath:     "/nonexistent/key.pem",
			filePath:    filepath.Join("..", "..", "testdata", "test-bottle.tar.gz"),
			expectError: "failed to load signer",
		},
		{
			name:        "nonexistent file",
			keyPath:     "", // filled in below if dev key present
			filePath:    "/nonexistent/file.tar.gz",
			expectError: "failed to read file to sign",
			needsDevKey: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyPath := tt.keyPath
			if tt.needsDevKey {
				keyPath = requireSigningKey(t)
			}
			signer := NewSigner(keyPath)
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
