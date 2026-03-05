package store

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeTestTarGz writes a gzipped tar archive to a temp file.
// files maps relative path → file content. Returns the temp file path.
func makeTestTarGz(t *testing.T, files map[string]string) string {
	t.Helper()

	tmp, err := os.CreateTemp(t.TempDir(), "test-bottle-*.tar.gz")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	gw := gzip.NewWriter(tmp)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Size:     int64(len(content)),
			Mode:     0o644,
		}

		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header: %v", err)
		}

		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write tar entry: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}

	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	if err := tmp.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	return tmp.Name()
}

// sha256OfFile returns the hex-encoded SHA-256 digest of a file's contents.
func sha256OfFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file for sha256: %v", err)
	}

	h := sha256.Sum256(data)

	return hex.EncodeToString(h[:])
}

func TestStoreHas(t *testing.T) {
	t.Parallel()

	t.Run("absent returns false", func(t *testing.T) {
		t.Parallel()

		st, err := New(t.TempDir())
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		if st.Has("deadbeef") {
			t.Error("expected Has to return false for absent entry")
		}
	})

	t.Run("present returns true", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		st, err := New(dir)
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		digest := "abc123"

		if err := os.WriteFile(filepath.Join(dir, digest+".tar.gz"), []byte("data"), 0o600); err != nil {
			t.Fatalf("seed store file: %v", err)
		}

		if !st.Has(digest) {
			t.Error("expected Has to return true for present entry")
		}
	})
}

func TestStorePut(t *testing.T) {
	t.Parallel()

	t.Run("writes file at correct path", func(t *testing.T) {
		t.Parallel()

		storeDir := t.TempDir()

		st, err := New(storeDir)
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		src := makeTestTarGz(t, map[string]string{"hello.txt": "hello"})
		digest := sha256OfFile(t, src)

		if err := st.Put(digest, src); err != nil {
			t.Fatalf("Put: %v", err)
		}

		dest := filepath.Join(storeDir, digest+".tar.gz")
		if _, err := os.Stat(dest); err != nil {
			t.Errorf("expected store entry at %s: %v", dest, err)
		}
	})

	t.Run("idempotent - second Put leaves original unchanged", func(t *testing.T) {
		t.Parallel()

		storeDir := t.TempDir()

		st, err := New(storeDir)
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		src := makeTestTarGz(t, map[string]string{"a.txt": "original"})
		digest := sha256OfFile(t, src)

		if err := st.Put(digest, src); err != nil {
			t.Fatalf("first Put: %v", err)
		}

		// Modify source so a second real copy would differ.
		src2 := makeTestTarGz(t, map[string]string{"a.txt": "modified"})

		if err := st.Put(digest, src2); err != nil {
			t.Fatalf("second Put: %v", err)
		}

		// The stored file should still match the first Put.
		stored := filepath.Join(storeDir, digest+".tar.gz")

		storedData, err := os.ReadFile(stored)
		if err != nil {
			t.Fatalf("read stored: %v", err)
		}

		originalData, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read original: %v", err)
		}

		if !bytes.Equal(storedData, originalData) {
			t.Error("second Put overwrote existing store entry")
		}
	})

	t.Run("source file missing returns error", func(t *testing.T) {
		t.Parallel()

		st, err := New(t.TempDir())
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		if err := st.Put("deadbeef", "/nonexistent/path/bottle.tar.gz"); err == nil {
			t.Error("expected error for missing source file, got nil")
		}
	})
}

func TestStoreGet(t *testing.T) {
	t.Parallel()

	t.Run("absent returns ErrNotCached", func(t *testing.T) {
		t.Parallel()

		st, err := New(t.TempDir())
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		_, err = st.Get("nonexistent")
		if !errors.Is(err, ErrNotCached) {
			t.Errorf("expected ErrNotCached, got %v", err)
		}
	})

	t.Run("present returns correct path", func(t *testing.T) {
		t.Parallel()

		storeDir := t.TempDir()

		st, err := New(storeDir)
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		digest := "feedcafe"

		expected := filepath.Join(storeDir, digest+".tar.gz")
		if err := os.WriteFile(expected, []byte("data"), 0o600); err != nil {
			t.Fatalf("seed store: %v", err)
		}

		got, err := st.Get(digest)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}

		if got != expected {
			t.Errorf("Get path = %q, want %q", got, expected)
		}
	})
}

func TestStoreExtract(t *testing.T) {
	t.Parallel()

	t.Run("extracts regular file to dest dir", func(t *testing.T) {
		t.Parallel()

		storeDir := t.TempDir()

		st, err := New(storeDir)
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		const content = "hello from bottle"

		src := makeTestTarGz(t, map[string]string{"bin/greet": content})
		digest := sha256OfFile(t, src)

		if err := st.Put(digest, src); err != nil {
			t.Fatalf("Put: %v", err)
		}

		destDir := t.TempDir()

		if err := st.Extract(digest, destDir); err != nil {
			t.Fatalf("Extract: %v", err)
		}

		gotBytes, err := os.ReadFile(filepath.Join(destDir, "bin", "greet"))
		if err != nil {
			t.Fatalf("read extracted file: %v", err)
		}

		if string(gotBytes) != content {
			t.Errorf("extracted content = %q, want %q", string(gotBytes), content)
		}
	})

	t.Run("absent digest returns ErrNotCached", func(t *testing.T) {
		t.Parallel()

		st, err := New(t.TempDir())
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		if err := st.Extract("nonexistent", t.TempDir()); !errors.Is(err, ErrNotCached) {
			t.Errorf("expected ErrNotCached, got %v", err)
		}
	})
}

func TestExtractEntryPathTraversal(t *testing.T) {
	t.Parallel()

	// Build a tar.gz with a path traversal entry.
	tmp, err := os.CreateTemp(t.TempDir(), "traversal-*.tar.gz")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}

	gw := gzip.NewWriter(tmp)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name:     "../evil.txt",
		Typeflag: tar.TypeReg,
		Size:     5,
		Mode:     0o644,
	}

	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write header: %v", err)
	}

	if _, err := tw.Write([]byte("pwned")); err != nil {
		t.Fatalf("write data: %v", err)
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close tw: %v", err)
	}

	if err := gw.Close(); err != nil {
		t.Fatalf("close gw: %v", err)
	}

	if err := tmp.Close(); err != nil {
		t.Fatalf("close tmp: %v", err)
	}

	err = extractTarGz(tmp.Name(), t.TempDir())
	if err == nil {
		t.Fatal("expected error for path traversal entry, got nil")
	}

	if !strings.Contains(err.Error(), "unsafe path") {
		t.Errorf("expected 'unsafe path' error, got: %v", err)
	}
}

func TestErrNotCached(t *testing.T) {
	t.Parallel()

	if ErrNotCached.Error() != "bottle not in local cache" {
		t.Errorf("ErrNotCached message = %q", ErrNotCached.Error())
	}
}
