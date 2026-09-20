package rule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemovedActionsRejected(t *testing.T) {
	for _, action := range []string{"clone", "hardlink", "symlink"} {
		t.Run(action, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			data := "version: 1\nrules:\n  - actions:\n      - " + action + ": /synthetic\n"
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "field "+action+" not found") {
				t.Fatalf("removed action was not rejected by YAML decoder: %v", err)
			}
		})
	}
}
