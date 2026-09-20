package rule

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mono0x/tamis/internal/meta"
)

func TestDestinationRejectsMetadataPathComponents(t *testing.T) {
	for _, field := range []string{"model", "make", "lens", "basename", "capture.value"} {
		for _, value := range []string{"../escaped", "a/b", `a\b`, ".", "..", "a\x00b"} {
			t.Run(field+"/"+value, func(t *testing.T) {
				env := Env{Model: value, Make: value, Lens: value, Basename: value, Capture: map[string]string{"value": value}}
				template := compileTemplate("/archive/{{"+field+"}}/", nil)
				if dest, err := template.RenderPath(env); err == nil {
					t.Fatalf("unsafe metadata accepted: %q", dest)
				}
			})
		}
	}
}

func TestDestinationAllowsPathsDatesAndSlug(t *testing.T) {
	stamp := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	env := Env{Path: "/inbox/nested/synthetic.jpg", RelPath: "nested/synthetic.jpg", Captured: &stamp, Model: "../Synthetic/Camera"}
	for _, tc := range []struct{ template, want string }{
		{`/archive/{{captured | date "2006/01"}}/`, "/archive/2001/02/"},
		{`/archive/{{relpath}}`, "/archive/nested/synthetic.jpg"},
		{`{{path}}`, env.Path},
		{`/archive/{{model | slug}}/`, "/archive/synthetic-camera/"},
	} {
		got, err := compileTemplate(tc.template, nil).RenderPath(env)
		if err != nil || got != tc.want {
			t.Fatalf("%s: got %q err=%v", tc.template, got, err)
		}
	}
}

func TestEvaluateRejectsEscapingMetadata(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{Version: 1, Rules: []Rule{{Name: "archive", Location: dir, When: `model != ""`, Actions: []Action{{Move: filepath.Join(dir, "archive") + "/{{model}}/"}}}}}
	if err := cfg.defaultsAndValidate(); err != nil {
		t.Fatal(err)
	}
	c, err := compile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Evaluate([]meta.Record{{Path: filepath.Join(dir, "synthetic.jpg"), Model: "../escaped"}}, time.Now()); err == nil {
		t.Fatal("metadata escaped destination during evaluation")
	}
}

func TestRenameRejectsDirectoryComponents(t *testing.T) {
	for _, value := range []string{"", ".", ".."} {
		rename := compileTemplate("{{model}}", nil)
		action := compiledAction{kind: "move", dest: compileTemplate("/archive/", nil), rename: &rename}
		if _, err := renderAction(action, Env{Model: value}, &meta.Record{Path: "/inbox/synthetic.jpg"}, false); err == nil {
			t.Fatalf("unsafe rename accepted: %q", value)
		}
	}
}
