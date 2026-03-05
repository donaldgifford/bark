package policy

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	t.Run("loads valid policy file", func(t *testing.T) {
		t.Parallel()

		cfg, err := LoadConfig("testdata/license-policy.yaml")
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}

		if len(cfg.AllowedSPDX) == 0 {
			t.Error("expected AllowedSPDX to be non-empty")
		}

		if len(cfg.DeniedSPDX) == 0 {
			t.Error("expected DeniedSPDX to be non-empty")
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		t.Parallel()

		_, err := LoadConfig("/nonexistent/path/policy.yaml")
		if err == nil {
			t.Error("expected error for missing file, got nil")
		}
	})

	t.Run("returns error for invalid YAML", func(t *testing.T) {
		t.Parallel()

		tmp := filepath.Join(t.TempDir(), "bad.yaml")
		if err := os.WriteFile(tmp, []byte(":\ninvalid: [yaml"), 0o600); err != nil {
			t.Fatalf("write bad yaml: %v", err)
		}

		_, err := LoadConfig(tmp)
		if err == nil {
			t.Error("expected error for invalid YAML, got nil")
		}
	})
}

func TestEvaluator_Evaluate(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		AllowedSPDX: []string{"MIT", "Apache-2.0"},
		WarnSPDX:    []string{"GPL-3.0-only"},
		DeniedSPDX:  []string{"BUSL-1.1", "SSPL-1.0"},
	}

	ev := NewEvaluator(cfg)

	tests := []struct {
		name          string
		files         []ScanCodeFile
		wantPassed    bool
		wantViolation string
		wantWarning   string
	}{
		{
			name:       "all allowed licenses - passes",
			files:      makeScanFiles("MIT", "Apache-2.0"),
			wantPassed: true,
		},
		{
			name:          "denied license blocks pipeline",
			files:         makeScanFiles("MIT", "BUSL-1.1"),
			wantPassed:    false,
			wantViolation: "BUSL-1.1",
		},
		{
			name:        "warned license passes with warning",
			files:       makeScanFiles("MIT", "GPL-3.0-only"),
			wantPassed:  true,
			wantWarning: "GPL-3.0-only",
		},
		{
			name:       "empty files - passes with no licenses",
			files:      nil,
			wantPassed: true,
		},
		{
			name:       "unlisted license is implicitly allowed",
			files:      makeScanFiles("BSD-3-Clause"),
			wantPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := &ScanCodeOutput{Files: tt.files}
			result := ev.Evaluate(out)

			if result.Passed != tt.wantPassed {
				t.Errorf("Passed = %v, want %v", result.Passed, tt.wantPassed)
			}

			if tt.wantViolation != "" && !slices.Contains(result.Violations, tt.wantViolation) {
				t.Errorf("expected violation %q in %v", tt.wantViolation, result.Violations)
			}

			if tt.wantWarning != "" && !slices.Contains(result.Warnings, tt.wantWarning) {
				t.Errorf("expected warning %q in %v", tt.wantWarning, result.Warnings)
			}
		})
	}
}

func TestEvaluator_EvaluateFile(t *testing.T) {
	t.Parallel()

	t.Run("reads and evaluates scancode JSON", func(t *testing.T) {
		t.Parallel()

		const scanCodeJSON = `{
			"files": [
				{"licenses": [{"spdx_license_key": "MIT"}]},
				{"licenses": [{"spdx_license_key": "Apache-2.0"}]}
			]
		}`

		path := filepath.Join(t.TempDir(), "scancode.json")
		if err := os.WriteFile(path, []byte(scanCodeJSON), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		cfg := &Config{AllowedSPDX: []string{"MIT", "Apache-2.0"}}
		ev := NewEvaluator(cfg)

		result, err := ev.EvaluateFile(path)
		if err != nil {
			t.Fatalf("EvaluateFile: %v", err)
		}

		if !result.Passed {
			t.Errorf("expected Passed=true, violations: %v", result.Violations)
		}

		if len(result.AllLicenses) != 2 {
			t.Errorf("expected 2 unique licenses, got %d: %v", len(result.AllLicenses), result.AllLicenses)
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		t.Parallel()

		ev := NewEvaluator(&Config{})

		_, err := ev.EvaluateFile("/nonexistent/scancode.json")
		if err == nil {
			t.Error("expected error for missing file, got nil")
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(path, []byte("not-json{"), 0o600); err != nil {
			t.Fatalf("write bad json: %v", err)
		}

		ev := NewEvaluator(&Config{})

		_, err := ev.EvaluateFile(path)
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})
}

// makeScanFiles builds a []ScanCodeFile from a list of SPDX IDs.
func makeScanFiles(spdxIDs ...string) []ScanCodeFile {
	files := make([]ScanCodeFile, 0, len(spdxIDs))
	for _, id := range spdxIDs {
		files = append(files, ScanCodeFile{
			Licenses: []ScanCodeLicense{{SPDXLicenseKey: id}},
		})
	}

	return files
}
