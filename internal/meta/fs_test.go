package meta

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFSProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.jpg")
	if err := os.WriteFile(path, []byte{0xff, 0xd8, 0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := extractFixture(FSProvider{}, path)
	if err != nil {
		t.Fatal(err)
	}
	if r.Format != FormatJPEG || r.Captured == nil {
		t.Fatalf("unexpected record: %#v", r)
	}
}
