package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// Scanner wraps CLI tool invocations for syft, grype, and scancode.
// The execCmd variable is replaced in tests to avoid real tool calls.
type Scanner struct{}

// execCmd is the function used to create external commands. Replaced in tests.
//
//nolint:gochecknoglobals // intentional test seam
var execCmd = exec.CommandContext

// RunSyft invokes syft to produce a CycloneDX JSON SBOM.
// The SBOM is written to sbomPath.
func (Scanner) RunSyft(ctx context.Context, bottlePath, sbomPath string) error {
	cmd := execCmd(ctx, "syft", "packages", bottlePath,
		"-o", "cyclonedx-json="+sbomPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("syft: %w", err)
	}

	return nil
}

// RunGrype invokes grype against the SBOM at sbomPath and writes JSON to outputPath.
func (Scanner) RunGrype(ctx context.Context, sbomPath, outputPath string) error {
	cmd := execCmd(ctx, "grype", "sbom:"+sbomPath,
		"-o", "json", "--file", outputPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("grype: %w", err)
	}

	return nil
}

// RunScanCode invokes scancode against extractDir and writes JSON to outputPath.
func (Scanner) RunScanCode(ctx context.Context, extractDir, outputPath string) error {
	cmd := execCmd(ctx, "scancode", "--license", "--json-pp", outputPath, extractDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("scancode: %w", err)
	}

	return nil
}

// GrypeResult holds a parsed grype result evaluated against the CVE policy.
type GrypeResult struct {
	// CriticalCount is the number of critical-severity vulnerabilities found.
	CriticalCount int
	// HighCount is the number of high-severity vulnerabilities found.
	HighCount int
	// Passed is true when CriticalCount == 0.
	Passed bool
}

// ParseGrypeOutput reads the grype JSON file at path and evaluates it.
func ParseGrypeOutput(path string) (*GrypeResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read grype output: %w", err)
	}

	var out GrypeOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse grype output: %w", err)
	}

	res := &GrypeResult{}

	for _, m := range out.Matches {
		switch m.Vulnerability.Severity {
		case "Critical":
			res.CriticalCount++
		case "High":
			res.HighCount++
		}
	}

	res.Passed = res.CriticalCount == 0

	return res, nil
}
