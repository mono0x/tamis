package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mono0x/tamis/internal/meta"
	"github.com/mono0x/tamis/internal/plan"
)

func TestOverlappingRuleLocations(t *testing.T) {
	config, source, _ := commandFixture(t, "copy", "fail")
	root := filepath.Dir(config)
	data := fmt.Sprintf(`version: 1
rules:
  - name: excluded
    location: %q
    recursive: true
    include: ['**/*.png']
    exclude: [inbox]
    actions:
      - trash: true
  - name: parent
    location: %q
    recursive: true
    include: ['**/*.png']
    actions:
      - copy: %q
  - name: same-scope
    location: %q
    recursive: true
    include: ['**/*.png']
  - name: child
    location: %q
    actions:
      - move: %q
  - name: later
    location: %q
    actions:
      - copy: %q
`, root, root, filepath.Join(root, "copies", "{{relpath}}"), root, filepath.Dir(source), filepath.Join(root, "final", "{{relpath}}"), filepath.Dir(source), filepath.Join(root, "later"))
	writeFile(t, config, []byte(data))
	output, _, err := executeCommand("", "scan", "--config", config)
	if err != nil {
		t.Fatal(err)
	}
	var record meta.Record
	if err := json.Unmarshal([]byte(output), &record); err != nil || record.Path != source {
		t.Fatalf("expected one record: %s, %v", output, err)
	}
	for _, command := range []string{"preview", "explain"} {
		args := []string{command, config, "--json"}
		if command == "explain" {
			args = append(args, source)
		}
		output, _, err := executeCommand("", args...)
		if err != nil {
			t.Fatal(err)
		}
		var p plan.Plan
		if command == "explain" {
			var result struct {
				Plan plan.Plan `json:"plan"`
			}
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatal(err)
			}
			p = result.Plan
		} else {
			p, err = plan.ReadNDJSON(strings.NewReader(output))
			if err != nil {
				t.Fatal(err)
			}
		}
		if len(p.Items) != 3 || p.Items[0].Rule != "parent" || p.Items[1].Rule != "same-scope" || p.Items[2].Rule != "child" {
			t.Fatalf("%s plan=%+v", command, p)
		}
		if p.Items[0].Ops[0].Dest != filepath.Join(root, "copies", "inbox", "sample.png") || p.Items[2].Ops[0].Dest != filepath.Join(root, "final", "sample.png") {
			t.Fatalf("wrong relative paths: %+v", p)
		}
	}
	output, _, err = executeCommand("", "apply", config)
	if err != nil {
		t.Fatalf("%s: %v", output, err)
	}
	for _, path := range []string{filepath.Join(root, "copies", "inbox", "sample.png"), filepath.Join(root, "final", "sample.png")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{source, filepath.Join(root, "later")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("unexpected file: %s, %v", path, err)
		}
	}
	for _, args := range [][]string{
		{"scan", "--config", config, "--rule", "unknown"},
		{"preview", config, "--source", "parent"},
		{"explain", config, config},
	} {
		if _, _, err := executeCommand("", args...); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
