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

func TestRunRejectsInvalidPlanBeforeAnyMutation(t *testing.T) {
	for _, scenario := range []string{"missing source", "cycle", "second move", "relative destination"} {
		for _, force := range []bool{false, true} {
			t.Run(scenario+map[bool]string{false: "", true: "/force"}[force], func(t *testing.T) {
				dir := t.TempDir()
				a, b, dest := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "dest")
				for _, path := range []string{a, b} {
					if err := os.WriteFile(path, []byte(filepath.Base(path)), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				p := plan.Plan{Items: []plan.Item{
					{Source: a, Ops: []plan.Op{{Kind: plan.Move, Dest: dest}}},
					{Source: b, Ops: []plan.Op{{Kind: plan.Move, Dest: filepath.Join(dir, "other")}}},
				}}
				switch scenario {
				case "missing source":
					p.Items[1].Source = filepath.Join(dir, "missing")
				case "cycle":
					p.Items[1].Ops[0].Dest = a
				case "second move":
					p.Items[1].Source = a
				case "relative destination":
					p.Items[1].Ops[0].Dest = "relative"
				}
				result, err := Run(context.Background(), p, Options{Force: force})
				if err == nil || result.Completed != 0 {
					t.Fatalf("invalid plan executed: %+v, %v", result, err)
				}
				for _, path := range []string{a, b} {
					assertContents(t, path, filepath.Base(path))
				}
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 2 {
					t.Fatalf("validation changed files: %v, %v", entries, err)
				}
			})
		}
	}
}

func TestCopyFailureCleansTemporaryFiles(t *testing.T) {
	for _, scenario := range []string{"read failure", "destination appeared"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			source, dest := filepath.Join(dir, "source"), filepath.Join(dir, "dest")
			if scenario == "read failure" {
				if err := os.Mkdir(source, 0o700); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(source, []byte("source"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dest, []byte("existing"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := copyFile(source, dest); err == nil {
				t.Fatal("expected copy failure")
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".tamis-") {
					t.Fatalf("temporary file leaked: %s", entry.Name())
				}
			}
			if scenario == "destination appeared" {
				assertContents(t, source, "source")
				assertContents(t, dest, "existing")
			} else {
				if _, err := os.Stat(source); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Lstat(dest); !os.IsNotExist(err) {
					t.Fatalf("failed copy published destination: %v", err)
				}
			}
		})
	}
}

func TestCopyPreservesContentsModeAndTime(t *testing.T) {
	dir := t.TempDir()
	source, dest := filepath.Join(dir, "source"), filepath.Join(dir, "dest")
	if err := os.WriteFile(source, []byte("synthetic contents"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(source, 0o640); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(source, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(source, dest); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{source, dest} {
		assertContents(t, path, "synthetic contents")
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o640 || !info.ModTime().Equal(stamp) {
			t.Fatalf("metadata changed: mode=%v mtime=%v", info.Mode(), info.ModTime())
		}
	}
}

func TestRunCancelledLeavesSourceUntouched(t *testing.T) {
	dir := t.TempDir()
	source, dest := filepath.Join(dir, "source"), filepath.Join(dir, "archive", "dest")
	if err := os.WriteFile(source, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := Run(ctx, plan.Plan{Items: []plan.Item{{Source: source, Ops: []plan.Op{{Kind: plan.Move, Dest: dest}}}}}, Options{})
	if !errors.Is(err, context.Canceled) || r.Completed != 0 {
		t.Fatalf("result=%+v err=%v", r, err)
	}
	assertContents(t, source, "synthetic")
	if _, err := os.Stat(filepath.Dir(dest)); !os.IsNotExist(err) {
		t.Fatalf("cancelled apply created destination directory: %v", err)
	}
}

func assertContents(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != want {
		t.Fatalf("%s: contents=%q want=%q err=%v", path, data, want, err)
	}
}
