package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mono0x/tamis/internal/plan"
)

func TestShellOperationLayout(t *testing.T) {
	for _, tc := range []struct {
		name, run, body string
	}{
		{"single line", "echo synthetic", "    echo synthetic\n"},
		{"multiline", "printf '%s' \\\n  \"$VALUE\"\n", "    printf '%s' \\\n      \"$VALUE\"\n"},
		{"heredoc", "cat <<'END'\nsynthetic\n\nEND\n", "    cat <<'END'\n    synthetic\n    \n    END\n"},
	} {
		for _, indent := range []string{"  ", "    "} {
			t.Run(tc.name+"/indent="+indent, func(t *testing.T) {
				command := &plan.Command{Run: tc.run, Env: map[string]string{"VALUE": "synthetic\nvalue", "OTHER": "example"}}
				var output bytes.Buffer
				writeOps(&output, indent, []plan.Op{
					{Kind: plan.Shell, Command: command},
					{Kind: plan.Copy, Dest: "/synthetic/archive.png"},
					{Kind: plan.Trash},
				})
				want := "shell\n  command\n" + tc.body + "  env OTHER=\"example\"\n  env VALUE=\"synthetic\\nvalue\"\ncopy     → /synthetic/archive.png\ntrash    → system Trash\n"
				want = indent + strings.ReplaceAll(strings.TrimSuffix(want, "\n"), "\n", "\n"+indent) + "\n"
				if output.String() != want {
					t.Fatalf("unexpected layout:\n%s\nwant:\n%s", output.String(), want)
				}
				if command.Run != tc.run {
					t.Fatal("rendering modified the command")
				}
			})
		}
	}
}

func TestShellPreviewAndApply(t *testing.T) {
	config, source, dest := commandFixture(t, "copy", "fail")
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "    actions:\n", `    actions:
      - shell:
          run: 'printf "%s" "$VALUE" > "$TAMIS_FILE.new"; mv "$TAMIS_FILE.new" "$TAMIS_FILE"'
          env:
            VALUE: 'synthetic {{basename}}'
`, 1))
	writeFile(t, config, data)
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"preview", config}, {"preview", config, "--json"}} {
		output, _, err := executeCommand("", args...)
		if err != nil || !strings.Contains(output, "synthetic sample") || !strings.Contains(output, "TAMIS_FILE") {
			t.Fatalf("preview=%s error=%v", output, err)
		}
		after, err := os.ReadFile(source)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("preview modified source: %v", err)
		}
		if _, err := os.Stat(filepath.Dir(dest)); !os.IsNotExist(err) {
			t.Fatalf("preview created destination: %v", err)
		}
	}
	output, _, err := executeCommand("", "apply", config, "--json")
	if err != nil {
		t.Fatal(err)
	}
	readResult(t, output, 2)
	for _, path := range []string{source, dest} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "synthetic sample" {
			t.Fatalf("wrong output: %q %v", data, err)
		}
	}
}
