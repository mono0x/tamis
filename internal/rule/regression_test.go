package rule

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mono0x/tamis/internal/meta"
)

func TestRegressionTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := &Config{Version: 1, Rules: []Rule{{Name: "archive", Location: home, Actions: []Action{{Copy: "~/Media/Archive/"}}}}}
	if err := cfg.defaultsAndValidate(); err != nil {
		t.Fatal(err)
	}
	compiled, err := compile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := compiled.Evaluate([]meta.Record{{Path: filepath.Join(home, "synthetic.jpg")}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Items[0].Ops[0].Dest; got != filepath.Join(home, "Media/Archive/synthetic.jpg") {
		t.Fatal(got)
	}
}
func TestRegressionTrunc(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	_, err := compileTemplate("{{ basename | trunc -1 }}", nil).Render(Env{Basename: "synthetic"})
	if err == nil {
		t.Fatal("negative length accepted")
	}
}
func TestRegressionCaptureLeak(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "config.yaml")
	if err := os.WriteFile(p, []byte("version: 1\nrules:\n  - name: first\n    location: /synthetic\n    capture: '^(?P<value>.+)$'\n    when: 'false'\n  - name: second\n    location: /synthetic\n    when: 'capture.value == \"a\"'\n    actions:\n      - copy: '/archive/{{capture.value}}'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.Evaluate([]meta.Record{{Path: "/synthetic/a", Captured: &meta.Timestamp{Time: time.Now()}}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 0 {
		t.Fatalf("capture leaked: %+v", out.Items)
	}
}
