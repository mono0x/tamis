//go:build darwin

package apply

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCopyPreservesXattrs(t *testing.T) {
	for _, mode := range []string{"native", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			clone := cloneFile
			if mode == "fallback" {
				clone = func(*os.File, string) error { return unix.ENOTSUP }
			}
			dir := t.TempDir()
			source, dest := filepath.Join(dir, "source"), filepath.Join(dir, "dest")
			if err := os.WriteFile(source, []byte("synthetic"), 0o600); err != nil {
				t.Fatal(err)
			}
			attributes := map[string]string{"com.tamis.synthetic": "meaningless", "com.tamis.empty": ""}
			for name, value := range attributes {
				if err := unix.Setxattr(source, name, []byte(value), 0); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(source, 0o400); err != nil {
				t.Fatal(err)
			}
			if err := copyFileWith(source, dest, clone, syncDir); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{source, dest} {
				for name, want := range attributes {
					value := make([]byte, 64)
					n, err := unix.Getxattr(path, name, value)
					if err != nil {
						t.Fatal(err)
					}
					if string(value[:n]) != want {
						t.Fatalf("%s %s: value=%q err=%v", path, name, value, err)
					}
				}
			}
		})
	}
}
