package rule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mono0x/tamis/internal/meta"
)

func TestLocationConfigValidation(t *testing.T) {
	for _, tc := range []struct{ name, data, want string }{
		{"missing", "rules:\n  - name: archive\n", "location is required"},
		{"blank", "rules:\n  - name: archive\n    location: '  '\n", "location is required"},
		{"unnamed", "rules:\n  - location: /synthetic\n", "name must be non-empty and unique"},
		{"duplicate", "rules:\n  - name: archive\n    location: /synthetic\n  - name: archive\n    location: /other\n", "name must be non-empty and unique"},
		{"old sources", "sources: []\n", "field sources not found"},
		{"old reference", "rules:\n  - name: archive\n    location: /synthetic\n    source: inbox\n", "field source not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte("version: 1\n"+tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %s", err, tc.want)
			}
		})
	}
}

func TestLocationExpansion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Chdir(dir)
	cfg := &Config{Version: 1, Rules: []Rule{{Name: "home", Location: "~/inbox"}, {Name: "relative", Location: "inbox"}}}
	if err := cfg.defaultsAndValidate(); err != nil {
		t.Fatal(err)
	}
	for _, r := range cfg.Rules {
		if r.Location != filepath.Join(dir, "inbox") {
			t.Fatal(r.Location)
		}
	}
	// A bare "~" is the home directory itself, not a child named "~".
	if got, err := expandHome("~"); err != nil || got != dir {
		t.Fatalf("expandHome(~) = %q, %v", got, err)
	}
}

func TestOverlappingLocationsUseEachRulesRelativePath(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	cfg := &Config{Version: 1, Rules: []Rule{
		{Name: "excluded", Location: root, Recursive: true, Exclude: []string{"nested"}, Actions: []Action{{Trash: true}}},
		{Name: "flat", Location: root, Actions: []Action{{Trash: true}}},
		{Name: "parent", Location: root, Recursive: true, Include: []string{"**/*.jpg"}, Actions: []Action{{Copy: "/synthetic/parent/{{relpath}}"}}},
		{Name: "child", Location: nested, Actions: []Action{{Copy: "/synthetic/child/{{relpath}}"}}},
		{Name: "finish", Location: root, Recursive: true, Actions: []Action{{Trash: true}}},
		{Name: "later", Location: nested, Actions: []Action{{Copy: "/synthetic/later"}}},
	}}
	if err := cfg.defaultsAndValidate(); err != nil {
		t.Fatal(err)
	}
	c, err := compile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// The paths do not exist: evaluation must remain independent of filesystem I/O.
	record := meta.Record{Path: filepath.Join(nested, "sample.jpg"), RelPath: "unchanged"}
	p, err := c.Evaluate([]meta.Record{record}, time.Now())
	if err != nil || len(p.Items) != 3 {
		t.Fatalf("plan=%+v error=%v", p, err)
	}
	if p.Items[0].Rule != "parent" || p.Items[0].Ops[0].Dest != "/synthetic/parent/nested/sample.jpg" || p.Items[1].Rule != "child" || p.Items[1].Ops[0].Dest != "/synthetic/child/sample.jpg" || p.Items[2].Rule != "finish" {
		t.Fatalf("plan=%+v", p)
	}
	if record.RelPath != "unchanged" {
		t.Fatal("mutated input")
	}
}
