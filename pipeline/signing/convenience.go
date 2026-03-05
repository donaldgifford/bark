// Package signing convenience functions for backward compatibility with the interface specified in the requirements.
package signing

import "context"

// SignBottle signs a bottle tarball using the specified key path and returns a base64-encoded detached signature.
// This is a convenience function that matches the interface specified in the requirements.
func SignBottle(bottlePath, keyPath string) (string, error) {
	signer := NewSigner(keyPath)
	return signer.SignBottle(context.Background(), bottlePath)
}

// SignSBOM signs an SBOM file using the specified key path and returns a base64-encoded detached signature.
// This is a convenience function that matches the interface specified in the requirements.
func SignSBOM(sbomPath, keyPath string) (string, error) {
	signer := NewSigner(keyPath)
	return signer.SignSBOM(context.Background(), sbomPath)
}
