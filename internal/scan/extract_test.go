package scan

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mono0x/tamis/internal/meta"
)

type extractProvider func(context.Context, *meta.Record) (*meta.Record, error)

func (extractProvider) Supports(meta.Format) bool { return true }
func (extractProvider) Provides() []string        { return nil }
func (p extractProvider) Extract(ctx context.Context, base *meta.Record, _ io.ReadSeeker) (*meta.Record, error) {
	return p(ctx, base)
}

func extractCandidates(t *testing.T) []Candidate {
	t.Helper()
	dir := t.TempDir()
	var candidates []Candidate
	for _, name := range []string{"c", "b", "a"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("synthetic"), 0o600); err != nil {
			t.Fatal(err)
		}
		candidates = append(candidates, Candidate{Path: path, RelPath: name})
	}
	return candidates
}

func TestExtractBoundsWorkersAndSortsResults(t *testing.T) {
	candidates := extractCandidates(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	started := make(chan struct{}, len(candidates))
	release := make(chan struct{})
	var active, maximum atomic.Int32
	provider := extractProvider(func(ctx context.Context, base *meta.Record) (*meta.Record, error) {
		count := active.Add(1)
		defer active.Add(-1)
		for old := maximum.Load(); count > old; old = maximum.Load() {
			if maximum.CompareAndSwap(old, count) {
				break
			}
		}
		started <- struct{}{}
		select {
		case <-release:
			return (meta.FSProvider{}).Extract(ctx, base, nil)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	go func() {
		defer close(release)
		for range 2 {
			select {
			case <-started:
			case <-ctx.Done():
				return
			}
		}
	}()
	records, err := Extract(ctx, candidates, meta.Chain{Providers: []meta.Provider{provider}}, 2, time.Second)
	if err != nil || len(records) != 3 || maximum.Load() != 2 || active.Load() != 0 {
		t.Fatalf("records=%+v max workers=%d active=%d err=%v", records, maximum.Load(), active.Load(), err)
	}
	for i, name := range []string{"a", "b", "c"} {
		if filepath.Base(records[i].Path) != name || records[i].RelPath != name {
			t.Fatalf("incorrect order or source attribution: %+v", records)
		}
	}
}

func TestExtractTimeoutPreservesOtherResults(t *testing.T) {
	candidates := extractCandidates(t)
	provider := extractProvider(func(ctx context.Context, base *meta.Record) (*meta.Record, error) {
		if filepath.Base(base.Path) == "b" {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return (meta.FSProvider{}).Extract(ctx, base, nil)
	})
	records, err := Extract(context.Background(), candidates, meta.Chain{Providers: []meta.Provider{provider}}, 2, 50*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), candidates[1].Path) || !strings.Contains(err.Error(), "deadline exceeded") || len(records) != 2 {
		t.Fatalf("records=%+v err=%v", records, err)
	}
	if records[0].RelPath != "a" || records[1].RelPath != "c" {
		t.Fatalf("unexpected surviving records: %+v", records)
	}
}

func TestExtractCancellationStopsScheduling(t *testing.T) {
	candidates := extractCandidates(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls int
	provider := extractProvider(func(context.Context, *meta.Record) (*meta.Record, error) {
		calls++
		cancel()
		return nil, context.Canceled
	})
	records, err := Extract(ctx, candidates, meta.Chain{Providers: []meta.Provider{provider}}, 1, time.Second)
	if !errors.Is(err, context.Canceled) || calls != 1 || len(records) != 0 {
		t.Fatalf("records=%+v calls=%d err=%v", records, calls, err)
	}
}
