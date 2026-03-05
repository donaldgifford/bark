// Package ingest orchestrates the full pipeline for ingesting an internal-tier bottle:
// scan (syft → SBOM, grype → CVEs, scancode → licenses), sign with cosign,
// upload artifacts to S3, and register the version with the bark API.
package ingest

import (
	"fmt"
	"strings"

	"github.com/donaldgifford/bark/api/s3"
)

// Config holds the configuration for the ingest pipeline.
type Config struct {
	// APIBaseURL is the base URL of the bark API (e.g. "https://bark.internal").
	APIBaseURL string
	// APIToken is the bearer token for pipeline → API authentication.
	APIToken string
	// S3 is the S3 client configuration used to upload artifacts.
	S3 s3.Config
	// SigningKeyPath is the path to the cosign private key file.
	SigningKeyPath string
	// PolicyPath is the path to the license-policy.yaml file.
	PolicyPath string
	// AllowedOrigins is the list of allowed bottle source URL prefixes.
	AllowedOrigins []string
}

// Validate returns an error if any required field is missing.
func (c *Config) Validate() error {
	var missing []string

	if c.APIBaseURL == "" {
		missing = append(missing, "APIBaseURL")
	}

	if c.APIToken == "" {
		missing = append(missing, "APIToken")
	}

	if c.SigningKeyPath == "" {
		missing = append(missing, "SigningKeyPath")
	}

	if c.PolicyPath == "" {
		missing = append(missing, "PolicyPath")
	}

	if len(missing) > 0 {
		return fmt.Errorf("ingest config missing required fields: %s", strings.Join(missing, ", "))
	}

	return nil
}

// Request contains the parameters for ingesting a single bottle.
type Request struct {
	// BottlePath is the absolute filesystem path to the bottle .tar.gz.
	BottlePath string
	// Name is the package name (e.g. "mytool").
	Name string
	// Version is the package version string (e.g. "1.2.3").
	Version string
}

// Validate returns an error if any required field is missing.
func (r *Request) Validate() error {
	var missing []string

	if r.BottlePath == "" {
		missing = append(missing, "BottlePath")
	}

	if r.Name == "" {
		missing = append(missing, "Name")
	}

	if r.Version == "" {
		missing = append(missing, "Version")
	}

	if len(missing) > 0 {
		return fmt.Errorf("ingest request missing required fields: %s", strings.Join(missing, ", "))
	}

	return nil
}
