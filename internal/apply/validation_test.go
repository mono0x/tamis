package apply

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mono0x/tamis/internal/plan"
)

func TestRejectUnsupportedOperationsBeforeMutation(t *testing.T) {
	for _, kind := range []plan.OpKind{"clone", "hardlink", "symlink", "unknown"} {
		for _, force := range []bool{false, true} {
			dir := t.TempDir()
			source, dest := filepath.Join(dir, "source"), filepath.Join(dir, "dest")
			if err := os.WriteFile(source, []byte("synthetic"), 0o600); err != nil {
				t.Fatal(err)
			}
			p := plan.Plan{Items: []plan.Item{{Source: source, Ops: []plan.Op{
				{Kind: plan.Copy, Dest: dest}, {Kind: kind, Dest: dest + "-unsupported"},
			}}}}
			result, err := Run(context.Background(), p, Options{Force: force})
			if err == nil || !strings.Contains(err.Error(), "unsupported operation") || result.Completed != 0 {
				t.Fatalf("kind=%s force=%v: result=%+v error=%v", kind, force, result, err)
			}
			assertContents(t, source, "synthetic")
			assertNoCopyTemps(t, dir, 1)
		}
	}
}

func TestRejectDestinationUsedAsDirectory(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, force := range []bool{false, true} {
			dir := t.TempDir()
			a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
			for _, path := range []string{a, b} {
				if err := os.WriteFile(path, []byte("synthetic"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			dest := filepath.Join(dir, "output")
			p := plan.Plan{Items: []plan.Item{
				{Source: a, Ops: []plan.Op{{Kind: plan.Move, Dest: dest}}},
				{Source: b, Ops: []plan.Op{{Kind: plan.Move, Dest: filepath.Join(dest, "nested", "child")}}},
			}}
			if reverse {
				p.Items[0], p.Items[1] = p.Items[1], p.Items[0]
			}
			result, err := Run(context.Background(), p, Options{Force: force})
			if err == nil || result.Completed != 0 {
				t.Fatalf("reverse=%v force=%v: result=%+v err=%v", reverse, force, result, err)
			}
			assertContents(t, a, "synthetic")
			assertContents(t, b, "synthetic")
			if _, err := os.Stat(dest); !os.IsNotExist(err) {
				t.Fatalf("invalid plan created destination: %v", err)
			}
		}
	}
}

// Destination directories are answered once and cached, so a writable one must
// not vouch for an unwritable one in the same plan.
func TestRejectUnwritableDestinationPerDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the write permission check")
	}
	dir := t.TempDir()
	open, locked := filepath.Join(dir, "open"), filepath.Join(dir, "locked")
	for _, path := range []string{open, locked} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(dir, "source")
	if err := os.WriteFile(source, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	p := plan.Plan{Items: []plan.Item{{Source: source, Ops: []plan.Op{
		{Kind: plan.Copy, Dest: filepath.Join(open, "a")},
		{Kind: plan.Copy, Dest: filepath.Join(locked, "b")},
	}}}}
	if _, err := Validate(p, Options{}); err == nil || !strings.Contains(err.Error(), "not writable") {
		t.Fatalf("unwritable destination accepted: %v", err)
	}
	v, err := Validate(p, Options{Force: true})
	if err != nil || len(v.Warnings) != 1 || !strings.Contains(v.Warnings[0], "not writable") {
		t.Fatalf("force did not downgrade exactly one directory: %v, %v", v.Warnings, err)
	}
}

func TestValidateDoesNotMutateInput(t *testing.T) {
	dir := t.TempDir()
	source, dest := filepath.Join(dir, "source"), filepath.Join(dir, "dest")
	for _, path := range []string{source, dest} {
		if err := os.WriteFile(path, []byte("synthetic"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p := plan.Plan{Items: []plan.Item{{Source: source, Ops: []plan.Op{{Kind: plan.Copy, Dest: dest, OnConflict: "rename"}}}}}
	v, err := Validate(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Items[0].Ops[0].Dest != dest || p.Items[0].SourceInfo != nil || v.Plan.Items[0].Ops[0].Dest == dest {
		t.Fatal("validation did not keep the input plan independent")
	}
	result, err := v.Run(context.Background(), Options{})
	if err != nil || result.Completed != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertContents(t, v.Plan.Items[0].Ops[0].Dest, "synthetic")
}
