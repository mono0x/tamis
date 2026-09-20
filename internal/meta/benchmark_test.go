package meta

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkChain(b *testing.B) {
	for _, format := range []string{"jpg", "png", "mp4"} {
		for _, tc := range []struct {
			name   string
			fields Fields
		}{
			{"all", nil},
			{"captured", Fields{FieldCaptured: true}},
			{"filesystem", Fields{FieldMTime: true}},
		} {
			b.Run(format+"/"+tc.name, func(b *testing.B) {
				path := filepath.Join("..", "..", "testdata", "samples", "meaningless."+format)
				chain := NewChain(time.UTC, time.UTC)
				chain.RequiredFields = func(string) Fields { return tc.fields }
				b.ReportAllocs()
				for b.Loop() {
					record, err := chain.Extract(context.Background(), path)
					if err != nil || record.ExtractionFailed {
						b.Fatalf("synthetic extraction failed: %v", err)
					}
				}
			})
		}
	}
}
