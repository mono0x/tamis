package rule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mono0x/tamis/internal/meta"
)

func TestEvaluateCaptureFanout(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	yaml := `version: 1
vars:
  library: ` + dir + `/library
rules:
  - name: screenshot
    location: ` + dir + `
    recursive: true
    capture: '^(?P<year>\d{4})_(?P<month>\d{2}).*\.png$'
    when: 'kind == "image"'
    actions:
      - copy: '{{library}}/{{capture.year}}/{{capture.month}}/'
      - trash: true
    stop: true
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	compiled, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	record := meta.Record{Path: filepath.Join(dir, "2026_05 meaningless.png"), Kind: meta.KindImage, Format: meta.FormatPNG, FileCTime: time.Now()}
	got, err := compiled.Evaluate([]meta.Record{record}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || len(got.Items[0].Ops) != 2 {
		t.Fatalf("unexpected plan: %#v", got)
	}
	want := filepath.Join(dir, "library", "2026", "05", "2026_05 meaningless.png")
	if got.Items[0].Ops[0].Dest != want {
		t.Fatalf("got %s want %s", got.Items[0].Ops[0].Dest, want)
	}
}
func TestNormalizeFilename(t *testing.T) {
	got := normalizeFilename("A/B\x00é.jpg", false)
	if got != "A-B-é.jpg" {
		t.Fatal(got)
	}
	long := strings.Repeat("🍰", 100) + ".jpg"
	if len(normalizeFilename(long, false)) > 255 {
		t.Fatal("too long")
	}
	if len(normalizeFilename("synthetic."+strings.Repeat("a", 256), false)) > 255 {
		t.Fatal("long extension was not truncated")
	}
}
