package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/bark/cli/internal/verifier"
	"github.com/donaldgifford/bark/pipeline/signing"
)

// devKeyPath returns the path to a file in infra/keys/ and skips the test if
// the file does not exist (keys are gitignored; run 'make dev-keys').
func devKeyPath(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("..", "infra", "keys", name)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Skip("dev key not found; run 'make dev-keys' to generate: " + p)
	}
	return p
}

// readDevKey reads a key file from infra/keys/ and skips the test if missing.
func readDevKey(t *testing.T, name string) string {
	t.Helper()
	path := devKeyPath(t, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dev key %s: %v", name, err)
	}
	return string(data)
}

// TestSigningVerificationRoundTrip demonstrates the complete signing and verification flow.
func TestSigningVerificationRoundTrip(t *testing.T) {
	keyPath := devKeyPath(t, "cosign-dev.key")
	pubKey := readDevKey(t, "cosign-dev.pub")
	testFilePath := filepath.Join("..", "testdata", "test-bottle.tar.gz")

	signer := signing.NewSigner(keyPath)
	sig, err := signer.SignBottle(context.Background(), testFilePath)
	if err != nil {
		t.Fatalf("Failed to sign bottle: %v", err)
	}
	t.Logf("Generated signature: %s...", sig[:50])

	v := verifier.NewVerifier(verifier.Config{})
	if err := v.Verify(context.Background(), testFilePath, sig, pubKey); err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	t.Log("Round-trip signing and verification successful!")

	sig2, err := signing.SignBottle(testFilePath, keyPath)
	if err != nil {
		t.Fatalf("Failed to sign with convenience function: %v", err)
	}
	if err := verifier.Verify(testFilePath, sig2, pubKey); err != nil {
		t.Fatalf("Failed to verify with convenience function: %v", err)
	}
	t.Log("Convenience functions work correctly!")

	if sig == sig2 {
		t.Log("Note: Signatures are identical (unexpected but not an error)")
	} else {
		t.Log("Signatures are different as expected (ECDSA randomness)")
	}
}

// TestSigningSBOMRoundTrip tests SBOM signing and verification.
func TestSigningSBOMRoundTrip(t *testing.T) {
	keyPath := devKeyPath(t, "cosign-dev.key")
	pubKey := readDevKey(t, "cosign-dev.pub")
	sbomPath := filepath.Join("..", "testdata", "test-package.txt")

	signer := signing.NewSigner(keyPath)
	sig, err := signer.SignSBOM(context.Background(), sbomPath)
	if err != nil {
		t.Fatalf("Failed to sign SBOM: %v", err)
	}

	v := verifier.NewVerifier(verifier.Config{})
	if err := v.Verify(context.Background(), sbomPath, sig, pubKey); err != nil {
		t.Fatalf("Failed to verify SBOM signature: %v", err)
	}
	t.Log("SBOM signing and verification successful!")
}
