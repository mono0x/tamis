package rule

import (
	"fmt"
	"testing"
	"time"

	"github.com/expr-lang/expr"
	"github.com/mono0x/tamis/internal/meta"
	"github.com/mono0x/tamis/internal/scan"
)

func BenchmarkEvaluate10000(b *testing.B) {
	program, err := expr.Compile(`kind == "image"`, expr.Env(Env{}), expr.AsBool(), expr.MaxNodes(200))
	if err != nil {
		b.Fatal(err)
	}
	cfg := &Config{Hash: "benchmark", Settings: Settings{OnConflict: "rename"}}
	compiled := &Compiled{Config: cfg, rules: []compiledRule{
		{rule: Rule{Name: "images", Location: "/synthetic", Stop: true}, matcher: scan.NewMatcher(scan.Source{Path: "/input"}), when: program, actions: []compiledAction{
			{kind: "copy", dest: compileTemplate(`/tmp/benchmark/{{ captured | date "2006/01" }}/`, nil), onConflict: "rename"},
		}},
	}}
	captured := timestampForBenchmark()
	records := make([]meta.Record, 10_000)
	for i := range records {
		records[i] = meta.Record{Path: fmt.Sprintf("/input/meaningless-%05d.jpg", i), Kind: meta.KindImage, Format: meta.FormatJPEG, Captured: &captured}
	}
	now := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	for b.Loop() {
		if _, err := compiled.Evaluate(records, now); err != nil {
			b.Fatal(err)
		}
	}
}

func timestampForBenchmark() meta.Timestamp {
	return meta.Timestamp{Time: time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC), HasOffset: true, Source: "synthetic"}
}
