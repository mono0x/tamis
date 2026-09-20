package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegressionNonrecursive(t *testing.T) {
	d := t.TempDir()
	if err := os.Mkdir(filepath.Join(d, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "nested", "x"), []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Walk(Source{Path: d})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v err %v", got, err)
	}
}
func TestRegressionSingleFile(t *testing.T) {
	s := filepath.Join(t.TempDir(), "x")
	if err := os.WriteFile(s, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Walk(Source{Path: s})
	if err != nil || len(got) != 1 {
		t.Fatalf("count %d err %v", len(got), err)
	}
}
