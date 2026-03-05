package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLatestInstalledVersion(t *testing.T) {
	t.Parallel()

	t.Run("returns last alphabetical version", func(t *testing.T) {
		t.Parallel()

		base := t.TempDir()

		for _, v := range []string{"1.0.0", "1.1.0", "2.0.0"} {
			if err := os.MkdirAll(filepath.Join(base, v), 0o750); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
		}

		got, err := latestInstalledVersion(base)
		if err != nil {
			t.Fatalf("latestInstalledVersion: %v", err)
		}

		if got != "2.0.0" {
			t.Errorf("got %q, want %q", got, "2.0.0")
		}
	})

	t.Run("single version returns it", func(t *testing.T) {
		t.Parallel()

		base := t.TempDir()

		if err := os.MkdirAll(filepath.Join(base, "1.0.0"), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		got, err := latestInstalledVersion(base)
		if err != nil {
			t.Fatalf("latestInstalledVersion: %v", err)
		}

		if got != "1.0.0" {
			t.Errorf("got %q, want %q", got, "1.0.0")
		}
	})

	t.Run("empty dir returns not-installed error", func(t *testing.T) {
		t.Parallel()

		_, err := latestInstalledVersion(t.TempDir())
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !strings.Contains(err.Error(), "not installed") {
			t.Errorf("expected 'not installed' in error, got: %v", err)
		}
	})

	t.Run("nonexistent dir returns not-installed error", func(t *testing.T) {
		t.Parallel()

		_, err := latestInstalledVersion("/nonexistent/cellar/base")
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !strings.Contains(err.Error(), "not installed") {
			t.Errorf("expected 'not installed' in error, got: %v", err)
		}
	})
}

func TestLinkBinaries(t *testing.T) {
	t.Parallel()

	t.Run("creates symlinks for each file in cellarDir/bin/", func(t *testing.T) {
		t.Parallel()

		cellarDir := t.TempDir()
		binDir := t.TempDir()

		srcBin := filepath.Join(cellarDir, "bin")
		if err := os.MkdirAll(srcBin, 0o750); err != nil {
			t.Fatalf("mkdir bin: %v", err)
		}

		for _, name := range []string{"tool-a", "tool-b"} {
			if err := os.WriteFile(filepath.Join(srcBin, name), []byte("#!/bin/sh"), 0o755); err != nil {
				t.Fatalf("write binary: %v", err)
			}
		}

		if err := linkBinaries(cellarDir, binDir); err != nil {
			t.Fatalf("linkBinaries: %v", err)
		}

		for _, name := range []string{"tool-a", "tool-b"} {
			link := filepath.Join(binDir, name)

			info, err := os.Lstat(link)
			if err != nil {
				t.Errorf("symlink %s missing: %v", name, err)
				continue
			}

			if info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("%s is not a symlink", name)
			}
		}
	})

	t.Run("no error when cellarDir/bin/ does not exist", func(t *testing.T) {
		t.Parallel()

		if err := linkBinaries(t.TempDir(), t.TempDir()); err != nil {
			t.Errorf("expected no error for missing bin/, got: %v", err)
		}
	})
}

func TestRemoveSymlinksTo(t *testing.T) {
	t.Parallel()

	t.Run("removes symlinks pointing into targetDir", func(t *testing.T) {
		t.Parallel()

		targetDir := t.TempDir()
		binDir := t.TempDir()

		// Create a file inside targetDir.
		target := filepath.Join(targetDir, "mytool")
		if err := os.WriteFile(target, []byte("binary"), 0o755); err != nil {
			t.Fatalf("write target: %v", err)
		}

		// Symlink from binDir into targetDir.
		link := filepath.Join(binDir, "mytool")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}

		if err := removeSymlinksTo(binDir, targetDir); err != nil {
			t.Fatalf("removeSymlinksTo: %v", err)
		}

		if _, err := os.Lstat(link); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("expected symlink to be removed, Lstat err = %v", err)
		}
	})

	t.Run("leaves symlinks pointing elsewhere untouched", func(t *testing.T) {
		t.Parallel()

		targetDir := t.TempDir()
		otherDir := t.TempDir()
		binDir := t.TempDir()

		// Create a file in otherDir (not targetDir).
		otherTarget := filepath.Join(otherDir, "othertool")
		if err := os.WriteFile(otherTarget, []byte("binary"), 0o755); err != nil {
			t.Fatalf("write other target: %v", err)
		}

		// Symlink into otherDir (should be preserved).
		link := filepath.Join(binDir, "othertool")
		if err := os.Symlink(otherTarget, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}

		if err := removeSymlinksTo(binDir, targetDir); err != nil {
			t.Fatalf("removeSymlinksTo: %v", err)
		}

		if _, err := os.Lstat(link); err != nil {
			t.Errorf("expected symlink to be preserved, Lstat err = %v", err)
		}
	})

	t.Run("no error when dir does not exist", func(t *testing.T) {
		t.Parallel()

		if err := removeSymlinksTo("/nonexistent/bin", "/nonexistent/target"); err != nil {
			t.Errorf("expected no error for nonexistent dir, got: %v", err)
		}
	})
}
