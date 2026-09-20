package meta

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegressionITXt(t *testing.T) {
	k, v := pngText("iTXt", []byte("Creation Time\x00\x00\x00\x00\x002001-02-03T04:05:06Z"))
	if k != "Creation Time" || v != "2001-02-03T04:05:06Z" {
		t.Fatalf("keyword=%q value=%q", k, v)
	}
}
func TestRegressionEmptyFile(t *testing.T) {
	s := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(s, nil, 0600); err != nil {
		t.Fatal(err)
	}
	_, err := NewChain(nil, nil).Extract(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
}
func TestRegressionVideoDuration(t *testing.T) {
	data, err := os.ReadFile("../../testdata/samples/meaningless.mp4")
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("0123456789abcdef<CreateDate>2001-02-03T04:05:06Z</CreateDate>")
	var b bytes.Buffer
	b.Write(data)
	if err := binary.Write(&b, binary.BigEndian, uint32(8+len(payload))); err != nil {
		t.Fatal(err)
	}
	b.WriteString("uuid")
	b.Write(payload)
	s := filepath.Join(t.TempDir(), "x.mp4")
	if err := os.WriteFile(s, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := NewChain(nil, nil).Extract(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if r.Duration == nil || *r.Duration != time.Second || r.Captured.Source != "qt:xml.creationdatevalue" {
		t.Fatalf("duration nil, captured %v", r.Captured)
	}
}

func TestPNGInternationalText(t *testing.T) {
	const value = "2001-02-03T04:05:06Z"
	for _, compressed := range []bool{false, true} {
		var payload bytes.Buffer
		payload.WriteString("Creation Time\x00")
		if compressed {
			payload.WriteByte(1)
		} else {
			payload.WriteByte(0)
		}
		payload.WriteByte(0)
		payload.WriteString("en\x00Creation Time\x00")
		if compressed {
			writer := zlib.NewWriter(&payload)
			if _, err := writer.Write([]byte(value)); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
		} else {
			payload.WriteString(value)
		}
		var png bytes.Buffer
		png.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
		writeChunk(&png, "iTXt", payload.Bytes())
		writeChunk(&png, "IEND", nil)
		path := filepath.Join(t.TempDir(), "synthetic.png")
		if err := os.WriteFile(path, png.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		record, err := extractFixture(PNGTextProvider{}, path)
		if err != nil {
			t.Fatal(err)
		}
		if record.Captured == nil || record.Captured.Time.Format(time.RFC3339) != value {
			t.Fatalf("compressed=%v captured=%v", compressed, record.Captured)
		}
	}
}
