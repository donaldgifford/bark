package manifest_test

import (
	"testing"

	"github.com/donaldgifford/bark/pkg/manifest"
)

func TestS3KeyBottle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tier    manifest.Tier
		pkgName string
		version string
		want    string
	}{
		{
			name:    "internal tier",
			tier:    manifest.TierInternal,
			pkgName: "my-tool",
			version: "1.0.0",
			want:    "bottles/internal/my-tool/1.0.0/my-tool-1.0.0.arm64_sonoma.bottle.tar.gz",
		},
		{
			name:    "external-binary tier",
			tier:    manifest.TierExternalBinary,
			pkgName: "third-party",
			version: "2.3.1",
			want:    "bottles/external-binary/third-party/2.3.1/third-party-2.3.1.arm64_sonoma.bottle.tar.gz",
		},
		{
			name:    "external-built tier",
			tier:    manifest.TierExternalBuilt,
			pkgName: "jq",
			version: "1.7.1",
			want:    "bottles/external-built/jq/1.7.1/jq-1.7.1.arm64_sonoma.bottle.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := manifest.S3KeyBottle(tt.tier, tt.pkgName, tt.version)
			if got != tt.want {
				t.Errorf("S3KeyBottle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestS3KeySBOM(t *testing.T) {
	t.Parallel()

	got := manifest.S3KeySBOM("my-tool", "1.0.0")
	want := "sboms/my-tool/1.0.0/sbom.cdx.json"
	if got != want {
		t.Errorf("S3KeySBOM() = %q, want %q", got, want)
	}
}

func TestS3KeyVulnScan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		scanner string
		want    string
	}{
		{name: "grype", scanner: "grype", want: "scans/my-tool/1.0.0/grype.json"},
		{name: "wiz", scanner: "wiz", want: "scans/my-tool/1.0.0/wiz.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := manifest.S3KeyVulnScan("my-tool", "1.0.0", tt.scanner)
			if got != tt.want {
				t.Errorf("S3KeyVulnScan() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestS3KeyLicenseScan(t *testing.T) {
	t.Parallel()

	got := manifest.S3KeyLicenseScan("my-tool", "1.0.0")
	want := "scans/my-tool/1.0.0/scancode.json"
	if got != want {
		t.Errorf("S3KeyLicenseScan() = %q, want %q", got, want)
	}
}
