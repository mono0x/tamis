package apply

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/mono0x/tamis/internal/plan"
)

func TestRegressionForceOverwrite(t *testing.T) {
	d := t.TempDir()
	s := filepath.Join(d, "s")
	dst := filepath.Join(d, "d")
	if err := os.WriteFile(s, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), plan.Plan{Items: []plan.Item{{Source: s, Ops: []plan.Op{{Kind: plan.Copy, Dest: dst, OnConflict: "fail"}}}}}, Options{Force: true})
	b, _ := os.ReadFile(dst)
	if string(b) != "existing" {
		t.Fatalf("overwritten: %q err=%v", b, err)
	}
}
func TestRegressionTrashThenCopy(t *testing.T) {
	d := t.TempDir()
	s := filepath.Join(d, "s")
	if err := os.WriteFile(s, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Validate(plan.Plan{Items: []plan.Item{{Source: s, Ops: []plan.Op{{Kind: plan.Trash}, {Kind: plan.Copy, Dest: filepath.Join(d, "d")}}}}}, Options{TrashDir: filepath.Join(d, "trash")})
	if err == nil {
		t.Fatal("invalid operation order accepted")
	}
}

func TestRegressionAliasOverwrite(t *testing.T) {
	d := t.TempDir()
	if err := os.Mkdir(filepath.Join(d, "out"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(d, "out"), filepath.Join(d, "alias")); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(d, "a")
	b := filepath.Join(d, "b")
	if err := os.WriteFile(a, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	p := plan.Plan{Items: []plan.Item{{Source: a, Ops: []plan.Op{{Kind: plan.Move, Dest: filepath.Join(d, "out", "x"), OnConflict: "rename"}}}, {Source: b, Ops: []plan.Op{{Kind: plan.Move, Dest: filepath.Join(d, "alias", "x"), OnConflict: "rename"}}}}}
	r, err := Run(context.Background(), p, Options{})
	if !errors.Is(err, os.ErrExist) || r.Completed != 1 {
		t.Fatalf("expected stop at alias conflict: result=%+v err=%v", r, err)
	}

	for path, want := range map[string]string{filepath.Join(d, "out", "x"): "first", b: "second"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("data lost at %s: %q, %v", path, data, err)
		}
	}
}
func TestRegressionLongConflict(t *testing.T) {
	dir := t.TempDir()
	name := strings.Repeat("a", 255)
	dest := filepath.Join(dir, name)
	if err := os.WriteFile(dest, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	resolved, _, err := resolveConflict(dest, "rename", map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if resolved == dest || len(filepath.Base(resolved)) > 255 {
		t.Fatalf("invalid renamed path: %s", resolved)
	}
	if err := os.WriteFile(resolved, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestExclusiveOperationsPreserveDestination(t *testing.T) {
	for _, kind := range []plan.OpKind{plan.Copy, plan.Move} {
		t.Run(string(kind), func(t *testing.T) {
			dir := t.TempDir()
			source, dest := filepath.Join(dir, "source"), filepath.Join(dir, "dest")
			if err := os.WriteFile(source, []byte("source"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dest, []byte("existing"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := execute(source, plan.Op{Kind: kind, Dest: dest}); !os.IsExist(err) {
				t.Fatalf("expected existence error, got %v", err)
			}
			for path, want := range map[string]string{source: "source", dest: "existing"} {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != want {
					t.Fatalf("%s: %q, %v", path, data, err)
				}
			}
		})
	}
}
func TestDestinationDevice(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.WriteFile(source, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	dirs := destDirs{}
	for _, kind := range []plan.OpKind{plan.Move, plan.Copy} {
		device, written, err := validateDestination(dirs, info, &plan.Op{Kind: kind, Dest: filepath.Join(dir, "dest")})
		if err != nil {
			t.Fatal(err)
		}
		if device != uint64(info.Sys().(*syscall.Stat_t).Dev) {
			t.Fatalf("wrong device: %d", device)
		}
		want := uint64(0)
		if kind == plan.Copy {
			want = uint64(info.Size())
		}
		if written.bytes != want {
			t.Fatalf("%s: bytes %d, want %d", kind, written.bytes, want)
		}
	}
}

type crossVolumeInfo struct {
	os.FileInfo
	stat syscall.Stat_t
}

func (i crossVolumeInfo) Sys() any { return &i.stat }

func TestValidateCrossVolumeOperations(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.WriteFile(source, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	stat := *info.Sys().(*syscall.Stat_t)
	// Only the reported source device changes; no second volume is needed.
	stat.Dev ^= 1
	for _, kind := range []plan.OpKind{plan.Move, plan.Copy, plan.Trash} {
		t.Run(string(kind), func(t *testing.T) {
			_, written, err := validateDestination(destDirs{}, crossVolumeInfo{info, stat}, &plan.Op{Kind: kind, Dest: filepath.Join(dir, "dest")})
			if err != nil || written.bytes != uint64(info.Size()) {
				t.Fatalf("incorrect cross-volume requirements: %+v err=%v", written, err)
			}
		})
	}
}
