package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	applypkg "github.com/mono0x/tamis/internal/apply"
	"github.com/mono0x/tamis/internal/rule"
)

// afterValidationWriter simulates a concurrent change when the validated plan
// is displayed, before execution rechecks the sources and destinations.
type afterValidationWriter struct {
	io.Writer
	change func()
}

func (w *afterValidationWriter) Write(p []byte) (int, error) {
	if w.change != nil {
		change := w.change
		w.change = nil
		change()
	}
	return w.Writer.Write(p)
}

func TestApplyRejectsSourceChangedAfterValidation(t *testing.T) {
	for _, change := range []string{"directory", "symlink", "replacement", "modified"} {
		for _, force := range []bool{false, true} {
			t.Run(change+map[bool]string{true: "/force"}[force], func(t *testing.T) {
				f := newEndToEndFixture(t, "move", "rename")
				// Change the second source: the first must also remain untouched.
				source := filepath.Join(f.root, "inbox", "nested", "video.mp4")
				info, err := os.Stat(source)
				if err != nil {
					t.Fatal(err)
				}
				cmd := newRoot()
				var output, stderr bytes.Buffer
				cmd.SetOut(&output)
				cmd.SetErr(&stderr)
				var afterChange map[string]string
				cmd.SetOut(&afterValidationWriter{Writer: &output, change: func() {
					if change != "modified" {
						if err := os.Rename(source, source+".original"); err != nil {
							t.Fatal(err)
						}
					}
					switch change {
					case "directory":
						writeFile(t, filepath.Join(source, "unscanned.txt"), []byte("synthetic replacement"))
					case "symlink":
						if err := os.Symlink(source+".original", source); err != nil {
							t.Fatal(err)
						}
					case "replacement":
						writeFile(t, source, bytes.Repeat([]byte("x"), int(info.Size())))
						if err := os.Chtimes(source, info.ModTime(), info.ModTime()); err != nil {
							t.Fatal(err)
						}
					case "modified":
						writeFile(t, source, []byte("synthetic update"))
					}
					afterChange = snapshot(t, f.root)
				}})
				args := []string{"apply", f.config, "--json"}
				if force {
					args = append(args, "--force")
				}
				cmd.SetArgs(args)
				if err := cmd.Execute(); err == nil || (!strings.Contains(err.Error(), "source changed") && !strings.Contains(err.Error(), "regular file")) {
					t.Fatalf("changed source accepted: %v", err)
				}
				assertSnapshot(t, f.root, afterChange)
			})
		}
	}
}

func TestApplyRejectsNewConflictAfterValidation(t *testing.T) {
	f := newEndToEndFixture(t, "move", "rename")
	dest := filepath.Join(f.root, "archive", "2001", "02", "video.mp4")
	cmd := newRoot()
	var output, stderr bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&stderr)
	var afterChange map[string]string
	cmd.SetOut(&afterValidationWriter{Writer: &output, change: func() {
		writeFile(t, dest, []byte("synthetic concurrent destination"))
		afterChange = snapshot(t, f.root)
	}})
	cmd.SetArgs([]string{"apply", f.config, "--json"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "destination exists") {
		t.Fatalf("new conflict accepted: %v", err)
	}
	assertSnapshot(t, f.root, afterChange)
	if strings.Contains(output.String(), `"type":"result"`) {
		t.Fatal("failed apply reported success")
	}
}

func TestPlanRetainsScannedSourceIdentity(t *testing.T) {
	config, source, _ := commandFixture(t, "move", "rename")
	c, err := rule.Load(config)
	if err != nil {
		t.Fatal(err)
	}
	records, err := extractRules(context.Background(), c, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(source, source+".original"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, source, []byte("synthetic replacement"))
	p, err := c.Evaluate(records, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applypkg.Validate(p, applypkg.Options{}); err == nil || !strings.Contains(err.Error(), "source changed") {
		t.Fatalf("scan identity was lost: %v", err)
	}
}
