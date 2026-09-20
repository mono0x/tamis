package plan

import (
	"bytes"
	"testing"
	"time"
)

func TestNDJSONRoundTrip(t *testing.T) {
	want := Plan{CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), ConfigHash: "abc", Items: []Item{{Source: "/a", Rule: "r", Ops: []Op{{Kind: Copy, Dest: "/b"}}}}, Skipped: []SkipRecord{{Source: "/c", Reason: "none"}}}
	var buf bytes.Buffer
	if err := WriteNDJSON(&buf, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadNDJSON(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.ConfigHash != "abc" || got.Items[0].Ops[0].Dest != "/b" {
		t.Fatalf("unexpected %#v", got)
	}
}
