// Package signing provides cosign-based signing functionality for the bark pipeline.
package signing

import (
	"bytes"
	"context"
	"crypto"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
)

// Signer provides cosign-based artifact signing using a local private key.
type Signer struct {
	keyPath string
}

// NewSigner creates a new Signer backed by the private key at keyPath.
// The key must be in sigstore encrypted PEM format (produced by cosign generate-key-pair).
func NewSigner(keyPath string) *Signer {
	return &Signer{keyPath: keyPath}
}

// SignBottle signs a bottle tarball and returns a base64-encoded detached signature.
func (s *Signer) SignBottle(ctx context.Context, bottlePath string) (string, error) {
	return s.signFile(ctx, bottlePath)
}

// SignSBOM signs an SBOM file and returns a base64-encoded detached signature.
func (s *Signer) SignSBOM(ctx context.Context, sbomPath string) (string, error) {
	return s.signFile(ctx, sbomPath)
}

// signFile hashes and signs the file at filePath using the configured private key.
func (s *Signer) signFile(ctx context.Context, filePath string) (string, error) {
	_ = ctx // reserved for future use (e.g. KMS signing)

	passFunc := func(_ bool) ([]byte, error) { return []byte(""), nil }

	signer, err := signature.LoadSignerFromPEMFile(s.keyPath, crypto.SHA256, passFunc)
	if err != nil {
		return "", fmt.Errorf("failed to load signer: %w", err)
	}

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file to sign: %w", err)
	}

	sig, err := signer.SignMessage(bytes.NewReader(fileBytes))
	if err != nil {
		return "", fmt.Errorf("failed to sign file: %w", err)
	}

	return base64.StdEncoding.EncodeToString(sig), nil
}

// LoadPublicKeyPEM parses a PEM-encoded public key and returns a Verifier.
// Exported for use by the CLI verifier package.
func LoadPublicKeyPEM(pemBytes []byte) (signature.Verifier, error) {
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key PEM: %w", err)
	}

	v, err := signature.LoadVerifier(pubKey, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("load verifier: %w", err)
	}

	return v, nil
}
