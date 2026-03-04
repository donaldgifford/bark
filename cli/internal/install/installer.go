// Package install orchestrates the full package installation flow.
package install

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/donaldgifford/bark/cli/internal/apiclient"
	"github.com/donaldgifford/bark/cli/internal/store"
	"github.com/donaldgifford/bark/cli/internal/verifier"
	"github.com/donaldgifford/bark/pkg/types"
)

// Installer orchestrates the resolve → verify → download → cache → install flow.
type Installer struct {
	api        *apiclient.Client
	store      *store.Store
	verifier   *verifier.Verifier
	prefixPath string
}

// New creates an Installer.
// prefixPath is the root under which Cellar/ and bin/ directories live.
func New(api *apiclient.Client, st *store.Store, v *verifier.Verifier, prefixPath string) *Installer {
	return &Installer{
		api:        api,
		store:      st,
		verifier:   v,
		prefixPath: prefixPath,
	}
}

// Install fetches, verifies, caches, and installs the named package.
// It prints progress to stdout. The version parameter is optional; if empty,
// the latest approved version is installed.
func (i *Installer) Install(ctx context.Context, name, version string) error {
	fmt.Printf("==> Resolving %s", name)
	if version != "" {
		fmt.Printf("@%s", version)
	}
	fmt.Println("...")

	var (
		manifest *manifestInfo
		err      error
	)

	if version == "" {
		manifest, err = i.resolve(ctx, name, "")
	} else {
		manifest, err = i.resolve(ctx, name, version)
	}

	if err != nil {
		if errors.Is(err, apiclient.ErrNotFound) {
			return fmt.Errorf("package %q not found; try 'pkgtool search %s'", name, name)
		}

		return fmt.Errorf("resolve %s: %w", name, err)
	}

	// Determine whether we already have the bottle in the store.
	_, cacheErr := i.store.Get(manifest.sha256)
	needDownload := errors.Is(cacheErr, store.ErrNotCached)

	var bottlePath string

	if needDownload {
		fmt.Printf("==> Downloading %s@%s...\n", manifest.name, manifest.version)

		bottlePath, err = i.download(ctx, manifest.presignedURL, manifest.sha256)
		if err != nil {
			return fmt.Errorf("download %s: %w", name, err)
		}

		defer os.Remove(bottlePath) //nolint:errcheck // temp file cleanup
	} else {
		fmt.Printf("==> Using cached bottle for %s@%s\n", manifest.name, manifest.version)

		bottlePath, _ = i.store.Get(manifest.sha256) //nolint:errcheck // checked above
	}

	fmt.Printf("==> Verifying signature for %s@%s...\n", manifest.name, manifest.version)

	if err := i.verifier.Verify(ctx, bottlePath, manifest.cosignSigRef, manifest.publicKey); err != nil {
		return fmt.Errorf("signature verification failed for %s — aborting: %w", name, err)
	}

	if needDownload {
		if err := i.store.Put(manifest.sha256, bottlePath); err != nil {
			return fmt.Errorf("cache bottle: %w", err)
		}
	}

	cellarDir := filepath.Join(i.prefixPath, "Cellar", manifest.name, manifest.version)

	fmt.Printf("==> Installing %s@%s...\n", manifest.name, manifest.version)

	if err := i.store.Extract(manifest.sha256, cellarDir); err != nil {
		return fmt.Errorf("extract bottle: %w", err)
	}

	fmt.Println("==> Linking binaries...")

	if err := linkBinaries(cellarDir, filepath.Join(i.prefixPath, "bin")); err != nil {
		return fmt.Errorf("link binaries: %w", err)
	}

	fmt.Printf("✓ %s@%s installed\n", manifest.name, manifest.version)

	return nil
}

// Uninstall removes the installed package from the prefix.
// It removes the Cellar directory and any symlinks pointing into it.
func (i *Installer) Uninstall(name, version string) error {
	cellarBase := filepath.Join(i.prefixPath, "Cellar", name)

	if version == "" {
		var err error
		if version, err = latestInstalledVersion(cellarBase); err != nil {
			return err
		}
	}

	cellarDir := filepath.Join(cellarBase, version)
	binDir := filepath.Join(i.prefixPath, "bin")

	if err := removeSymlinksTo(binDir, cellarDir); err != nil {
		return fmt.Errorf("remove symlinks: %w", err)
	}

	if err := os.RemoveAll(cellarDir); err != nil {
		return fmt.Errorf("remove cellar directory: %w", err)
	}

	fmt.Printf("✓ %s@%s uninstalled\n", name, version)

	return nil
}

// InstalledPackage is a summary of an installed package.
type InstalledPackage struct {
	Name    string
	Version string
}

