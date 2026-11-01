package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mono0x/tamis/internal/meta"
	"golang.org/x/sys/unix"
)

func TestWalkIncludesOnlyRegularFiles(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "synthetic.png")
	if err := os.WriteFile(source, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "fifo.png")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, filepath.Join(dir, "link.png")); err != nil {
		t.Fatal(err)
	}
	candidates, err := Walk(Source{Path: dir})
	if err != nil || len(candidates) != 1 || candidates[0].Path != source {
		t.Fatalf("candidates=%v err=%v", candidates, err)
	}
	candidates, err = Walk(Source{Path: fifo})
	if err != nil || len(candidates) != 0 {
		t.Fatalf("direct FIFO source accepted: %v %v", candidates, err)
	}
}

func TestExtractRejectsFIFOReplacedAfterWalk(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "synthetic.png")
	if err := os.WriteFile(source, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	candidates, err := Walk(Source{Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(source, source+".original"); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(source, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := Extract(context.Background(), candidates, meta.NewChain(nil, nil), 1, 20*time.Millisecond)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO was extracted")
		}
	case <-time.After(time.Second):
		// Release a regressed blocking open without leaving a worker behind.
		fd, err := unix.Open(source, unix.O_WRONLY|unix.O_NONBLOCK, 0)
		if err == nil {
			if err := unix.Close(fd); err != nil {
				t.Error(err)
			}
		}
		select {
		case err := <-done:
			if err != nil {
				t.Logf("extraction after releasing FIFO: %v", err)
			}
		case <-time.After(time.Second):
		}
		t.Fatal("FIFO blocked extraction beyond its deadline")
	}
}
