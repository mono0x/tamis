package meta

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

type cancellingReader struct {
	*bytes.Reader
	cancel context.CancelFunc
}

func (r cancellingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.cancel()
	return n, err
}

func TestDecoderReaderStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	underlying := bytes.NewReader([]byte("synthetic metadata"))
	reader := contextReader{ctx, cancellingReader{underlying, cancel}}
	var buffer [4]byte
	if n, err := reader.Read(buffer[:]); n != 4 || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation during read lost: n=%d err=%v", n, err)
	}
	remaining := underlying.Len()
	if n, err := reader.Read(buffer[:]); n != 0 || !errors.Is(err, context.Canceled) || underlying.Len() != remaining {
		t.Fatalf("decoder kept reading: n=%d err=%v", n, err)
	}
	if _, err := reader.Seek(0, io.SeekStart); !errors.Is(err, context.Canceled) || underlying.Len() != remaining {
		t.Fatalf("decoder kept seeking: %v", err)
	}
}
