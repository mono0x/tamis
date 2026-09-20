package rule

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mono0x/tamis/internal/plan"
)

func TestActionsRequireFilterMetadata(t *testing.T) {
	for _, name := range []string{"captured", "mtime", "ctime", "date_added", "make", "model", "lens"} {
		for _, action := range []Action{
			{Copy: "/synthetic/{{" + name + "}}/"},
			{Move: "/synthetic/", Rename: "{{" + name + "}}"},
			{Shell: &plan.Command{Run: ":", Env: map[string]string{"VALUE": "{{" + name + "}}"}}},
		} {
			cfg := &Config{Version: 1, Rules: []Rule{{Name: "r", Location: "/input", When: `ext == ".jpg"`, Actions: []Action{action}}}}
			if err := cfg.defaultsAndValidate(); err != nil {
				t.Fatal(err)
			}
			if _, err := compile(cfg); err == nil || !strings.Contains(err.Error(), "must be referenced in when") {
				t.Fatalf("%s: %v", name, err)
			}
		}
	}
}

func TestRequiredFields(t *testing.T) {
	root := t.TempDir()
	cfg := &Config{Version: 1, Vars: map[string]string{"model": "/synthetic"}, Rules: []Rule{
		{Name: "capture", Location: root, Capture: `^photo`, When: `ext == ".jpg" && captured != nil`, Actions: []Action{{Copy: `/synthetic/{{captured | date "2006"}}/`}}},
		{Name: "nested", Location: filepath.Join(root, "nested"), When: `model != "" && gps != nil && gps.lat > 0`, Actions: []Action{{Copy: `/synthetic/{{ model | slug }}/`}}},
		{Name: "other", Location: filepath.Join(root, "other"), When: `raw["example"] != nil`},
		{Name: "literal", Location: root, When: `basename == "model"`, Actions: []Action{{Copy: `{{vars.model}}/{{basename}}{{ext}}`}}},
	}}
	if err := cfg.defaultsAndValidate(); err != nil {
		t.Fatal(err)
	}
	c, err := compile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path   string
		want   []string
		absent []string
	}{
		{"photo.jpg", []string{"captured", "ext", "basename"}, []string{"raw", "model", "gps"}},
		{"unmatched.jpg", []string{"basename"}, []string{"captured", "model", "raw"}},
		{"nested/photo.jpg", []string{"model", "gps"}, []string{"captured", "raw"}},
		{"other/photo.jpg", []string{"raw"}, []string{"captured", "model"}},
	} {
		fields := c.RequiredFields(filepath.Join(root, tc.path))
		for _, name := range tc.want {
			if !fields.Has(name) {
				t.Fatalf("%s missing %s", tc.path, name)
			}
		}
		for _, name := range tc.absent {
			if fields.Has(name) {
				t.Fatalf("%s unexpectedly requires %s", tc.path, name)
			}
		}
	}
}

func TestFilterFieldReferencesAreSyntactic(t *testing.T) {
	for _, condition := range []string{`model == "" || model != ""`, `any([model], {# != ""})`} {
		fields, err := filterFields(condition)
		if err != nil || !fields.Has("model") {
			t.Fatalf("%s: %v %v", condition, fields, err)
		}
	}
	if _, err := filterFields(`$env["model"] != ""`); err == nil {
		t.Fatal("accepted dynamic environment")
	}
}
