package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHiddenPathsAreAlwaysExcluded(t *testing.T) {
	root := t.TempDir()
	hidden := []string{".photo.jpg", ".git/photo.jpg", "nested/.photo.jpg", "nested/.private/photo.jpg"}
	writeTree(t, root, hidden...)
	blocked := filepath.Join(root, ".blocked")
	if err := os.Mkdir(blocked, 0000); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chmod(blocked, 0700); err != nil {
			t.Error(err)
		}
	}()
	for _, source := range []Source{
		{Path: root, Recursive: true},
		{Path: root, Recursive: true, Include: []string{"**/*.jpg", ".blocked/**"}},
		{Path: filepath.Join(root, ".git"), Recursive: true},
		{Path: filepath.Join(root, ".git/photo.jpg")},
		{Path: filepath.Join(root, ".photo.jpg")},
		{Path: blocked, Recursive: true},
	} {
		got, err := Walk(source)
		if err != nil || len(got) != 0 {
			t.Fatalf("source=%+v: files=%v error=%v", source, got, err)
		}
		matcher := NewMatcher(source)
		for _, name := range hidden {
			if _, ok := matcher.Match(filepath.Join(root, name)); ok {
				t.Fatalf("matched hidden path %s with %+v", name, source)
			}
		}
	}
}
