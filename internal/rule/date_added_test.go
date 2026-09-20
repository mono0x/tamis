package rule

import (
	"testing"
	"time"

	"github.com/mono0x/tamis/internal/meta"
	"github.com/mono0x/tamis/internal/plan"
)

func TestTrashByDateAdded(t *testing.T) {
	cfg := &Config{Version: 1, Rules: []Rule{{
		Name: "expire", Location: "/synthetic", When: `date_added != nil && now.Sub(date_added).Hours() > 72`, Actions: []Action{{Trash: true}},
	}}}
	if err := cfg.defaultsAndValidate(); err != nil {
		t.Fatal(err)
	}
	c, err := compile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		elapsed time.Duration
		missing bool
		trash   bool
	}{
		{"old", 72*time.Hour + time.Second, false, true},
		{"boundary", 72 * time.Hour, false, false},
		{"recent", time.Hour, false, false},
		{"future", -time.Hour, false, false},
		{"unavailable", 0, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := now.Add(-365 * 24 * time.Hour)
			r := meta.Record{Path: "/synthetic/input.jpg", FileCTime: old, FileMTime: old, Captured: &meta.Timestamp{Time: old}}
			added := now.Add(-tc.elapsed)
			if !tc.missing {
				r.DateAdded = &added
			}
			p, err := c.Evaluate([]meta.Record{r}, now)
			if err != nil {
				t.Fatal(err)
			}
			if tc.trash {
				if len(p.Items) != 1 || len(p.Items[0].Ops) != 1 || p.Items[0].Ops[0].Kind != plan.Trash {
					t.Fatalf("old addition not trashed: %+v", p)
				}
			} else if len(p.Items) != 0 {
				t.Fatalf("birthtime or mtime triggered trash: %+v", p)
			}
			if got := newEnv(&r, "input.jpg", now, 1).AgeHours; got != 365*24 {
				t.Fatalf("age_hours changed meaning: %v", got)
			}
		})
	}
}

func TestDateAddedTemplate(t *testing.T) {
	stamp := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	template := compileTemplate(`/synthetic/{{date_added | date "2006/01/02"}}/`, nil)
	got, err := template.RenderPath(Env{DateAdded: &stamp})
	if err != nil || got != "/synthetic/2001/02/03/" {
		t.Fatalf("date added path: %q, %v", got, err)
	}
	if _, err := template.RenderPath(Env{}); err == nil {
		t.Fatal("date filter accepted an unavailable date added")
	}
	got, err = compileTemplate(`{{date_added | default "unknown"}}`, nil).Render(Env{})
	if err != nil || got != "unknown" {
		t.Fatalf("missing date added default: %q, %v", got, err)
	}
}
