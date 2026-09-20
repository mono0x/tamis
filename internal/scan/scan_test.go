package scan

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWalkMissingSource(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"missing", "missing/nested"} {
		t.Run(name, func(t *testing.T) {
			got, err := Walk(Source{Path: filepath.Join(root, name), Recursive: true})
			if err != nil || len(got) != 0 {
				t.Fatalf("candidates=%v err=%v", got, err)
			}
		})
	}
}

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{{"**/.DS_Store", ".DS_Store", true}, {"**/@eaDir/**", "a/@eaDir/thumb/x.jpg", true}, {"Untitled*/PRIVATE/M4ROOT/CLIP/*.MP4", "Untitled 1/PRIVATE/M4ROOT/CLIP/C0001.MP4", true}, {"*.jpg", "nested/a.jpg", false}}
	for _, tc := range cases {
		if got := compileGlob(tc.pattern).MatchString(tc.path); got != tc.want {
			t.Errorf("%q %q: got %v want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestWalkIncludePrunesUnrelatedDirectories(t *testing.T) {
	root := t.TempDir()
	want := "Untitled 1/PRIVATE/M4ROOT/CLIP/synthetic.MP4"
	file := filepath.Join(root, filepath.FromSlash(want))
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Archive", "Untitled 1/DCIM", "Untitled 1/PRIVATE/OTHER", "Untitled 1/PRIVATE/M4ROOT/CLIP/nested"} {
		dir := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
		if _, err := os.ReadDir(dir); err == nil {
			t.Skip("unreadable directories are accessible to this user")
		}
	}
	got, err := Walk(Source{Path: root, Recursive: true, Include: []string{"Untitled*/PRIVATE/M4ROOT/CLIP/*"}})
	if err != nil || len(got) != 1 || got[0].RelPath != want {
		t.Fatalf("candidates=%v err=%v", got, err)
	}
}

func TestWalkIncludePreservesMatches(t *testing.T) {
	root := t.TempDir()
	files := []string{"a.jpg", "album/b.jpg", "album/nested/c.jpg", "other/d.png"}
	for _, name := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("synthetic"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, patterns := range [][]string{
		nil, {"*.jpg"}, {"**/*.jpg"}, {"album/**/c.jpg"},
		{"album/*.jpg", "other/*.png"}, {"a**c.jpg"}, {"album/nested/c.jpg"},
	} {
		var want []string
		for _, name := range files {
			if len(patterns) == 0 || matchesAny(compileGlobs(patterns), name) {
				want = append(want, name)
			}
		}
		got, err := Walk(Source{Path: root, Recursive: true, Include: patterns})
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, candidate := range got {
			names = append(names, candidate.RelPath)
		}
		if !reflect.DeepEqual(names, want) {
			t.Errorf("patterns=%v got=%v want=%v", patterns, names, want)
		}
	}
}
