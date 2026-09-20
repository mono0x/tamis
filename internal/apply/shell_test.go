package apply

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mono0x/tamis/internal/plan"
)

func TestShellReplacementFeedsLaterCopiesAndTrash(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source with spaces; quotes'")
	if err := os.WriteFile(source, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	value := "synthetic ' $(touch SHOULD_NOT_EXIST) ;\nsecond line"
	command := &plan.Command{Run: `printf '%s' "$VALUE" > "$TAMIS_FILE.new"
mv "$TAMIS_FILE.new" "$TAMIS_FILE"`, Env: map[string]string{"VALUE": value}}
	p := plan.Plan{Items: []plan.Item{
		{Source: source, Ops: []plan.Op{{Kind: plan.Shell, Command: command}}},
		{Source: source, Ops: []plan.Op{{Kind: plan.Copy, Dest: filepath.Join(dir, "one")}, {Kind: plan.Copy, Dest: filepath.Join(dir, "two")}, {Kind: plan.Trash}}},
	}}
	v, err := Validate(p, Options{TrashDir: filepath.Join(dir, "trash")})
	if err != nil {
		t.Fatal(err)
	}
	// The validated command must not share mutable state with the input plan.
	command.Env["VALUE"] = "changed after validation"
	r, err := v.Run(context.Background(), Options{})
	if err != nil || r.Completed != 4 {
		t.Fatalf("result=%+v error=%v", r, err)
	}
	for _, dest := range []string{filepath.Join(dir, "one"), filepath.Join(dir, "two"), v.Plan.Items[1].Ops[2].Dest} {
		assertContents(t, dest, value)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source not trashed: %v", err)
	}
}

func TestShellFailureStopsLaterOperations(t *testing.T) {
	for _, script := range []string{`printf changed > "$TAMIS_FILE"; exit 7`, `rm "$TAMIS_FILE"`, `rm "$TAMIS_FILE"; ln -s /nonexistent "$TAMIS_FILE"`} {
		dir := t.TempDir()
		source, dest := filepath.Join(dir, "source"), filepath.Join(dir, "copy")
		if err := os.WriteFile(source, []byte("before"), 0o600); err != nil {
			t.Fatal(err)
		}
		p := plan.Plan{Items: []plan.Item{{Source: source, Ops: []plan.Op{
			{Kind: plan.Shell, Command: &plan.Command{Run: script}}, {Kind: plan.Copy, Dest: dest}, {Kind: plan.Trash},
		}}}}
		r, err := Run(context.Background(), p, Options{TrashDir: filepath.Join(dir, "trash")})
		if err == nil || r.Completed != 0 {
			t.Fatalf("result=%+v error=%v", r, err)
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Fatalf("copy executed after shell failure: %v", err)
		}
	}
}

func TestShellAfterCopyRejectedAcrossItems(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.WriteFile(source, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := plan.Plan{Items: []plan.Item{
		{Source: source, Ops: []plan.Op{{Kind: plan.Copy, Dest: filepath.Join(dir, "copy")}}},
		{Source: source, Ops: []plan.Op{{Kind: plan.Shell, Command: &plan.Command{Run: `printf changed > "$TAMIS_FILE"`}}}},
	}}
	if _, err := Run(context.Background(), p, Options{Force: true}); err == nil {
		t.Fatal("shell after copy accepted")
	}
	assertContents(t, source, "before")
	assertNoCopyTemps(t, dir, 1)
}

func TestShellCancellationAndBoundedErrors(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "synthetic")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := runShell(ctx, source, plan.Command{Run: `sleep 30`}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("shell cancellation waited for child")
	}
	err := runShell(context.Background(), source, plan.Command{Run: `printf '%20000s' x >&2; exit 7`})
	if err == nil || !strings.Contains(err.Error(), "exit status 7") || len(err.Error()) > 17*1024 {
		t.Fatalf("unexpected stderr handling: %v", err)
	}
}
