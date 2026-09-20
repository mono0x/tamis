package meta

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestVideoSidecarTimestamp(t *testing.T) {
	for _, tc := range []struct {
		name, lower, upper, want string
		hasOffset                bool
	}{
		{name: "absent"},
		{name: "invalid", lower: `<xmp:CreateDate>invalid</xmp:CreateDate>`},
		{name: "attribute", lower: `<rdf xmp:CreateDate="2001-02-03T04:05:06+09:00"/>`, want: "2001-02-03T04:05:06+09:00", hasOffset: true},
		{name: "assumed timezone", upper: `<xmp:CreateDate>2001-02-03T04:05:06</xmp:CreateDate>`, want: "2001-02-03T04:05:06+09:00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := chainFixture(t)
			for extension, data := range map[string]string{".xmp": tc.lower, ".XMP": tc.upper} {
				if data != "" {
					if err := os.WriteFile(path+extension, []byte(data), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			base, err := NewRecord(path)
			if err != nil {
				t.Fatal(err)
			}
			before := base.clone()
			r, err := (VideoSidecarProvider{AssumeLocation: time.FixedZone("synthetic", 9*60*60)}).Extract(context.Background(), base, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(base, before) {
				t.Fatal("sidecar extraction changed base")
			}
			if tc.want == "" {
				if r.Captured != nil {
					t.Fatalf("unexpected timestamp: %+v", r.Captured)
				}
			} else if r.Captured == nil || r.Captured.Time.Format(time.RFC3339) != tc.want || r.Captured.HasOffset != tc.hasOffset || r.Captured.Source != "sidecar:xmp:CreateDate" {
				t.Fatalf("unexpected timestamp: %+v", r.Captured)
			}
		})
	}
}

func TestVideoChainFallsBackToSidecar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "synthetic.mp4")
	// A valid ftyp box with no movie timestamp must allow sidecar extraction.
	if err := os.WriteFile(path, []byte{0, 0, 0, 20, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0, 'i', 's', 'o', 'm'}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".mp4")+".xmp", []byte(`<rdf xmp:CreateDate="2001-02-03T04:05:06Z"/>`), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := NewChain(time.UTC, time.UTC).Extract(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if r.ExtractionFailed || r.Captured == nil || r.Captured.Source != "sidecar:xmp:CreateDate" || r.Captured.Time.Format(time.RFC3339) != "2001-02-03T04:05:06Z" {
		t.Fatalf("sidecar fallback failed: %+v", r)
	}
}

func TestVideoChainFallsBackToImageSidecar(t *testing.T) {
	for _, extension := range []string{".THM", ".jpg"} {
		t.Run(extension, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "synthetic.mp4")
			if err := os.WriteFile(path, videoBox("ftyp", []byte("isom\x00\x00\x00\x00isom")), 0o600); err != nil {
				t.Fatal(err)
			}
			sidecar := strings.TrimSuffix(path, ".mp4") + extension
			if err := os.WriteFile(sidecar, subsecondImage(t, "jpg", "123456", false), 0o600); err != nil {
				t.Fatal(err)
			}
			chain := NewChain(time.UTC, time.UTC)
			chain.RequiredFields = func(string) Fields { return Fields{FieldCaptured: true} }
			record, err := chain.Extract(context.Background(), path)
			if err != nil || record.ExtractionFailed || record.Captured == nil || !strings.EqualFold(record.Captured.Source, "sidecar:synthetic"+extension+":exif:DateTimeOriginal + OffsetTimeOriginal") {
				t.Fatalf("image sidecar extraction failed: record=%+v err=%v", record, err)
			}
		})
	}
}
