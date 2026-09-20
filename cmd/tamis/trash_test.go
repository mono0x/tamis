package main

import (
	"os"
	"strings"
	"testing"
)

func TestNativeTrashPreview(t *testing.T) {
	config, source, _ := commandFixture(t, "trash", "rename")
	output, _, err := executeCommand("", "preview", config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "system Trash") {
		t.Fatalf("missing native destination: %s", output)
	}
	output, _, err = executeCommand("", "preview", config, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, `"kind":"trash"`) || strings.Contains(output, `"dest":`) {
		t.Fatalf("unexpected native plan: %s", output)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("preview changed source: %v", err)
	}
}
