package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mono0x/tamis/internal/meta"
	"github.com/mono0x/tamis/internal/plan"
)

func TestExplainDateAdded(t *testing.T) {
	stamp := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	for _, tc := range []struct {
		added *time.Time
		want  string
	}{
		{&stamp, "date_added  2001-02-03T04:05:06Z"},
		{nil, "date_added  unavailable"},
	} {
		var out bytes.Buffer
		writeExplanation(&out, &meta.Record{Path: "/synthetic/input.jpg", DateAdded: tc.added}, plan.Plan{})
		if !strings.Contains(out.String(), tc.want) {
			t.Fatalf("missing addition time explanation: %s", out.String())
		}
	}
}
