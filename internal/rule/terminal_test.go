package rule

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mono0x/tamis/internal/meta"
)

func TestTerminalRuleStopsEvaluationForEachInput(t *testing.T) {
	for _, action := range []Action{{Move: "/synthetic/moved"}, {Trash: true}} {
		cfg := &Config{Version: 1, Rules: []Rule{
			{Name: "copy", Location: "/synthetic", Actions: []Action{{Copy: "/synthetic/one"}, {Copy: "/synthetic/two"}}},
			{Name: "finish", Location: "/synthetic", When: `basename == "a"`, Actions: []Action{{Copy: "/synthetic/three"}, action}},
			{Name: "later", Location: "/synthetic", Actions: []Action{{Copy: "/synthetic/later"}}},
		}}
		if err := cfg.defaultsAndValidate(); err != nil {
			t.Fatal(err)
		}
		c, err := compile(cfg)
		if err != nil {
			t.Fatal(err)
		}
		var records []meta.Record
		for _, name := range []string{"a", "b"} {
			records = append(records, meta.Record{Path: filepath.Join(cfg.Rules[0].Location, name)})
		}
		p, err := c.Evaluate(records, time.Now())
		if err != nil || len(p.Items) != 4 {
			t.Fatalf("plan=%+v error=%v", p, err)
		}
		for i, name := range []string{"copy", "finish", "copy", "later"} {
			if p.Items[i].Rule != name || p.Items[i].Source != records[i/2].Path {
				t.Fatalf("unexpected evaluation order: %+v", p.Items)
			}
		}
		if len(p.Items[0].Ops) != 2 || len(p.Items[1].Ops) != 2 {
			t.Fatalf("copy fanout lost: %+v", p.Items)
		}
	}
}

func TestTerminalActionMustBeLastAndUnique(t *testing.T) {
	for _, first := range []Action{{Move: "/synthetic/move"}, {Trash: true}} {
		for _, second := range []Action{{Copy: "/synthetic/copy"}, {Move: "/synthetic/move2"}, {Trash: true}} {
			cfg := Config{Version: 1, Rules: []Rule{
				{Name: "bad", Location: "/synthetic", Actions: []Action{first, second}},
			}}
			if err := cfg.defaultsAndValidate(); err == nil {
				t.Fatalf("accepted actions after terminal: %+v", cfg.Rules)
			}
		}
	}
}

func TestCopyRuleCanStillStopExplicitly(t *testing.T) {
	cfg := &Config{Version: 1, Rules: []Rule{
		{Name: "first", Location: "/synthetic", Stop: true, Actions: []Action{{Copy: "/synthetic/first"}}},
		{Name: "later", Location: "/synthetic", Actions: []Action{{Copy: "/synthetic/later"}}},
	}}
	if err := cfg.defaultsAndValidate(); err != nil {
		t.Fatal(err)
	}
	c, err := compile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Evaluate([]meta.Record{{Path: "/synthetic/source"}}, time.Now())
	if err != nil || len(p.Items) != 1 || p.Items[0].Rule != "first" {
		t.Fatalf("explicit stop ignored: %+v %v", p, err)
	}
}
