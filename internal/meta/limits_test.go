package meta

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSidecarSizeLimit(t *testing.T) {
	for _, extra := range []int64{0, 1} {
		dir := t.TempDir()
		video := filepath.Join(dir, "synthetic.mp4")
		if err := os.WriteFile(video, []byte("synthetic"), 0o600); err != nil {
			t.Fatal(err)
		}
		sidecar, err := os.Create(filepath.Join(dir, "synthetic.xmp"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sidecar.WriteString(`<xmp:CreateDate>2001-02-03T04:05:06Z</xmp:CreateDate>`); err != nil {
			t.Fatal(err)
		}
		if err := sidecar.Truncate(maxSidecarSize + extra); err != nil {
			t.Fatal(err)
		}
		if err := sidecar.Close(); err != nil {
			t.Fatal(err)
		}
		r, err := extractFixture(VideoSidecarProvider{}, video)
		if extra == 0 {
			if err != nil || r.Captured == nil {
				t.Fatalf("bounded sidecar failed: record=%+v err=%v", r, err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "sidecar exceeds") {
			t.Fatalf("oversized sidecar accepted: %v", err)
		}
	}
}

func TestPNGRejectsTruncatedChunk(t *testing.T) {
	var data bytes.Buffer
	data.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	if err := binary.Write(&data, binary.BigEndian, uint32(64<<20)); err != nil {
		t.Fatal(err)
	}
	data.WriteString("tEXt")
	data.Write([]byte{0, 0, 0, 0})
	path := filepath.Join(t.TempDir(), "synthetic.png")
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractFixture(PNGTextProvider{}, path); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("chunk length was not checked against file size: %v", err)
	}
}

func TestPNGSkipsLargeNonTextChunks(t *testing.T) {
	for _, kind := range []string{"IDAT", "vpAg", "tEXt"} {
		path := filepath.Join(t.TempDir(), "synthetic.png")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		var prefix bytes.Buffer
		prefix.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
		if err := binary.Write(&prefix, binary.BigEndian, uint32(65<<20)); err != nil {
			t.Fatal(err)
		}
		prefix.WriteString(kind)
		if _, err := file.Write(prefix.Bytes()); err != nil {
			t.Fatal(err)
		}
		if _, err := file.Seek((65<<20)+4, io.SeekCurrent); err != nil {
			t.Fatal(err)
		}
		var tail bytes.Buffer
		writeChunk(&tail, "tEXt", []byte("Creation Time\x002001-02-03T04:05:06Z"))
		writeChunk(&tail, "IEND", nil)
		if _, err := file.Write(tail.Bytes()); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		r, err := extractFixture(PNGTextProvider{}, path)
		if kind == "tEXt" {
			if err == nil || !strings.Contains(err.Error(), "PNG text chunk exceeds") {
				t.Fatalf("unbounded text accepted: %v", err)
			}
		} else if err != nil || r.Captured == nil || r.Captured.Time.Year() != 2001 {
			t.Fatalf("%s: record=%+v err=%v", kind, r, err)
		}
	}
}

func TestUnknownFormatUsesFilesystemTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "synthetic.bin")
	if err := os.WriteFile(path, []byte("synthetic without metadata"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := NewChain(nil, nil).Extract(context.Background(), path)
	if err != nil || r.ExtractionFailed || r.Captured == nil || !strings.HasPrefix(r.Captured.Source, "fs:") {
		t.Fatalf("filesystem fallback changed: record=%+v err=%v", r, err)
	}
}
