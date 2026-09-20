package rule

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/expr-lang/expr"
	"github.com/mono0x/tamis/internal/meta"
	"github.com/mono0x/tamis/internal/scan"
)

func TestEvaluateSkipsUnmatched(t *testing.T) {
	for _, captured := range []*meta.Timestamp{nil, {Time: time.Date(2001, 2, 3, 0, 0, 0, 0, time.UTC)}} {
		name := "without timestamp"
		if captured != nil {
			name = "with timestamp"
		}
		t.Run(name, func(t *testing.T) {
			cfg := &Config{Hash: "x", Settings: Settings{OnConflict: "rename"}}
			program, err := expr.Compile("false", expr.Env(Env{}), expr.AsBool())
			if err != nil {
				t.Fatal(err)
			}
			compiled := &Compiled{Config: cfg, rules: []compiledRule{{rule: Rule{Name: "none", Location: "/synthetic"}, matcher: scan.NewMatcher(scan.Source{Path: "/synthetic"}), when: program}}}
			record := meta.Record{Path: filepath.Join("/synthetic", "meaningless.jpg"), Captured: captured}
			got, err := compiled.Evaluate([]meta.Record{record}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Items) != 0 || len(got.Skipped) != 1 || got.Skipped[0].Source != record.Path || got.Skipped[0].Reason != "no matching rule" {
				t.Fatalf("unexpected: %#v", got)
			}
		})
	}
}
func TestEvaluateSkipsParserFailure(t *testing.T) {
	cfg := &Config{Settings: Settings{OnConflict: "rename"}}
	record := meta.Record{Path: "/meaningless.jpg", ExtractionFailed: true}
	got, err := (&Compiled{Config: cfg}).Evaluate([]meta.Record{record}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 0 || len(got.Skipped) != 1 {
		t.Fatalf("unexpected: %#v", got)
	}
}
