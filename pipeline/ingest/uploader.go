package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/donaldgifford/bark/api/s3"
	"github.com/donaldgifford/bark/pkg/manifest"
	"github.com/donaldgifford/bark/pkg/types"
)

// uploader handles S3 artifact uploads and bark API registration.
type uploader struct {
	s3Client   *s3.Client
	apiBaseURL string
	apiToken   string
	httpClient *http.Client
}

// newUploader creates an uploader.
func newUploader(s3Client *s3.Client, apiBaseURL, apiToken string) *uploader {
	return &uploader{
		s3Client:   s3Client,
		apiBaseURL: apiBaseURL,
		apiToken:   apiToken,
		httpClient: &http.Client{},
	}
}

// artifacts bundles the local paths and S3 keys for all pipeline artifacts.
type artifacts struct {
	BottlePath   string
	BottleSHA256 string
	SBOMPath     string
	GrypePath    string
	ScanCodePath string
	CosignSig    string

	// S3 keys populated during upload.
	BottleS3Key   string
	SBOMS3Key     string
	GrypeS3Key    string
	ScanCodeS3Key string

	// Scan pass/fail state.
	GrypePassed    bool
	ScanCodePassed bool
}

// upload uploads all artifacts to S3 and populates the S3 keys on a.
func (u *uploader) upload(ctx context.Context, name, version string, a *artifacts) error {
	type entry struct {
		localPath   string
		s3Key       *string
		key         string
		contentType string
	}

	entries := []entry{
		{a.BottlePath, &a.BottleS3Key, fmt.Sprintf("bottles/internal/%s/%s/bottle.tar.gz", name, version), "application/gzip"},
		{a.SBOMPath, &a.SBOMS3Key, fmt.Sprintf("sboms/%s/%s/sbom.cdx.json", name, version), "application/json"},
		{a.GrypePath, &a.GrypeS3Key, fmt.Sprintf("scans/%s/%s/grype.json", name, version), "application/json"},
		{a.ScanCodePath, &a.ScanCodeS3Key, fmt.Sprintf("scans/%s/%s/scancode.json", name, version), "application/json"},
	}

	for _, e := range entries {
		if e.localPath == "" {
			continue
		}

		f, err := os.Open(e.localPath)
		if err != nil {
			return fmt.Errorf("open %s: %w", e.localPath, err)
		}

		if putErr := u.s3Client.PutObject(ctx, e.key, f, e.contentType); putErr != nil {
			_ = f.Close() //nolint:errcheck // cleanup; original error takes precedence
			return fmt.Errorf("upload %s: %w", e.key, putErr)
		}

		if err := f.Close(); err != nil {
			return fmt.Errorf("close %s: %w", e.localPath, err)
		}

		*e.s3Key = e.key
	}

	return nil
}

// register calls the bark API to register the new version.
func (u *uploader) register(ctx context.Context, name, version string, a *artifacts) (string, error) {
	scanResults := []types.ScanResultRef{
		{
			Scanner:     "grype",
			ResultS3Key: a.GrypeS3Key,
			Passed:      a.GrypePassed,
		},
		{
			Scanner:     "scancode",
			ResultS3Key: a.ScanCodeS3Key,
			Passed:      a.ScanCodePassed,
		},
	}

	req := types.RegisterVersionRequest{
		Version:      version,
		BottleS3Key:  a.BottleS3Key,
		SHA256:       a.BottleSHA256,
		CosignSigRef: a.CosignSig,
		SBOMS3Key:    a.SBOMS3Key,
		ScanResults:  scanResults,
		Tier:         manifest.TierInternal,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal register request: %w", err)
	}

	url := u.apiBaseURL + "/v1/packages/" + name + "/versions"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build register request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+u.apiToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := u.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("register API call: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body) //nolint:errcheck // best-effort body for error message
		return "", fmt.Errorf("register API returned %d: %s", resp.StatusCode, string(body))
	}

	var regResp types.RegisterVersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return "", fmt.Errorf("decode register response: %w", err)
	}

	return regResp.VersionID, nil
}
