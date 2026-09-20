package apply

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mono0x/tamis/internal/plan"
)

func TestRunUsesOnlyTemporarySyntheticFiles(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "meaningless.bin")
	if err := os.WriteFile(source, []byte("meaningless synthetic bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	copyDest := filepath.Join(dir, "copy", "x.bin")
	moveDest := filepath.Join(dir, "move", "x.bin")
	p := plan.Plan{
		Items: []plan.Item{
			{Source: source, Rule: "test", Ops: []plan.Op{
				{Kind: plan.Copy, Dest: copyDest, OnConflict: "fail"},
				{Kind: plan.Move, Dest: moveDest, OnConflict: "fail"},
			}},
		},
	}
	result, err := Run(context.Background(), p, Options{TrashDir: filepath.Join(dir, "trash")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("source was not moved")
	}
	if _, err := os.Stat(copyDest); err != nil {
		t.Fatal(err)
	}
	if result.Completed != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestValidateRejectsCollisionBeforeMutation(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	_ = os.WriteFile(a, []byte("a"), 0o600)
	_ = os.WriteFile(b, []byte("b"), 0o600)
	dest := filepath.Join(dir, "dest")
	p := plan.Plan{Items: []plan.Item{
		{Source: a, Ops: []plan.Op{{Kind: plan.Copy, Dest: dest, OnConflict: "fail"}}},
		{Source: b, Ops: []plan.Op{{Kind: plan.Copy, Dest: dest, OnConflict: "fail"}}},
	}}
	if _, err := Validate(p, Options{}); err == nil {
		t.Fatal("expected collision")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("validation mutated destination")
	}
}
