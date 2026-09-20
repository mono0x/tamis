package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mono0x/tamis/internal/meta"
	"github.com/mono0x/tamis/internal/plan"
)

func TestPreviewExtractsOnlyFilterMetadata(t *testing.T) {
	config, source, _ := commandFixture(t, "copy", "fail")
	// A PNG signature suffices for kind detection, but its metadata is truncated.
	writeFile(t, source, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	output, _, err := executeCommand("", "preview", config, "--json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := plan.ReadNDJSON(strings.NewReader(output))
	if err != nil || len(p.Items) != 1 {
		t.Fatalf("plan=%+v error=%v", p, err)
	}
	output, _, err = executeCommand("", "scan", "--config", config)
	if err != nil {
		t.Fatal(err)
	}
	var r meta.Record
	if err := json.Unmarshal([]byte(output), &r); err != nil || !r.ExtractionFailed {
		t.Fatalf("diagnostic scan lost failure: %+v %v", r, err)
	}
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, config, []byte(strings.Replace(string(data), `kind == "image"`, `kind == "image" && captured != nil`, 1)))
	output, _, err = executeCommand("", "preview", config, "--json")
	if err != nil {
		t.Fatal(err)
	}
	p, err = plan.ReadNDJSON(strings.NewReader(output))
	if err != nil || len(p.Items) != 0 || len(p.Skipped) != 1 || !strings.Contains(p.Skipped[0].Reason, "metadata extraction failed") {
		t.Fatalf("plan=%+v error=%v", p, err)
	}
}
