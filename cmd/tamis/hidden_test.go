package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHiddenFilesSurviveApplyWithoutExcludes(t *testing.T) {
	config, source, _ := commandFixture(t, "trash", "fail")
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	contents := strings.Replace(string(data), "    when: 'kind == \"image\"'", "    recursive: true\n    when: 'true'", 1)
	writeFile(t, config, []byte(contents))
	root := filepath.Dir(source)
	hidden := []string{".DS_Store", ".private/sample.txt", "nested/.sample.txt"}
	visible := []string{"thumbs.db", "desktop.ini", "~$sample.txt", "@eaDir/sample.txt"}
	for _, name := range append(hidden, visible...) {
		writeFile(t, filepath.Join(root, name), []byte("synthetic"))
	}
	for _, args := range [][]string{{"scan", root}, {"scan", "--config", config}} {
		output, _, err := executeCommand("", args...)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range hidden {
			if strings.Contains(output, filepath.Join(root, name)) {
				t.Fatalf("scan included hidden path %s", name)
			}
		}
		for _, name := range visible {
			if !strings.Contains(output, filepath.Join(root, name)) {
				t.Fatalf("scan excluded visible path %s", name)
			}
		}
	}
	for _, name := range hidden {
		if _, _, err := executeCommand("", "explain", config, filepath.Join(root, name)); err == nil {
			t.Fatalf("explain accepted hidden path %s", name)
		}
	}
	if _, _, err := executeCommand("", "apply", config, "--trash-dir", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	for _, name := range hidden {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("hidden file changed: %s, %v", name, err)
		}
	}
	for _, name := range append(visible, filepath.Base(source)) {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("visible file not trashed: %s, %v", name, err)
		}
	}
}
