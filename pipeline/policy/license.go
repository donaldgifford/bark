package policy

import (
	"encoding/json"
	"fmt"
	"os"
)

// ScanCodeOutput is the top-level structure of a scancode --json-pp file.
type ScanCodeOutput struct {
	Files []ScanCodeFile `json:"files"`
}

// ScanCodeFile represents one file entry in scancode output.
type ScanCodeFile struct {
	Licenses []ScanCodeLicense `json:"licenses"`
}

// ScanCodeLicense contains the license metadata for a single match.
type ScanCodeLicense struct {
	SPDXLicenseKey string `json:"spdx_license_key"`
}

// PolicyResult contains the outcome of evaluating scancode output against a Config.
type PolicyResult struct {
	// AllLicenses holds every unique SPDX identifier found across the scanned files.
	AllLicenses []string
	// Violations lists SPDX identifiers that appear in the denied list.
	Violations []string
	// Warnings lists SPDX identifiers that appear in the warn list.
	Warnings []string
	// Passed is true when Violations is empty.
	Passed bool
}

// Evaluator evaluates ScanCode output against a license policy.
type Evaluator struct {
	cfg *Config
}

// NewEvaluator creates an Evaluator for the given policy Config.
func NewEvaluator(cfg *Config) *Evaluator {
	return &Evaluator{cfg: cfg}
}

// EvaluateFile reads the scancode JSON output at path and evaluates it.
func (e *Evaluator) EvaluateFile(path string) (*PolicyResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scancode output: %w", err)
	}

	var out ScanCodeOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse scancode output: %w", err)
	}

	return e.Evaluate(&out), nil
}

// Evaluate evaluates the in-memory ScanCodeOutput against the policy.
func (e *Evaluator) Evaluate(out *ScanCodeOutput) *PolicyResult {
	unique := collectUniqueLicenses(out)

	denied := e.cfg.deniedSet()
	warn := e.cfg.warnSet()

	result := &PolicyResult{
		AllLicenses: unique,
		Passed:      true,
	}

	for _, id := range unique {
		if _, ok := denied[id]; ok {
			result.Violations = append(result.Violations, id)
			result.Passed = false
		} else if _, ok := warn[id]; ok {
			result.Warnings = append(result.Warnings, id)
		}
	}

	return result
}

// collectUniqueLicenses extracts the sorted list of unique SPDX identifiers.
func collectUniqueLicenses(out *ScanCodeOutput) []string {
	seen := make(map[string]struct{})

	for _, f := range out.Files {
		for _, lic := range f.Licenses {
			if lic.SPDXLicenseKey != "" {
				seen[lic.SPDXLicenseKey] = struct{}{}
			}
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}

	return ids
}
