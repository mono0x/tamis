package apply

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mono0x/tamis/internal/plan"
)

func TestTerminalRequiresAllCopiesAcrossItems(t *testing.T) {
	for _, kind := range []plan.OpKind{plan.Move, plan.Trash} {
		for _, skipIndex := range []int{-1, 0, 1} {
			t.Run(fmt.Sprintf("%s/skip%d", kind, skipIndex), func(t *testing.T) {
				dir := t.TempDir()
				a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
				for _, source := range []string{a, b} {
					if err := os.WriteFile(source, []byte("synthetic"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				copies := []string{filepath.Join(dir, "copy1"), filepath.Join(dir, "copy2")}
				if skipIndex >= 0 {
					if err := os.WriteFile(copies[skipIndex], []byte("existing different contents"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				p := plan.Plan{Items: []plan.Item{
					{Source: a, Ops: []plan.Op{{Kind: plan.Copy, Dest: copies[0], OnConflict: "skip"}}},
					{Source: b, Ops: []plan.Op{{Kind: kind, Dest: filepath.Join(dir, "b-final")}}},
					{Source: a, Ops: []plan.Op{{Kind: plan.Copy, Dest: copies[1], OnConflict: "skip"}, {Kind: kind, Dest: filepath.Join(dir, "a-final")}}},
				}}
				v, err := Validate(p, Options{TrashDir: filepath.Join(dir, "trash")})
				if err != nil {
					t.Fatal(err)
				}
				r, err := v.Run(context.Background(), Options{})
				if err != nil {
					t.Fatal(err)
				}
				want := 4
				if skipIndex >= 0 {
					want = 2
					assertContents(t, a, "synthetic")
					if len(r.Skipped) != 2 || r.Skipped[1].Kind != kind || !strings.Contains(r.Skipped[1].Reason, "earlier copy") {
						t.Fatalf("missing skip reasons: %+v", r)
					}
					if _, err := os.Lstat(v.Plan.Items[2].Ops[1].Dest); !os.IsNotExist(err) {
						t.Fatalf("skipped terminal executed: %v", err)
					}
				} else {
					if _, err := os.Stat(a); !os.IsNotExist(err) || len(r.Skipped) != 0 {
						t.Fatalf("terminal did not execute: %+v %v", r, err)
					}
					assertContents(t, v.Plan.Items[2].Ops[1].Dest, "synthetic")
				}
				if r.Completed != want {
					t.Fatalf("completed=%d want=%d", r.Completed, want)
				}
				for i, copy := range copies {
					want := "synthetic"
					if i == skipIndex {
						want = "existing different contents"
					}
					assertContents(t, copy, want)
				}
				if _, err := os.Stat(b); !os.IsNotExist(err) {
					t.Fatalf("unrelated input was blocked: %v", err)
				}
				assertContents(t, v.Plan.Items[1].Ops[0].Dest, "synthetic")
			})
		}
	}
}

func TestCopyFailureKeepsSourceAndCompletedCopies(t *testing.T) {
	for _, kind := range []plan.OpKind{plan.Move, plan.Trash} {
		dir := t.TempDir()
		source, archive, alias := filepath.Join(dir, "source"), filepath.Join(dir, "archive"), filepath.Join(dir, "alias")
		if err := os.WriteFile(source, []byte("synthetic"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(archive, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(archive, alias); err != nil {
			t.Fatal(err)
		}
		p := plan.Plan{Items: []plan.Item{{Source: source, Ops: []plan.Op{
			{Kind: plan.Copy, Dest: filepath.Join(archive, "copy")},
			{Kind: plan.Copy, Dest: filepath.Join(alias, "copy")},
			{Kind: kind, Dest: filepath.Join(dir, "final")},
		}}}}
		v, err := Validate(p, Options{TrashDir: filepath.Join(dir, "trash")})
		if err != nil {
			t.Fatal(err)
		}
		r, err := v.Run(context.Background(), Options{})
		if !errors.Is(err, os.ErrExist) || r.Completed != 1 {
			t.Fatalf("unexpected partial result: %+v %v", r, err)
		}
		assertContents(t, source, "synthetic")
		assertContents(t, filepath.Join(archive, "copy"), "synthetic")
		if _, err := os.Lstat(v.Plan.Items[0].Ops[2].Dest); !os.IsNotExist(err) {
			t.Fatalf("terminal executed after failed copy: %v", err)
		}
	}
}
