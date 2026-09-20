package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mono0x/tamis/internal/plan"
)

func TestEndToEnd(t *testing.T) {
	t.Run("organize", func(t *testing.T) {
		for _, action := range []string{"copy", "move"} {
			t.Run(action, func(t *testing.T) {
				t.Parallel()
				f := newEndToEndFixture(t, action, "rename")
				before := snapshot(t, f.root)
				preview := readPlan(t, f.run(t, "", "", "preview", "--json"))
				if len(preview.Items) != 2 || len(preview.Skipped) != 0 {
					t.Fatalf("unexpected preview: %+v", preview)
				}
				config, err := os.ReadFile(f.config)
				if err != nil {
					t.Fatal(err)
				}
				if preview.ConfigHash != fmt.Sprintf("%x", sha256.Sum256(config)) || preview.CreatedAt.IsZero() {
					t.Fatalf("invalid plan header: %+v", preview)
				}
				for i, name := range []string{"photo.png", "video.mp4"} {
					item := preview.Items[i]
					if item.Source != filepath.Join(f.root, "inbox", "nested", name) || item.Rule != "archive" || len(item.Ops) != 1 {
						t.Fatalf("unexpected item: %+v", item)
					}
					if op := item.Ops[0]; string(op.Kind) != action || op.Dest != filepath.Join(f.root, "archive", "2001", "02", name) {
						t.Fatalf("unexpected operation: %+v", op)
					}
				}
				assertSnapshot(t, f.root, before)

				output := f.run(t, "", "", "apply", "--json")
				applied := readResult(t, output, 2)
				if !reflect.DeepEqual(preview.Items, applied.Items) || preview.ConfigHash != applied.ConfigHash {
					t.Fatalf("apply differs from preview: %s", output)
				}
				for _, item := range preview.Items {
					rel, err := filepath.Rel(f.root, item.Source)
					if err != nil {
						t.Fatal(err)
					}
					data := before[rel]
					dest, err := filepath.Rel(f.root, item.Ops[0].Dest)
					if err != nil {
						t.Fatal(err)
					}
					before[dest] = data
					if action == "move" {
						delete(before, rel)
					}
				}
				assertSnapshot(t, f.root, before)
				if action == "move" {
					readResult(t, f.run(t, "", "", "apply", "--json"), 0)
					assertSnapshot(t, f.root, before)
				}
			})
		}
	})

	t.Run("conflicts", func(t *testing.T) {
		for _, policy := range []string{"fail", "skip", "rename"} {
			t.Run(policy, func(t *testing.T) {
				t.Parallel()
				f := newEndToEndFixture(t, "move", policy)
				// The second item conflicts, so fail must also leave the first untouched.
				writeFile(t, filepath.Join(f.root, "archive", "2001", "02", "video.mp4"), []byte("existing destination"))
				before := snapshot(t, f.root)
				wantError := ""
				if policy == "fail" {
					wantError = "destination exists"
				}
				f.run(t, "", wantError, "preview", "--json")
				assertSnapshot(t, f.root, before)
				output := f.run(t, "", wantError, "apply", "--json")
				if policy == "fail" {
					assertSnapshot(t, f.root, before)
					return
				}
				completed := 1
				if policy == "rename" {
					completed = 2
					before[filepath.Join("archive", "2001", "02", "video-2.mp4")] = before[filepath.Join("inbox", "nested", "video.mp4")]
					delete(before, filepath.Join("inbox", "nested", "video.mp4"))
				}
				readResult(t, output, completed)
				before[filepath.Join("archive", "2001", "02", "photo.png")] = before[filepath.Join("inbox", "nested", "photo.png")]
				delete(before, filepath.Join("inbox", "nested", "photo.png"))
				assertSnapshot(t, f.root, before)
			})
		}
	})
}

type endToEndFixture struct {
	root, config string
}

func TestApplyLeavesCorruptMediaUntouched(t *testing.T) {
	f := newEndToEndFixture(t, "move", "rename")
	// The signature selects the image parser, but the truncated chunk is invalid.
	writeFile(t, filepath.Join(f.root, "inbox", "nested", "photo.png"), []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 13, 'I', 'H', 'D', 'R'})
	before := snapshot(t, f.root)
	p := readPlan(t, f.run(t, "", "", "preview", "--json"))
	if len(p.Items) != 1 || len(p.Skipped) != 1 || p.Skipped[0].Source != filepath.Join(f.root, "inbox", "nested", "photo.png") || !strings.Contains(p.Skipped[0].Reason, "extraction failed") {
		t.Fatalf("corrupt media was not skipped: %+v", p)
	}
	assertSnapshot(t, f.root, before)
	readResult(t, f.run(t, "", "", "apply", "--json"), 1)
	source := filepath.Join("inbox", "nested", "video.mp4")
	before[filepath.Join("archive", "2001", "02", "video.mp4")] = before[source]
	delete(before, source)
	assertSnapshot(t, f.root, before)
}

func newEndToEndFixture(t *testing.T, action, conflict string) endToEndFixture {
	t.Helper()
	f := endToEndFixture{root: t.TempDir()}
	f.config = filepath.Join(f.root, "config.yaml")
	for _, media := range []struct{ sample, name string }{{"meaningless.png", "photo.png"}, {"meaningless.mp4", "video.mp4"}} {
		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "samples", media.sample))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(f.root, "inbox", "nested", media.name), data)
	}
	writeFile(t, filepath.Join(f.root, "inbox", "nested", "ignored.txt"), []byte("excluded synthetic data"))
	config := fmt.Sprintf(`version: 1
settings:
  assume_timezone: utc
  video:
    mvhd_timezone: utc
  on_conflict: %s
rules:
  - name: archive
    location: %q
    recursive: true
    exclude: ["**/*.txt"]
    when: '(kind == "image" || kind == "video") && captured != nil'
    actions:
      - %s: %q
`, conflict, filepath.Join(f.root, "inbox"), action,
		filepath.Join(f.root, "archive", `{{ captured | date "2006/01" }}`, "{{ basename }}{{ ext }}"))
	writeFile(t, f.config, []byte(config))
	return f
}

func (f endToEndFixture) run(t *testing.T, input, wantError string, args ...string) string {
	t.Helper()
	output, stderr, err := executeCommand(input, append(append([]string{"--jobs", "2"}, args...), f.config)...)
	if (wantError == "" && err != nil) || (wantError != "" && (err == nil || !strings.Contains(err.Error(), wantError))) {
		t.Fatalf("%v: error %v, want %q\nstdout: %s\nstderr: %s", args, err, wantError, output, stderr)
	}
	if strings.Contains(stderr, "[y/N]") {
		t.Fatalf("unexpected confirmation: %s", stderr)
	}
	return output
}

func readPlan(t *testing.T, output string) plan.Plan {
	t.Helper()
	p, err := plan.ReadNDJSON(strings.NewReader(output))
	if err != nil {
		t.Fatalf("decode plan: %v\n%s", err, output)
	}
	return p
}

func readResult(t *testing.T, output string, completed int) plan.Plan {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var result struct {
		Type      string `json:"type"`
		Completed int    `json:"completed"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &result); err != nil || result.Type != "result" || result.Completed != completed {
		t.Fatalf("unexpected result, want %d completed: %s (%v)", completed, output, err)
	}
	return readPlan(t, strings.Join(lines[:len(lines)-1], "\n"))
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		files[rel] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func assertSnapshot(t *testing.T, root string, want map[string]string) {
	t.Helper()
	if got := snapshot(t, root); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected filesystem contents: got %q, want %q", got, want)
	}
}
