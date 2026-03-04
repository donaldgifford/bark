package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/donaldgifford/bark/api/s3"
	"github.com/donaldgifford/bark/pipeline/policy"
	"github.com/donaldgifford/bark/pipeline/signing"
)

// Ingester orchestrates the full pipeline for ingesting an internal-tier bottle.
type Ingester struct {
	cfg      *Config
	scanner  Scanner
	uploader *uploader
}

// New creates an Ingester, initialising the S3 client and validating config.
func New(ctx context.Context, cfg *Config) (*Ingester, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	s3Client, err := s3.New(ctx, cfg.S3)
	if err != nil {
		return nil, fmt.Errorf("init S3 client: %w", err)
	}

	return &Ingester{
		cfg:      cfg,
		scanner:  Scanner{},
		uploader: newUploader(s3Client, cfg.APIBaseURL, cfg.APIToken),
	}, nil
}

// Ingest runs the full pipeline for the given bottle and returns the result.
func (i *Ingester) Ingest(ctx context.Context, req Request) (*Result, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	workDir, err := os.MkdirTemp("", "bark-ingest-*")
	if err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}

	defer func() { _ = os.RemoveAll(workDir) }() //nolint:errcheck // best-effort cleanup

	return i.run(ctx, req, workDir)
}

// run is the inner orchestration step separated out to reduce nesting.
func (i *Ingester) run(ctx context.Context, req Request, workDir string) (*Result, error) {
	result := &Result{Success: true}

	// 1. Compute SHA-256 of the bottle.
	bottleSHA256, err := sha256File(req.BottlePath)
	if err != nil {
		return nil, fmt.Errorf("hash bottle: %w", err)
	}

	arts := &artifacts{
		BottlePath:   req.BottlePath,
		BottleSHA256: bottleSHA256,
		SBOMPath:     workDir + "/sbom.cdx.json",
		GrypePath:    workDir + "/grype.json",
		ScanCodePath: workDir + "/scancode.json",
	}

	// 2. Generate SBOM.
	if err := i.scanner.RunSyft(ctx, req.BottlePath, arts.SBOMPath); err != nil {
		return nil, fmt.Errorf("generate SBOM: %w", err)
	}

	// 3. Vulnerability scan.
	grypeResult, err := i.runGrype(ctx, arts, result)
	if err != nil {
		return nil, err
	}

	arts.GrypePassed = grypeResult.Passed

	// 4. License scan.
	if err := i.runScanCode(ctx, req.BottlePath, arts, result, workDir); err != nil {
		return nil, err
	}

	// Bail early if either scan is blocking.
	if !result.Success {
		return result, nil
	}

	// 5. Sign the bottle. SignBottle does not accept a context (CPU-only work).
	sig, err := signing.SignBottle(req.BottlePath, i.cfg.SigningKeyPath) //nolint:contextcheck // SignBottle is CPU-only; no context needed
	if err != nil {
		return nil, fmt.Errorf("sign bottle: %w", err)
	}

	arts.CosignSig = sig

	// 6. Upload artifacts.
	if err := i.uploader.upload(ctx, req.Name, req.Version, arts); err != nil {
		return nil, fmt.Errorf("upload artifacts: %w", err)
	}

	// 7. Register with API.
	regID, err := i.uploader.register(ctx, req.Name, req.Version, arts)
	if err != nil {
		return nil, fmt.Errorf("register version: %w", err)
	}

	result.RegistrationID = regID

	return result, nil
}

// runGrype runs grype and evaluates the result, updating result in place.
func (i *Ingester) runGrype(ctx context.Context, arts *artifacts, result *Result) (*GrypeResult, error) {
	if err := i.scanner.RunGrype(ctx, arts.SBOMPath, arts.GrypePath); err != nil {
		return nil, fmt.Errorf("run grype: %w", err)
	}

	grypeResult, err := ParseGrypeOutput(arts.GrypePath)
	if err != nil {
		return nil, fmt.Errorf("parse grype output: %w", err)
	}

	summary := ScanSummary{
		Scanner: "grype",
		Passed:  grypeResult.Passed,
		Details: fmt.Sprintf("critical=%d high=%d", grypeResult.CriticalCount, grypeResult.HighCount),
	}

	result.Scans = append(result.Scans, summary)

	if !grypeResult.Passed {
		result.Success = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("grype: %d critical CVE(s) found", grypeResult.CriticalCount))
	}

	return grypeResult, nil
}

// runScanCode runs scancode, evaluates the license policy, and updates result.
func (i *Ingester) runScanCode(
	ctx context.Context,
	bottlePath string,
	arts *artifacts,
	result *Result,
	workDir string,
) error {
	extractDir := workDir + "/extracted"
	if err := os.MkdirAll(extractDir, 0o750); err != nil {
		return fmt.Errorf("create extract dir: %w", err)
	}

	if err := i.scanner.RunScanCode(ctx, bottlePath, arts.ScanCodePath); err != nil {
		// scancode failure is non-fatal; record warning.
		result.Warnings = append(result.Warnings, fmt.Sprintf("scancode: %v", err))
		arts.ScanCodePassed = true // don't block on tool failure

		return nil
	}

	policyCfg, err := policy.LoadConfig(i.cfg.PolicyPath)
	if err != nil {
		return fmt.Errorf("load license policy: %w", err)
	}

	ev := policy.NewEvaluator(policyCfg)

	pr, err := ev.EvaluateFile(arts.ScanCodePath)
	if err != nil {
		return fmt.Errorf("evaluate license policy: %w", err)
	}

	arts.ScanCodePassed = pr.Passed

	details := fmt.Sprintf("licenses=%v", pr.AllLicenses)
	if len(pr.Violations) > 0 {
		details += fmt.Sprintf(" violations=%v", pr.Violations)
	}

	result.Scans = append(result.Scans, ScanSummary{
		Scanner: "scancode",
		Passed:  pr.Passed,
		Details: details,
	})

	for _, w := range pr.Warnings {
		result.Warnings = append(result.Warnings, fmt.Sprintf("license warning: %s", w))
	}

	if !pr.Passed {
		result.Success = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("license violation(s): %v", pr.Violations))
	}

	return nil
}

// sha256File computes the hex-encoded SHA-256 of the file at path.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}

	defer f.Close() //nolint:errcheck // read-only; close error not actionable

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
