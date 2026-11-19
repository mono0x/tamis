package rule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsInvalidYAML(t *testing.T) {
	for _, tc := range []struct{ name, data, want string }{
		{"unknown root field", "version: 1\nunknown: true\n", "field unknown not found"},
		{"unknown setting", "version: 1\nsettings:\n  unknown: true\n", "field unknown not found"},
		{"duplicate key", "version: 1\nversion: 1\n", "already defined"},
		{"wrong type", "version: invalid\n", "into int"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %s", err, tc.want)
			}
		})
	}
}