// List returns all installed packages.
func (i *Installer) List() ([]InstalledPackage, error) {
	cellarBase := filepath.Join(i.prefixPath, "Cellar")

	entries, err := os.ReadDir(cellarBase)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("read cellar: %w", err)
	}

	var pkgs []InstalledPackage

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		versions, err := os.ReadDir(filepath.Join(cellarBase, e.Name()))
		if err != nil {
			continue
		}

		for _, v := range versions {
			if v.IsDir() {
				pkgs = append(pkgs, InstalledPackage{Name: e.Name(), Version: v.Name()})
			}
		}
	}

	return pkgs, nil
}

// =============================================================================
// internal helpers
// =============================================================================

// manifestInfo bundles resolved package metadata needed for the install flow.
type manifestInfo struct {
	name         string
	version      string
	sha256       string
	presignedURL string
	cosignSigRef string
	publicKey    string
}

// resolve fetches the manifest and the current signing key from the API.
func (i *Installer) resolve(ctx context.Context, name, version string) (*manifestInfo, error) {
	var (
		resp *types.ResolveResponse
		err  error
	)

	if version == "" {
		resp, err = i.api.ResolveLatest(ctx, name)
	} else {
		resp, err = i.api.ResolveVersion(ctx, name, version)
	}

	if err != nil {
		return nil, err
	}

	keyResp, err := i.api.GetSigningKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("get signing key: %w", err)
	}

	m := resp.Manifest

	return &manifestInfo{
		name:         m.Name,
		version:      m.Version,
		sha256:       m.BottleSHA256,
		presignedURL: m.BottlePresignedURL,
		cosignSigRef: m.CosignSigRef,
		publicKey:    keyResp.PublicKey,
	}, nil
}

// download fetches presignedURL to a temp file, verifies the SHA-256 digest, and
// returns the path to the temp file. Caller must remove the file when done.
func (*Installer) download(ctx context.Context, presignedURL, expectedSHA256 string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, presignedURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("build download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "bark-bottle-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	h := sha256.New()

	if _, err = io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		_ = tmp.Close()           //nolint:errcheck // cleanup; original error takes precedence
		_ = os.Remove(tmp.Name()) //nolint:errcheck // cleanup; original error takes precedence
		return "", fmt.Errorf("write download: %w", err)
	}

	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name()) //nolint:errcheck // cleanup; original error takes precedence
		return "", fmt.Errorf("close temp file: %w", err)
	}

	got := hex.EncodeToString(h.Sum(nil))
	if got != expectedSHA256 {
		_ = os.Remove(tmp.Name()) //nolint:errcheck // cleanup
		return "", fmt.Errorf("sha256 mismatch: expected %s, got %s", expectedSHA256, got)
	}

	return tmp.Name(), nil
}

// linkBinaries creates symlinks in binDir for every file found in cellarDir/bin/.
func linkBinaries(cellarDir, binDir string) error {
	srcBin := filepath.Join(cellarDir, "bin")

	entries, err := os.ReadDir(srcBin)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // no bin/ in bottle
		}

		return fmt.Errorf("read bin directory: %w", err)
	}

	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return fmt.Errorf("create bin directory: %w", err)
	}

	for _, e := range entries {
		src := filepath.Join(srcBin, e.Name())
		dst := filepath.Join(binDir, e.Name())

		_ = os.Remove(dst) //nolint:errcheck // remove stale symlink; absence is fine

		if err := os.Symlink(src, dst); err != nil {
			return fmt.Errorf("create symlink %s: %w", dst, err)
		}
	}

	return nil
}

// removeSymlinksTo removes symlinks in dir whose resolved targets are inside targetDir.
func removeSymlinksTo(dir, targetDir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("read bin directory: %w", err)
	}

	// Resolve targetDir to its canonical path so that comparisons work even when
	// the OS uses intermediate symlinks (e.g. /var → /private/var on macOS).
	if canonical, err := filepath.EvalSymlinks(targetDir); err == nil {
		targetDir = canonical
	}

	for _, e := range entries {
		p := filepath.Join(dir, e.Name())

		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			continue
		}

		rel, err := filepath.Rel(targetDir, resolved)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}

		_ = os.Remove(p) //nolint:errcheck // best-effort symlink removal
	}

	return nil
}

// latestInstalledVersion returns the last (alphabetically highest) version
// installed under cellarBase, or an error if none is found.
func latestInstalledVersion(cellarBase string) (string, error) {
	entries, err := os.ReadDir(cellarBase)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("package at %q is not installed", cellarBase)
		}

		return "", fmt.Errorf("list installed versions: %w", err)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("package at %q is not installed", cellarBase)
	}

	return entries[len(entries)-1].Name(), nil
}
