package scan

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTree creates each relative name under root as a synthetic file.
func writeTree(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, name := range names {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("synthetic"), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMatcherAgreesWithWalk(t *testing.T) {
	root := t.TempDir()
	names := []string{"sample.jpg", "sample.png", "nested/sample.jpg", "private/sample.jpg", "nested/private/sample.jpg"}
	writeTree(t, root, names...)
	for _, source := range []Source{
		{Path: root},
		{Path: root, Recursive: true, Include: []string{"**/*.jpg"}, Exclude: []string{"**/private"}},
		{Path: root, Recursive: true, Include: []string{"nested/*.jpg"}},
		{Path: root, Recursive: true, Exclude: []string{"nested/**"}},
		{Path: filepath.Join(root, "sample.jpg"), Include: []string{"*.jpg"}},
	} {
		walked, err := Walk(source)
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]string{}
		for _, candidate := range walked {
			found[candidate.Path] = candidate.RelPath
		}
		matcher := NewMatcher(source)
		for _, name := range append(names, "../outside.jpg") {
			path := filepath.Join(root, name)
			rel, match := matcher.Match(path)
			want, ok := found[path]
			if match != ok || (match && rel != want) {
				t.Fatalf("source=%+v path=%s: match=%v rel=%q, walked=%q", source, path, match, rel, want)
			}
		}
	}
}
