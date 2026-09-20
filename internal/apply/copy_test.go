package apply

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestCopyFallback(t *testing.T) {
	for _, cloneErr := range []error{syscall.ENOTSUP, syscall.EXDEV} {
		t.Run(cloneErr.Error(), func(t *testing.T) {
			testCopy(t, func(source *os.File, dest string) error { return cloneErr })
		})
	}
}

func testCopy(t *testing.T, clone func(*os.File, string) error) {
	t.Helper()
	dir := t.TempDir()
	source, dest := filepath.Join(dir, "source"), filepath.Join(dir, "dest")
	if err := os.WriteFile(source, []byte("synthetic"), 0o400); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(source, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := copyFileWith(source, dest, clone, syncDir); err != nil {
		t.Fatal(err)
	}
	src, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	dst, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(src, dst) || src.Mode() != dst.Mode() || !src.ModTime().Equal(dst.ModTime()) {
		t.Fatalf("copy did not preserve independent contents, mode and mtime: %v %v", src, dst)
	}
	assertContents(t, dest, "synthetic")
	if err := os.Chmod(dest, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertContents(t, source, "synthetic")
	assertNoCopyTemps(t, dir, 2)
}

func TestCopyStopsOnCloneIOFailure(t *testing.T) {
	for _, cloneErr := range []error{syscall.EIO, syscall.ENOSPC, syscall.EACCES} {
		t.Run(cloneErr.Error(), func(t *testing.T) {
			dir := t.TempDir()
			source, dest := filepath.Join(dir, "source"), filepath.Join(dir, "dest")
			if err := os.WriteFile(source, []byte("synthetic"), 0o600); err != nil {
				t.Fatal(err)
			}
			err := copyFileWith(source, dest, func(*os.File, string) error { return cloneErr }, syncDir)
			if !errors.Is(err, cloneErr) {
				t.Fatalf("clone failure hidden: %v", err)
			}
			assertContents(t, source, "synthetic")
			if _, err := os.Stat(dest); !os.IsNotExist(err) {
				t.Fatalf("destination created after clone failure: %v", err)
			}
			assertNoCopyTemps(t, dir, 1)
		})
	}
}

func assertNoCopyTemps(t *testing.T, dir string, count int) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != count {
		t.Fatalf("temporary files leaked: %v %v", entries, err)
	}
}
