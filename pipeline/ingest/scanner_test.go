package ingest

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestParseGrypeOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		fixture       string
		wantPassed    bool
		wantCritical  int
		wantHigh      int
		wantErrSubstr string
	}{
		{
			name:         "no vulnerabilities - passes",
			fixture:      "testdata/grype-clean.json",
			wantPassed:   true,
			wantCritical: 0,
			wantHigh:     0,
		},
		{
			name:         "critical CVE - fails",
			fixture:      "testdata/grype-critical.json",
			wantPassed:   false,
			wantCritical: 1,
			wantHigh:     1,
		},
		{
			name:         "high only - passes (no criticals)",
			fixture:      "testdata/grype-high-only.json",
			wantPassed:   true,
			wantCritical: 0,
			wantHigh:     1,
		},
		{
			name:          "missing file returns error",
			fixture:       "/nonexistent/grype.json",
			wantErrSubstr: "read grype output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := ParseGrypeOutput(tt.fixture)

			if tt.wantErrSubstr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErrSubstr)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseGrypeOutput: %v", err)
			}

			if result.Passed != tt.wantPassed {
				t.Errorf("Passed = %v, want %v", result.Passed, tt.wantPassed)
			}

			if result.CriticalCount != tt.wantCritical {
				t.Errorf("CriticalCount = %d, want %d", result.CriticalCount, tt.wantCritical)
			}

			if result.HighCount != tt.wantHigh {
				t.Errorf("HighCount = %d, want %d", result.HighCount, tt.wantHigh)
			}
		})
	}
}

func TestScanner_RunSyft_MockedCommand(t *testing.T) {
	// Not parallel: mutates package-level execCmd.
	old := execCmd
	t.Cleanup(func() { execCmd = old })

	execCmd = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		// Return a no-op command to simulate a successful syft run.
		return exec.Command("true") //nolint:noctx // test helper; context not needed for 'true'
	}

	s := Scanner{}
	if err := s.RunSyft(context.Background(), "bottle.tar.gz", "/tmp/sbom.json"); err != nil {
		t.Errorf("RunSyft with mocked success: %v", err)
	}
}

func TestScanner_RunGrype_MockedFailure(t *testing.T) {
	// Not parallel: mutates package-level execCmd.
	old := execCmd
	t.Cleanup(func() { execCmd = old })

	execCmd = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("false") //nolint:noctx // test helper; context not needed
	}

	s := Scanner{}
	err := s.RunGrype(context.Background(), "sbom.json", "/tmp/grype.json")

	if err == nil {
		t.Error("expected error from failing grype command, got nil")
	}

	if !strings.Contains(err.Error(), "grype") {
		t.Errorf("error = %q, expected 'grype' prefix", err.Error())
	}
}

func TestScanner_RunScanCode_MockedCommand(t *testing.T) {
	// Not parallel: mutates package-level execCmd.
	old := execCmd
	t.Cleanup(func() { execCmd = old })

	execCmd = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("true") //nolint:noctx // test helper; context not needed for 'true'
	}

	s := Scanner{}
	if err := s.RunScanCode(context.Background(), "/extract", "/tmp/sc.json"); err != nil {
		t.Errorf("RunScanCode with mocked success: %v", err)
	}
}

func TestParseGrypeOutput_InvalidJSON(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir() + "/bad.json"
	if err := os.WriteFile(tmp, []byte("not-valid-json{"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := ParseGrypeOutput(tmp)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}
