package manifest

import "fmt"

// S3KeyBottle returns the S3 key for a bottle tarball.
//
// Pattern: bottles/{tier}/{name}/{version}/{name}-{version}.arm64_sonoma.bottle.tar.gz.
func S3KeyBottle(tier Tier, name, version string) string {
	return fmt.Sprintf(
		"bottles/%s/%s/%s/%s-%s.arm64_sonoma.bottle.tar.gz",
		tier, name, version, name, version,
	)
}

// S3KeySBOM returns the S3 key for a CycloneDX SBOM document.
//
// Pattern: sboms/{name}/{version}/sbom.cdx.json.
func S3KeySBOM(name, version string) string {
	return fmt.Sprintf("sboms/%s/%s/sbom.cdx.json", name, version)
}

// S3KeyVulnScan returns the S3 key for a raw vulnerability scan result.
// The scanner parameter is the canonical name of the tool (e.g. "grype", "wiz").
//
// Pattern: scans/{name}/{version}/{scanner}.json.
func S3KeyVulnScan(name, version, scanner string) string {
	return fmt.Sprintf("scans/%s/%s/%s.json", name, version, scanner)
}

// S3KeyLicenseScan returns the S3 key for a raw ScanCode license scan result.
//
// Pattern: scans/{name}/{version}/scancode.json.
func S3KeyLicenseScan(name, version string) string {
	return fmt.Sprintf("scans/%s/%s/scancode.json", name, version)
}
