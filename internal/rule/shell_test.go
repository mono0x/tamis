package rule

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mono0x/tamis/internal/meta"
	"github.com/mono0x/tamis/internal/plan"
)

func TestShellTemplatesOnlyEnvironment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := `version: 1
rules:
  - name: edit
    location: /synthetic
    when: 'model != ""'
    capture: '^(?P<year>\d{4})'
    actions:
      - shell:
          run: 'printf "%s" "$VALUE" > "$TAMIS_FILE"; : "{{model}}"'
          env:
            VALUE: '{{capture.year}} {{model}}'
      - copy: /synthetic/copy
      - trash: true
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Evaluate([]meta.Record{{Path: "/synthetic/2001.png", Model: "$(unexpected)"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	op := p.Items[0].Ops[0]
	if op.Kind != plan.Shell || op.Command.Env["VALUE"] != "2001 $(unexpected)" || op.Command.Run != `printf "%s" "$VALUE" > "$TAMIS_FILE"; : "{{model}}"` {
		t.Fatalf("incorrect shell rendering: %+v", op.Command)
	}
}

func TestInvalidShellConfig(t *testing.T) {
	for _, actions := range [][]Action{
		{{Shell: &plan.Command{}}},
		{{Shell: &plan.Command{Run: ":", Env: map[string]string{"TAMIS_FILE": "bad"}}}},
		{{Shell: &plan.Command{Run: ":", Env: map[string]string{"BAD=NAME": "value"}}}},
		{{Shell: &plan.Command{Run: ":"}, Copy: "/synthetic"}},
		{{Shell: &plan.Command{Run: ":"}, OnConflict: "skip"}},
		{{Copy: "/synthetic"}, {Shell: &plan.Command{Run: ":"}}},
	} {
		cfg := Config{Version: 1, Rules: []Rule{{Name: "invalid", Location: "/synthetic", Actions: actions}}}
		if err := cfg.defaultsAndValidate(); err == nil {
			t.Fatalf("invalid actions accepted: %+v", actions)
		}
	}
}
