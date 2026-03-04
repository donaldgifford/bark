package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/bark/cli/internal/verifier"
	"github.com/donaldgifford/bark/pipeline/signing"
)

// TestSigningVerificationRoundTrip demonstrates the complete signing and verification flow.
func TestSigningVerificationRoundTrip(t *testing.T) {
	// Test data paths
	keyPath := filepath.Join("..", "infra", "keys", "cosign-dev.key")
	pubKeyPath := filepath.Join("..", "infra", "keys", "cosign-dev.pub")
	testFilePath := filepath.Join("..", "testdata", "test-bottle.tar.gz")

	// Ensure test file exists
	if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
		t.Fatalf("Test file does not exist: %s", testFilePath)
	}

	// Read the public key
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		t.Fatalf("Failed to read public key: %v", err)
	}
	pubKey := string(pubKeyBytes)

	// Step 1: Sign the file using the pipeline signing package
	signer := signing.NewSigner(keyPath)
	sig, err := signer.SignBottle(context.Background(), testFilePath)
	if err != nil {
		t.Fatalf("Failed to sign bottle: %v", err)
	}

	t.Logf("Generated signature: %s...", sig[:50])

	// Step 2: Verify the signature using the CLI verifier package
	verifierInstance := verifier.NewVerifier(verifier.Config{})
	err = verifierInstance.Verify(context.Background(), testFilePath, sig, pubKey)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}

	t.Log("Round-trip signing and verification successful!")

	// Step 3: Test convenience functions
	sig2, err := signing.SignBottle(testFilePath, keyPath)
	if err != nil {
		t.Fatalf("Failed to sign with convenience function: %v", err)
	}

	err = verifier.Verify(testFilePath, sig2, pubKey)
	if err != nil {
		t.Fatalf("Failed to verify with convenience function: %v", err)
	}

	t.Log("Convenience functions work correctly!")

	// Step 4: Verify signatures are valid but non-deterministic (due to ECDSA randomness)
	if sig == sig2 {
		t.Log("Note: Signatures are identical (unexpected but not an error)")
	} else {
		t.Log("Signatures are different as expected (ECDSA randomness)")
	}
}

// TestSigningSBOMRoundTrip tests SBOM signing and verification.
func TestSigningSBOMRoundTrip(t *testing.T) {
	// Test data paths
	keyPath := filepath.Join("..", "infra", "keys", "cosign-dev.key")
	pubKeyPath := filepath.Join("..", "infra", "keys", "cosign-dev.pub")
	sbomPath := filepath.Join("..", "testdata", "test-package.txt") // Using text file as SBOM fixture

	// Read the public key
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		t.Fatalf("Failed to read public key: %v", err)
	}
	pubKey := string(pubKeyBytes)

	// Sign the SBOM
	signer := signing.NewSigner(keyPath)
	sig, err := signer.SignSBOM(context.Background(), sbomPath)
	if err != nil {
		t.Fatalf("Failed to sign SBOM: %v", err)
	}

	// Verify the SBOM signature
	verifierInstance := verifier.NewVerifier(verifier.Config{})
	err = verifierInstance.Verify(context.Background(), sbomPath, sig, pubKey)
	if err != nil {
		t.Fatalf("Failed to verify SBOM signature: %v", err)
	}

	t.Log("SBOM signing and verification successful!")
}
