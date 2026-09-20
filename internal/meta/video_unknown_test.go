package meta

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func videoBox(kind string, payload []byte) []byte {
	data := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(data, uint32(len(data)))
	copy(data[4:8], kind)
	copy(data[8:], payload)
	return data
}

func TestVideoUnknownMetadataItem(t *testing.T) {
	const captured = "2001-02-03T04:05:06Z"
	sample, err := os.ReadFile("../../testdata/samples/meaningless.mp4")
	if err != nil {
		t.Fatal(err)
	}
	for _, knownDate := range []bool{false, true} {
		t.Run(map[bool]string{false: "movie header", true: "metadata date"}[knownDate], func(t *testing.T) {
			// An opaque vendor item need not contain child boxes.
			items := videoBox("zzzz", []byte("synthetic opaque metadata"))
			if knownDate {
				value := append([]byte{0, 0, 0, 1, 0, 0, 0, 0}, []byte(captured)...)
				items = append(items, videoBox(string(copyrightDay[:]), videoBox("data", value))...)
			}
			extra := videoBox("udta", videoBox("meta", append(make([]byte, 4), videoBox("ilst", items)...)))
			ftypSize := binary.BigEndian.Uint32(sample)
			data := bytes.Join([][]byte{sample[:ftypSize], videoBox("moov", bytes.Join([][]byte{sample[ftypSize+8:], extra}, nil))}, nil)
			path := filepath.Join(t.TempDir(), "synthetic.mp4")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			r, err := NewChain(time.UTC, time.UTC).Extract(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			if r.ExtractionFailed || len(r.Warnings) != 0 || r.Captured == nil || r.Duration == nil || *r.Duration != time.Second {
				t.Fatalf("record=%+v", r)
			}
			source := "qt:mvhd.creation_time"
			want := time.Unix(3063917106-qtEpochOffset, 0).UTC().Format(time.RFC3339)
			if knownDate {
				source = "qt:©day"
				want = captured
			}
			if r.Captured.Source != source || r.Captured.Time.Format(time.RFC3339) != want {
				t.Fatalf("captured=%+v", r.Captured)
			}
		})
	}
}

func TestVideoMalformedKnownMetadataStillFails(t *testing.T) {
	sample, err := os.ReadFile("../../testdata/samples/meaningless.mp4")
	if err != nil {
		t.Fatal(err)
	}
	// A recognized data box must contain its type and locale headers.
	items := videoBox(string(copyrightDay[:]), videoBox("data", nil))
	extra := videoBox("udta", videoBox("meta", append(make([]byte, 4), videoBox("ilst", items)...)))
	ftypSize := binary.BigEndian.Uint32(sample)
	data := bytes.Join([][]byte{sample[:ftypSize], videoBox("moov", bytes.Join([][]byte{sample[ftypSize+8:], extra}, nil))}, nil)
	path := filepath.Join(t.TempDir(), "synthetic.mp4")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	r, err := NewChain(time.UTC, time.UTC).Extract(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !r.ExtractionFailed {
		t.Fatal("malformed known metadata accepted")
	}
}
