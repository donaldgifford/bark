// Package policy implements license policy evaluation for the bark pipeline.
// It loads a YAML policy file and evaluates ScanCode output against the
// allow/warn/deny lists defined in that file.
package policy

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

// Config represents the contents of license-policy.yaml.
type Config struct {
	// AllowedSPDX lists SPDX identifiers that are unconditionally permitted.
	AllowedSPDX []string `json:"allowed_spdx"`
	// WarnSPDX lists SPDX identifiers that pass but trigger a warning.
	WarnSPDX []string `json:"warn_spdx"`
	// DeniedSPDX lists SPDX identifiers that block the pipeline.
	DeniedSPDX []string `json:"denied_spdx"`
}

// LoadConfig reads and parses a license-policy.yaml file at policyPath.
func LoadConfig(policyPath string) (*Config, error) {
	data, err := os.ReadFile(policyPath)
	if err != nil {
		return nil, fmt.Errorf("read policy file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse policy file: %w", err)
	}

	return &cfg, nil
}

// warnSet and deniedSet return maps for fast membership testing in Evaluator.
func (c *Config) warnSet() map[string]struct{}   { return toSet(c.WarnSPDX) }
func (c *Config) deniedSet() map[string]struct{} { return toSet(c.DeniedSPDX) }

func toSet(ids []string) map[string]struct{} {
	s := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		s[id] = struct{}{}
	}

	return s
}
