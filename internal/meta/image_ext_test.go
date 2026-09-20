package meta

import (
	"testing"
	"time"
)

func TestImageExtTimestampUsesAssumedLocationWithoutOffset(t *testing.T) {
	location := time.FixedZone("synthetic", 9*60*60)
	input := time.Date(2001, 2, 3, 4, 5, 6, 7, time.UTC)
	got := imageExtTimestamp(input, nil, location, "exif:DateTimeOriginal", "OffsetTimeOriginal")
	if got == nil || got.HasOffset || got.Time.Location() != location {
		t.Fatalf("imageExtTimestamp() = %#v", got)
	}
	if got.Time.Hour() != input.Hour() {
		t.Fatalf("hour = %d, want %d", got.Time.Hour(), input.Hour())
	}
}

func TestImageExtTimestampPreservesOffset(t *testing.T) {
	offset := time.FixedZone("synthetic", 2*60*60)
	input := time.Date(2001, 2, 3, 4, 5, 6, 7, offset)
	got := imageExtTimestamp(input, offset, time.UTC, "exif:DateTimeOriginal", "OffsetTimeOriginal")
	if got == nil || !got.HasOffset || !got.Time.Equal(input) {
		t.Fatalf("imageExtTimestamp() = %#v", got)
	}
}
