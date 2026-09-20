package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mono0x/tamis/internal/meta"
)

func TestCommandsAcceptMissingSource(t *testing.T) {
	config, source, dest := commandFixture(t, "copy", "fail")
	missing := filepath.Join(filepath.Dir(config), "missing", "nested")
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Replace(string(data), "rules:\n", fmt.Sprintf("rules:\n  - name: missing\n    location: %q\n", missing), 1)
	if err := os.WriteFile(config, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	emptyConfig := filepath.Join(filepath.Dir(config), "empty.yaml")
	writeFile(t, emptyConfig, fmt.Appendf(nil, "version: 1\nrules:\n  - name: missing\n    location: %q\n", missing))
	for _, args := range [][]string{
		{"scan", missing},
		{"scan", "--config", emptyConfig},
		{"preview", emptyConfig},
		{"apply", emptyConfig},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			output, _, err := executeCommand("", args...)
			if err != nil {
				t.Fatal(err)
			}
			if args[0] == "scan" && output != "" {
				t.Fatalf("expected no records, got %q", output)
			}
		})
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("empty source changed destination: %v", err)
	}
	output, _, err := executeCommand("", "scan", "--config", config)
	if err != nil {
		t.Fatal(err)
	}
	var record meta.Record
	if err := json.Unmarshal([]byte(output), &record); err != nil || record.Path != source {
		t.Fatalf("expected existing source record, got %q: %v", output, err)
	}
	if _, _, err := executeCommand("", "apply", config); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("existing source was not copied: %v", err)
	}
}

func TestScanReturnsPartialResultsButApplyRejectsIncompleteScan(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission restrictions do not apply to root")
	}
	f := newEndToEndFixture(t, "move", "rename")
	blocked := filepath.Join(f.root, "inbox", "a-blocked")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chmod(blocked, 0o700); err != nil {
			t.Error(err)
		}
	}()
	if _, err := os.ReadDir(blocked); err == nil {
		t.Skip("filesystem does not enforce directory permissions")
	}
	output, _, err := executeCommand("", "scan", "--config", f.config)
	if err == nil || !strings.Contains(err.Error(), "a-blocked") {
		t.Fatalf("scan lost error: %v", err)
	}
	decoder := json.NewDecoder(strings.NewReader(output))
	for _, name := range []string{"photo.png", "video.mp4"} {
		var record meta.Record
		if err := decoder.Decode(&record); err != nil || filepath.Base(record.Path) != name {
			t.Fatalf("scan lost readable file %s: %+v %v", name, record, err)
		}
	}
	for _, command := range []string{"preview", "apply"} {
		if _, _, err := executeCommand("y\n", command, f.config); err == nil {
			t.Fatalf("%s accepted an incomplete scan", command)
		}
	}
	if _, err := os.Stat(filepath.Join(f.root, "archive")); !os.IsNotExist(err) {
		t.Fatalf("incomplete scan changed destinations: %v", err)
	}
	for _, name := range []string{"photo.png", "video.mp4"} {
		if _, err := os.Stat(filepath.Join(f.root, "inbox", "nested", name)); err != nil {
			t.Fatal(err)
		}
	}
}
