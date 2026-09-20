package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	applypkg "github.com/mono0x/tamis/internal/apply"
	"github.com/mono0x/tamis/internal/plan"
)

func TestApplyReportsTerminalSkippedAfterValidation(t *testing.T) {
	for _, kind := range []plan.OpKind{plan.Move, plan.Trash} {
		for _, jsonOutput := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%v", kind, jsonOutput), func(t *testing.T) {
				config, source, dest := commandFixture(t, "copy", "skip")
				dir := filepath.Dir(config)
				terminal := "      - trash: true\n"
				if kind == plan.Move {
					terminal = fmt.Sprintf("      - move: %q\n", filepath.Join(dir, "moved"))
				}
				data, err := os.ReadFile(config)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, config, append(data, []byte(terminal)...))
				before, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}
				cmd := newRoot()
				var output, stderr bytes.Buffer
				cmd.SetOut(&output)
				cmd.SetErr(&stderr)
				cmd.SetOut(&afterValidationWriter{Writer: &output, change: func() { writeFile(t, dest, []byte("existing")) }})
				args := []string{"apply", config, "--trash-dir", filepath.Join(dir, "trash")}
				if jsonOutput {
					args = append(args, "--json")
				}
				cmd.SetArgs(args)
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				after, err := os.ReadFile(source)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("source changed: %v", err)
				}
				if !strings.Contains(stderr.String(), "skip "+string(kind)) || !strings.Contains(stderr.String(), "earlier copy was skipped") {
					t.Fatalf("skip reason missing: %s", stderr.String())
				}
				if jsonOutput {
					lines := strings.Split(strings.TrimSpace(output.String()), "\n")
					var result applypkg.Result
					if err := json.Unmarshal([]byte(lines[len(lines)-1]), &result); err != nil {
						t.Fatal(err)
					}
					if result.Completed != 0 || len(result.Skipped) != 2 || result.Skipped[1].Kind != kind || result.Skipped[1].Source != source {
						t.Fatalf("incorrect JSON result: %+v", result)
					}
				} else if !strings.Contains(output.String(), "applied 0 operations") {
					t.Fatalf("incorrect completion count: %s", output.String())
				}
			})
		}
	}
}
