package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mono0x/tamis/internal/meta"
)

func TestPositionalConfig(t *testing.T) {
	config, source, _ := commandFixture(t, "copy", "fail")
	t.Chdir(filepath.Dir(config))
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"check", []string{"check", "./config.yaml"}, "configuration is valid"},
		{"doctor", []string{"doctor", "./config.yaml"}, filepath.Dir(source)},
		{"explain", []string{"--json", "explain", "./config.yaml", source}, `"rule":"archive"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, _, err := executeCommand("", tc.args...)
			if err != nil || !strings.Contains(output, tc.want) {
				t.Fatalf("output = %s, error = %v", output, err)
			}
		})
	}
}

func TestConfigIsNeverImplicit(t *testing.T) {
	config, _, dest := commandFixture(t, "copy", "fail")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	defaultPath := filepath.Join(home, ".config", "tamis", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(defaultPath), 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defaultPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"apply", "preview", "check", "doctor", "explain", "scan"} {
		t.Run(command, func(t *testing.T) {
			if _, _, err := executeCommand("y\n", command); err == nil {
				t.Fatal("accepted missing configuration or PATH")
			}
			args := []string{command, filepath.Join(home, "missing.yaml")}
			if command == "scan" {
				args = []string{"scan", "--config", filepath.Join(home, "missing.yaml")}
			}
			if _, _, err := executeCommand("y\n", args...); err == nil {
				t.Fatal("accepted nonexistent input")
			}
		})
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("unexpected destination: %v", err)
	}
}

func TestConfigArgumentsRejectAmbiguity(t *testing.T) {
	config, source, _ := commandFixture(t, "copy", "fail")
	for _, args := range [][]string{
		{"apply", config, source},
		{"preview", config, source},
		{"check", config, source},
		{"doctor", config, source},
		{"explain", config, source, source},
		{"apply", "--config", config},
		{"scan", filepath.Dir(source), "--config", config},
	} {
		t.Run(strings.Join(args[:2], " "), func(t *testing.T) {
			if _, _, err := executeCommand("y\n", args...); err == nil {
				t.Fatalf("accepted ambiguous arguments: %v", args)
			}
		})
	}
}

func TestScanExplicitInputs(t *testing.T) {
	config, source, _ := commandFixture(t, "copy", "fail")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"path", []string{"scan", filepath.Dir(source)}},
		{"config", []string{"scan", "--config", config}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, _, err := executeCommand("", tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			var record meta.Record
			if err := json.Unmarshal([]byte(output), &record); err != nil {
				t.Fatal(err)
			}
			if record.Path != source {
				t.Fatalf("unexpected record: %+v", record)
			}
		})
	}
}
