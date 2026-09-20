package apply

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mono0x/tamis/internal/plan"
)

func TestNativeTrashExecution(t *testing.T) {
	for _, scenario := range []string{"success", "skip copy", "failure", "changed source"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			source, dest := filepath.Join(dir, "source"), filepath.Join(dir, "copy")
			if err := os.WriteFile(source, []byte("synthetic"), 0600); err != nil {
				t.Fatal(err)
			}
			if scenario == "skip copy" {
				if err := os.WriteFile(dest, []byte("existing"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			p := plan.Plan{Items: []plan.Item{
				{Source: source, Ops: []plan.Op{{Kind: plan.Copy, Dest: dest, OnConflict: "skip"}}},
				{Source: source, Ops: []plan.Op{{Kind: plan.Trash, OnConflict: "skip"}}},
			}}
			v, err := Validate(p, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if v.Plan.Items[1].Ops[0].Dest != "" {
				t.Fatal("native destination was assigned")
			}
			if scenario == "changed source" {
				if err := os.WriteFile(source, []byte("changed synthetic"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			failure := errors.New("synthetic native failure")
			result, err := v.run(context.Background(), Options{TrashDir: filepath.Join(dir, "ignored")}, func(path string) error {
				calls++
				if path != source {
					t.Fatalf("wrong source %q", path)
				}
				assertContents(t, dest, "synthetic")
				if scenario == "failure" {
					return failure
				}
				return os.Rename(path, filepath.Join(dir, "trashed"))
			})
			switch scenario {
			case "success":
				if err != nil || calls != 1 || result.Completed != 2 {
					t.Fatalf("%+v %v calls=%d", result, err, calls)
				}
				assertContents(t, filepath.Join(dir, "trashed"), "synthetic")
			case "skip copy":
				if err != nil || calls != 0 || len(result.Skipped) != 2 {
					t.Fatalf("%+v %v calls=%d", result, err, calls)
				}
				assertContents(t, source, "synthetic")
			case "failure":
				if !errors.Is(err, failure) || calls != 1 || result.Completed != 1 {
					t.Fatalf("%+v %v calls=%d", result, err, calls)
				}
				assertContents(t, source, "synthetic")
			case "changed source":
				if err == nil || calls != 0 || result.Completed != 0 {
					t.Fatalf("%+v %v calls=%d", result, err, calls)
				}
			}
		})
	}
}

func TestNativeTrashRejectsInvalidPath(t *testing.T) {
	for _, path := range []string{"relative", "/synthetic\x00suffix", "/synthetic\xff"} {
		if err := validateTrash(path, ""); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
}
