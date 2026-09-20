package meta

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBasicFieldsSkipMediaProviders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.mp4")
	if err := os.WriteFile(path, []byte("synthetic invalid media"), 0600); err != nil {
		t.Fatal(err)
	}
	chain := Chain{Providers: []Provider{testProvider(func(context.Context, *Record) (*Record, error) { t.Fatal("unnecessary provider call"); return nil, nil })}, RequiredFields: func(string) Fields { return Fields{"date_added": true, "ext": true} }}
	r, err := chain.Extract(context.Background(), path)
	if err != nil || r.ExtractionFailed || r.Captured != nil || r.SourceInfo == nil || r.Format != FormatUnknown {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestSelectiveImageDate(t *testing.T) {
	for _, format := range []string{"tiff", "jpg", "png", "webp"} {
		path := filepath.Join(t.TempDir(), "synthetic."+format)
		if err := os.WriteFile(path, subsecondImage(t, format, "123456", false), 0600); err != nil {
			t.Fatal(err)
		}
		chain := NewChain(time.UTC, time.UTC)
		chain.RequiredFields = func(string) Fields { return Fields{"captured": true} }
		r, err := chain.Extract(context.Background(), path)
		if err != nil || r.ExtractionFailed || r.Captured == nil || r.Captured.Source != "exif:DateTimeOriginal + OffsetTimeOriginal" || len(r.Raw) != 0 || r.Width != 0 || r.Height != 0 {
			t.Fatalf("%s: %+v %v", format, r, err)
		}
	}
}

func TestVideoDurationSkipsDateMetadata(t *testing.T) {
	sample, err := os.ReadFile("../../testdata/samples/meaningless.mp4")
	if err != nil {
		t.Fatal(err)
	}
	ftypSize := binary.BigEndian.Uint32(sample)
	// A malformed date box should matter only when a date is needed.
	item := videoBox(string(copyrightDay[:]), videoBox("data", nil))
	extra := videoBox("udta", videoBox("meta", append(make([]byte, 4), videoBox("ilst", item)...)))
	data := append(append([]byte{}, sample[:ftypSize]...), videoBox("moov", append(append([]byte{}, sample[ftypSize+8:]...), extra...))...)
	path := filepath.Join(t.TempDir(), "synthetic.mp4")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, fields := range []Fields{{"duration": true}, {"captured": true}} {
		chain := NewChain(time.UTC, time.UTC)
		chain.RequiredFields = func(string) Fields { return fields }
		r, err := chain.Extract(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		if fields.Has("duration") {
			if r.ExtractionFailed || r.Duration == nil || *r.Duration != time.Second || r.Captured != nil || len(r.Raw) != 0 {
				t.Fatalf("duration extraction: %+v", r)
			}
		} else if !r.ExtractionFailed {
			t.Fatal("ignored malformed requested date")
		}
	}
}

func BenchmarkRequestedFields(b *testing.B) {
	path := "../../testdata/samples/meaningless.png"
	for _, tc := range []struct {
		name   string
		fields Fields
	}{{"all", nil}, {"filesystem", Fields{"date_added": true}}, {"captured", Fields{"captured": true}}} {
		b.Run(tc.name, func(b *testing.B) {
			chain := NewChain(time.UTC, time.UTC)
			chain.RequiredFields = func(string) Fields { return tc.fields }
			b.ReportAllocs()
			for b.Loop() {
				r, err := chain.Extract(context.Background(), path)
				if err != nil || r.ExtractionFailed {
					b.Fatal("synthetic extraction failed")
				}
			}
		})
	}
}

func TestVideoReadsOnlyCreationDateItem(t *testing.T) {
	const captured = "2001-02-03T04:05:06Z"
	key := []byte("com.apple.quicktime.creationdate")
	keys := make([]byte, 16+len(key))
	binary.BigEndian.PutUint32(keys[4:], 1)
	binary.BigEndian.PutUint32(keys[8:], uint32(8+len(key)))
	copy(keys[12:], "mdta")
	copy(keys[16:], key)
	value := append([]byte{0, 0, 0, 1, 0, 0, 0, 0}, []byte(captured)...)
	// A recognized but unrelated cover item deliberately has no valid child header.
	items := append(videoBox("covr", []byte("synthetic opaque payload")), videoBox(string([]byte{0, 0, 0, 1}), videoBox("data", value))...)
	metadata := append(make([]byte, 4), videoBox("keys", keys)...)
	metadata = append(metadata, videoBox("ilst", items)...)
	data := append(videoBox("ftyp", []byte("isom\x00\x00\x00\x00isom")), videoBox("moov", videoBox("meta", metadata))...)
	path := filepath.Join(t.TempDir(), "synthetic.mp4")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	chain := NewChain(time.UTC, time.UTC)
	chain.RequiredFields = func(string) Fields { return Fields{"captured": true} }
	r, err := chain.Extract(context.Background(), path)
	if err != nil || r.ExtractionFailed || r.Captured == nil || r.Captured.Source != "qt:com.apple.quicktime.creationdate" || r.Captured.Time.Format(time.RFC3339) != captured {
		t.Fatalf("record=%+v error=%v", r, err)
	}
}
