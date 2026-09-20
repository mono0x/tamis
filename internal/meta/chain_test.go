package meta

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type testProvider func(context.Context, *Record) (*Record, error)

func (testProvider) Supports(Format) bool { return true }
func (testProvider) Provides() []string   { return capturedFields }
func (p testProvider) Extract(ctx context.Context, base *Record, _ io.ReadSeeker) (*Record, error) {
	return p(ctx, base)
}

func chainFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic")
	if err := os.WriteFile(path, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestChainRecoversProviderFailure(t *testing.T) {
	for _, panics := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "panic"}[panics], func(t *testing.T) {
			path := chainFixture(t)
			chain := Chain{Providers: []Provider{testProvider(func(context.Context, *Record) (*Record, error) {
				if panics {
					panic("synthetic failure")
				}
				return nil, errors.New("synthetic failure")
			}), FSProvider{}}}
			record, err := chain.Extract(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			if !record.ExtractionFailed || record.Captured == nil || !strings.HasPrefix(record.Captured.Source, "fs:") || len(record.Warnings) != 1 || !strings.Contains(record.Warnings[0], "synthetic failure") {
				t.Fatalf("unsafe fallback: %+v", record)
			}
			chain.Providers = chain.Providers[:1]
			if record, err := chain.Extract(context.Background(), path); err == nil || record != nil {
				t.Fatalf("all providers failed: record=%+v err=%v", record, err)
			}
		})
	}
}

func TestChainMergesMetadataAndStopsAtTimestamp(t *testing.T) {
	path := chainFixture(t)
	stamp := &Timestamp{Time: time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC), Source: "synthetic"}
	var shared *Record
	chain := Chain{Providers: []Provider{
		testProvider(func(_ context.Context, base *Record) (*Record, error) {
			shared = base
			r := base.clone()
			r.Captured = nil
			r.Raw["first"] = "retained"
			r.Warnings = []string{"first warning"}
			return r, nil
		}),
		testProvider(func(_ context.Context, base *Record) (*Record, error) {
			if base != shared || len(base.Raw) != 0 || len(base.Warnings) != 0 {
				t.Fatal("providers did not receive the unchanged shared base")
			}
			r := base.clone()
			r.Captured = stamp
			r.Raw = map[string]any{"first": "ignored", "second": "added"}
			r.Warnings = []string{"second warning"}
			return r, nil
		}),
		testProvider(func(context.Context, *Record) (*Record, error) {
			t.Fatal("provider called after timestamp was found")
			return nil, nil
		}),
	}}
	r, err := chain.Extract(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if r.Captured != stamp || r.ExtractionFailed || !reflect.DeepEqual(r.Raw, map[string]any{"first": "retained", "second": "added"}) || !reflect.DeepEqual(r.Warnings, []string{"first warning", "second warning"}) {
		t.Fatalf("incorrect merged record: %+v", r)
	}
}

func TestChainCancellationDoesNotFallBack(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	chain := Chain{Providers: []Provider{testProvider(func(_ context.Context, base *Record) (*Record, error) {
		cancel()
		return base.clone(), nil
	}), FSProvider{}}}
	if record, err := chain.Extract(ctx, chainFixture(t)); !errors.Is(err, context.Canceled) || record != nil {
		t.Fatalf("cancelled extraction returned record=%+v err=%v", record, err)
	}
}

type readerProvider func(context.Context, *Record, io.ReadSeeker) (*Record, error)

func (readerProvider) Supports(Format) bool { return true }
func (readerProvider) Provides() []string   { return capturedFields }
func (p readerProvider) Extract(ctx context.Context, base *Record, reader io.ReadSeeker) (*Record, error) {
	return p(ctx, base, reader)
}

func TestChainSharesSourceAndResetsReader(t *testing.T) {
	for _, outcome := range []string{"continue", "error", "panic"} {
		t.Run(outcome, func(t *testing.T) {
			path := chainFixture(t)
			var shared io.ReadSeeker
			chain := Chain{Providers: []Provider{
				readerProvider(func(_ context.Context, base *Record, reader io.ReadSeeker) (*Record, error) {
					shared = reader
					data, err := io.ReadAll(reader)
					if err != nil || string(data) != "synthetic" {
						t.Fatalf("reader was not reset after sniffing: %q %v", data, err)
					}
					// Replace the path while the original descriptor remains open.
					replacement := filepath.Join(filepath.Dir(path), "replacement")
					if err := os.WriteFile(replacement, []byte("different synthetic bytes"), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(replacement, path); err != nil {
						t.Fatal(err)
					}
					switch outcome {
					case "error":
						return nil, errors.New("synthetic failure")
					case "panic":
						panic("synthetic failure")
					}
					return base.clone(), nil
				}),
				readerProvider(func(ctx context.Context, base *Record, reader io.ReadSeeker) (*Record, error) {
					if reader != shared {
						t.Fatal("source reader was reopened")
					}
					data, err := io.ReadAll(reader)
					if err != nil || string(data) != "synthetic" {
						t.Fatalf("fallback did not read the original source from the start: %q %v", data, err)
					}
					return (FSProvider{}).Extract(ctx, base, reader)
				}),
			}}
			record, err := chain.Extract(context.Background(), path)
			if err != nil || record.Captured == nil || record.ExtractionFailed != (outcome != "continue") {
				t.Fatalf("record=%+v err=%v", record, err)
			}
			if _, err := shared.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("source was not closed: %v", err)
			}
		})
	}
}

func TestChainClosesReaderOnFailure(t *testing.T) {
	for _, outcome := range []string{"error", "panic", "cancel"} {
		t.Run(outcome, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var shared io.ReadSeeker
			chain := Chain{Providers: []Provider{readerProvider(func(_ context.Context, _ *Record, reader io.ReadSeeker) (*Record, error) {
				shared = reader
				switch outcome {
				case "panic":
					panic("synthetic failure")
				case "cancel":
					cancel()
				}
				return nil, errors.New("synthetic failure")
			})}}
			if _, err := chain.Extract(ctx, chainFixture(t)); err == nil {
				t.Fatal("extraction unexpectedly succeeded")
			}
			if _, err := shared.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("source was not closed after %s: %v", outcome, err)
			}
		})
	}
}
