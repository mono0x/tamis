package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mono0x/tamis/internal/plan"
)

func commandFixture(t *testing.T, action, conflict string) (config, source, dest string) {
	t.Helper()
	dir := t.TempDir()
	inbox := filepath.Join(dir, "inbox")
	if err := os.Mkdir(inbox, 0o700); err != nil {
		t.Fatal(err)
	}
	source = filepath.Join(inbox, "sample.png")
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	dest = filepath.Join(dir, "archive", "sample.png")
	config = filepath.Join(dir, "config.yaml")
	operation := fmt.Sprintf("%s: %q", action, dest)
	if action == "trash" {
		operation = "trash: true"
	}
	content := fmt.Sprintf(`version: 1
settings:
  on_conflict: %s
rules:
  - name: archive
    location: %q
    when: 'kind == "image"'
    actions:
      - %s
`, conflict, inbox, operation)
	if err := os.WriteFile(config, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return
}

func executeCommand(input string, args ...string) (string, string, error) {
	cmd := newRoot()
	var output, errors bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&errors)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return output.String(), errors.String(), err
}

func TestVersion(t *testing.T) {
	output, errors, err := executeCommand("", "--version")
	if err != nil || errors != "" || output != "tamis version "+version+"\n" {
		t.Fatalf("unexpected version output: %q, stderr: %q, error: %v", output, errors, err)
	}
}

func TestPreviewLeavesFilesUntouched(t *testing.T) {
	for _, action := range []string{"copy", "move", "trash"} {
		t.Run(action, func(t *testing.T) {
			config, source, dest := commandFixture(t, action, "rename")
			before, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			output, errors, err := executeCommand("", "preview", config, "--json", "--trash-dir", filepath.Join(filepath.Dir(config), "trash"))
			if err != nil {
				t.Fatal(err)
			}
			p, err := plan.ReadNDJSON(strings.NewReader(output))
			if err != nil || len(p.Items) != 1 || len(p.Items[0].Ops) != 1 || string(p.Items[0].Ops[0].Kind) != action {
				t.Fatalf("unexpected preview: %s; %v", output, err)
			}
			if strings.Contains(errors, "[y/N]") {
				t.Fatal("preview requested confirmation")
			}
			after, err := os.ReadFile(source)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("source changed: %v", err)
			}
			if _, err := os.Stat(filepath.Dir(dest)); !os.IsNotExist(err) {
				t.Fatalf("destination directory created: %v", err)
			}
		})
	}
}

type unreadableInput struct{ t *testing.T }

func (r unreadableInput) Read([]byte) (int, error) {
	r.t.Fatal("apply read standard input")
	return 0, nil
}

func TestApplyWithoutConfirmation(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		t.Run(fmt.Sprintf("json=%v", jsonOutput), func(t *testing.T) {
			config, source, dest := commandFixture(t, "copy", "rename")
			cmd := newRoot()
			var output, stderr bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&stderr)
			cmd.SetIn(unreadableInput{t})
			args := []string{"apply", config}
			if jsonOutput {
				args = append(args, "--json")
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(stderr.String(), "[y/N]") {
				t.Fatal("apply requested confirmation")
			}
			original, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			copied, err := os.ReadFile(dest)
			if err != nil || !bytes.Equal(original, copied) {
				t.Fatalf("copy mismatch: %v", err)
			}
			if jsonOutput {
				lines := strings.Split(strings.TrimSpace(output.String()), "\n")
				for _, line := range lines {
					if !json.Valid([]byte(line)) {
						t.Fatalf("invalid JSON: %s", line)
					}
				}
				if !strings.Contains(lines[len(lines)-1], `"completed":1`) {
					t.Fatalf("missing result: %s", output.String())
				}
			} else if !strings.Contains(output.String(), "applied 1 operations") {
				t.Fatalf("missing result: %s", output.String())
			}
		})
	}
}

func TestCommandsValidateConflicts(t *testing.T) {
	for _, command := range []string{"preview", "apply"} {
		t.Run(command, func(t *testing.T) {
			config, _, dest := commandFixture(t, "copy", "fail")
			if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dest, []byte("existing"), 0o600); err != nil {
				t.Fatal(err)
			}
			_, _, err := executeCommand("y\n", command, config)
			if err == nil || !strings.Contains(err.Error(), "destination exists") {
				t.Fatalf("expected conflict: %v", err)
			}
			data, err := os.ReadFile(dest)
			if err != nil || string(data) != "existing" {
				t.Fatalf("destination changed: %v", err)
			}
		})
	}
}

func TestCommandsRejectRuleSelection(t *testing.T) {
	for _, command := range []string{"preview", "apply"} {
		t.Run(command, func(t *testing.T) {
			config, _, dest := commandFixture(t, "copy", "rename")
			_, _, err := executeCommand("y\n", command, config, "--rule", "missing")
			if err == nil || !strings.Contains(err.Error(), "unknown flag: --rule") {
				t.Fatalf("expected source error: %v", err)
			}
			if _, err := os.Stat(dest); !os.IsNotExist(err) {
				t.Fatalf("unexpected destination: %v", err)
			}
		})
	}
}
