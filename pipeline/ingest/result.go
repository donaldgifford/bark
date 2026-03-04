package ingest

import (
	"fmt"
	"strings"
)

// Result contains the outcome of a bottle ingest operation.
type Result struct {
	// Success is true when the pipeline completed without blocking failures.
	Success bool
	// RegistrationID is the ID returned by the bark API on successful registration.
	RegistrationID string
	// Scans holds a summary for each scanner that ran.
	Scans []ScanSummary
	// Errors contains fatal error messages (pipeline was blocked).
	Errors []string
	// Warnings contains non-fatal messages (license warnings, etc.).
	Warnings []string
}

// ScanSummary holds a human-readable summary for one scanner.
type ScanSummary struct {
	// Scanner is the name of the tool (e.g. "grype", "scancode").
	Scanner string
	// Passed is false when the scan result triggered a blocking policy.
	Passed bool
	// Details is a one-line human-readable summary.
	Details string
}

// GrypeOutput is the top-level structure of a grype --output json file.
type GrypeOutput struct {
	Matches []GrypeMatch `json:"matches"`
}

// GrypeMatch represents one vulnerability match.
type GrypeMatch struct {
	Vulnerability GrypeVulnerability `json:"vulnerability"`
}

// GrypeVulnerability holds the severity of a matched CVE.
type GrypeVulnerability struct {
	Severity string `json:"severity"`
}

// FormatPRComment returns a Markdown-formatted GitHub PR comment body.
func (r *Result) FormatPRComment(name, version string) string {
	var sb strings.Builder

	status := "✅ Passed"
	if !r.Success {
		status = "❌ Failed"
	}

	fmt.Fprintf(&sb, "## Bark Internal Pipeline — %s@%s\n\n", name, version)
	fmt.Fprintf(&sb, "**Status:** %s\n\n", status)

	if len(r.Scans) > 0 {
		sb.WriteString("### Scan Results\n\n")

		for _, s := range r.Scans {
			icon := "✅"
			if !s.Passed {
				icon = "❌"
			}

			fmt.Fprintf(&sb, "- %s **%s**: %s\n", icon, s.Scanner, s.Details)
		}

		sb.WriteString("\n")
	}

	if len(r.Warnings) > 0 {
		sb.WriteString("### Warnings\n\n")

		for _, w := range r.Warnings {
			fmt.Fprintf(&sb, "- ⚠️ %s\n", w)
		}

		sb.WriteString("\n")
	}

	if len(r.Errors) > 0 {
		sb.WriteString("### Errors\n\n")

		for _, e := range r.Errors {
			fmt.Fprintf(&sb, "- ❌ %s\n", e)
		}

		sb.WriteString("\n")
	}

	if r.RegistrationID != "" {
		fmt.Fprintf(&sb, "_Registration ID: %s_\n", r.RegistrationID)
	}

	return sb.String()
}
