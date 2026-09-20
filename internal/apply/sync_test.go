package apply

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCopyReportsDirectorySyncFailureAndKeepsSource(t *testing.T) {
	for _, mode := range []string{"native", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			clone := cloneFile
			if mode == "fallback" {
				clone = func(*os.File, string) error { return syscall.ENOTSUP }
			}
			dir := t.TempDir()
			source, dest := filepath.Join(dir, "source"), filepath.Join(dir, "dest")
			if err := os.WriteFile(source, []byte("synthetic"), 0o600); err != nil {
				t.Fatal(err)
			}
			err := copyFileWith(source, dest, clone, func(parent string) error {
				if parent != dir {
					t.Fatalf("wrong sync target: %s", parent)
				}
				assertContents(t, dest, "synthetic")
				return syscall.EIO
			})
			if !errors.Is(err, syscall.EIO) {
				t.Fatalf("sync failure was hidden from cross-volume move: %v", err)
			}
			assertContents(t, source, "synthetic")
			assertContents(t, dest, "synthetic")
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 2 {
				t.Fatalf("temporary file leaked: %v %v", entries, err)
			}
		})
	}
}
