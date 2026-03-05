// Package store implements a content-addressable local cache for bottle tarballs.
// Each entry is keyed by the bottle's hex-encoded SHA-256 digest.
package store

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Store is a content-addressable cache for downloaded bottle tarballs.
// It stores the raw tarball at ~/.pkgtool/store/{sha256}.tar.gz.
type Store struct {
	basePath string
}

// New creates a Store rooted at basePath, creating the directory if necessary.
func New(basePath string) (*Store, error) {
	if err := os.MkdirAll(basePath, 0o750); err != nil {
		return nil, fmt.Errorf("create store directory: %w", err)
	}

	return &Store{basePath: basePath}, nil
}

// Has returns true if a bottle with the given SHA-256 digest is in the store.
func (s *Store) Has(sha256 string) bool {
	_, err := os.Stat(s.path(sha256))
	return err == nil
}

// Put copies the tarball at bottlePath into the store under sha256.
// If an entry already exists it is left unchanged and no error is returned.
func (s *Store) Put(sha256, bottlePath string) error {
	dest := s.path(sha256)
	if _, err := os.Stat(dest); err == nil {
		return nil // already cached
	}

	src, err := os.Open(bottlePath)
	if err != nil {
		return fmt.Errorf("open bottle: %w", err)
	}
	defer src.Close() //nolint:errcheck // read-only; close error not actionable

	// Write to a temp file then rename for atomicity.
	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()    //nolint:errcheck // cleanup; original error takes precedence
		_ = os.Remove(tmp) //nolint:errcheck // cleanup; original error takes precedence
		return fmt.Errorf("copy bottle to store: %w", err)
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(tmp) //nolint:errcheck // cleanup; original error takes precedence
		return fmt.Errorf("close store file: %w", err)
	}

	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp) //nolint:errcheck // cleanup; original error takes precedence
		return fmt.Errorf("commit bottle to store: %w", err)
	}

	return nil
}

// Get returns the filesystem path to the stored tarball for sha256.
// Returns ErrNotCached if the entry does not exist.
func (s *Store) Get(sha256 string) (string, error) {
	p := s.path(sha256)
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNotCached
		}

		return "", fmt.Errorf("stat store entry: %w", err)
	}

	return p, nil
}

// Extract unpacks the tarball for sha256 into destDir.
// All top-level paths from the tarball are written under destDir.
func (s *Store) Extract(sha256, destDir string) error {
	tarPath, err := s.Get(sha256)
	if err != nil {
		return err
	}

	return extractTarGz(tarPath, destDir)
}

// path returns the full filesystem path for a store entry.
func (s *Store) path(sha256 string) string {
	return filepath.Join(s.basePath, sha256+".tar.gz")
}

// =============================================================================
// tar.gz extraction
// =============================================================================

// extractTarGz extracts the gzipped tar archive at src into destDir.
// It rejects path traversal (../) in archive entries.
func extractTarGz(src, destDir string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only; close error not actionable

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer gz.Close() //nolint:errcheck // reader close error not actionable

	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		if err := extractEntry(tr, hdr, destDir); err != nil {
			return err
		}
	}

	return nil
}

// extractEntry writes a single tar entry to destDir.
func extractEntry(tr *tar.Reader, hdr *tar.Header, destDir string) error {
	// Reject path traversal.
	if strings.Contains(hdr.Name, "..") {
		return fmt.Errorf("unsafe path in archive: %q", hdr.Name)
	}

	target := filepath.Join(destDir, hdr.Name) //nolint:gosec // traversal checked above

	switch hdr.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(target, 0o750); err != nil {
			return fmt.Errorf("create directory %s: %w", target, err)
		}

	case tar.TypeReg:
		if err := writeRegularFile(tr, hdr, target); err != nil {
			return err
		}

	case tar.TypeSymlink:
		if err := os.Symlink(hdr.Linkname, target); err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("create symlink %s: %w", target, err)
		}
	}

	return nil
}

// writeRegularFile writes a regular file tar entry to target.
func writeRegularFile(tr *tar.Reader, hdr *tar.Header, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
	if err != nil {
		return fmt.Errorf("create file %s: %w", target, err)
	}

	if _, err := io.Copy(out, tr); err != nil {
		_ = out.Close() //nolint:errcheck // cleanup; original error takes precedence
		return fmt.Errorf("write file %s: %w", target, err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("close file %s: %w", target, err)
	}

	return nil
}

// ErrNotCached is returned by Get when the requested entry is not in the store.
var ErrNotCached = errors.New("bottle not in local cache")
