package ingest

import (
	"strings"
	"testing"
)

func TestResult_FormatPRComment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		result      Result
		wantSubstrs []string
	}{
		{
			name: "success with scans",
			result: Result{
				Success:        true,
				RegistrationID: "ver-abc123",
				Scans: []ScanSummary{
					{Scanner: "grype", Passed: true, Details: "critical=0 high=0"},
					{Scanner: "scancode", Passed: true, Details: "licenses=[MIT]"},
				},
			},
			wantSubstrs: []string{
				"✅ Passed",
				"grype",
				"critical=0 high=0",
				"scancode",
				"ver-abc123",
			},
		},
		{
			name: "failure with errors",
			result: Result{
				Success: false,
				Errors:  []string{"grype: 2 critical CVE(s) found"},
				Scans: []ScanSummary{
					{Scanner: "grype", Passed: false, Details: "critical=2 high=3"},
				},
			},
			wantSubstrs: []string{
				"❌ Failed",
				"grype: 2 critical CVE(s) found",
			},
		},
		{
			name: "warnings are included",
			result: Result{
				Success:  true,
				Warnings: []string{"license warning: GPL-3.0-only"},
			},
			wantSubstrs: []string{
				"⚠️",
				"GPL-3.0-only",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			comment := tt.result.FormatPRComment("mytool", "1.0.0")

			for _, want := range tt.wantSubstrs {
				if !strings.Contains(comment, want) {
					t.Errorf("PR comment missing %q\n--- comment ---\n%s", want, comment)
				}
			}
		})
	}
}
