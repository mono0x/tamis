package apply

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mono0x/tamis/internal/plan"
)

func TestTrashDestinationValidatedBeforeAnyMove(t *testing.T) {
	dir := t.TempDir()
	a, b, blocker := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "blocker")
	for _, path := range []string{a, b, blocker} {
		if err := os.WriteFile(path, []byte("synthetic"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p := plan.Plan{Items: []plan.Item{
		{Source: a, Ops: []plan.Op{{Kind: plan.Move, Dest: filepath.Join(dir, "moved")}}},
		{Source: b, Ops: []plan.Op{{Kind: plan.Trash}}},
	}}
	r, err := Run(context.Background(), p, Options{TrashDir: filepath.Join(blocker, "trash")})
	if err == nil || r.Completed != 0 {
		t.Fatalf("invalid trash partially applied: %+v %v", r, err)
	}
	assertContents(t, a, "synthetic")
	assertContents(t, b, "synthetic")
}

func TestTrashUsesValidatedDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.WriteFile(source, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "trash")
	p := plan.Plan{Items: []plan.Item{{Source: source, Ops: []plan.Op{{Kind: plan.Trash, OnConflict: "rename"}}}}}
	v, err := Validate(p, Options{TrashDir: root})
	if err != nil {
		t.Fatal(err)
	}
	dest := v.Plan.Items[0].Ops[0].Dest
	if filepath.Dir(dest) != root {
		t.Fatalf("trash destination missing from plan: %q", dest)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("validation created trash directory: %v", err)
	}
	r, err := v.Run(context.Background(), Options{TrashDir: filepath.Join(dir, "different")})
	if err != nil || r.Completed != 1 {
		t.Fatalf("trash failed: %+v %v", r, err)
	}
	assertContents(t, dest, "synthetic")
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("trash left source: %v", err)
	}
}

func TestTrashDefault(t *testing.T) {
	path, err := trashPath("", "/synthetic")
	if err != nil || path != "" {
		t.Fatalf("native trash should have no destination: %q %v", path, err)
	}
}
