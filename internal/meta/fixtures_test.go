package meta

import (
	"context"
	"io"
	"path/filepath"
	"testing"
)

func TestSyntheticFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "samples")
	png, err := extractFixture(PNGTextProvider{}, filepath.Join(root, "meaningless.png"))
	if err != nil {
		t.Fatal(err)
	}
	if png.Captured == nil || png.Captured.Time.Year() != 2001 {
		t.Fatalf("unexpected PNG time: %#v", png.Captured)
	}
	video, err := extractFixture(VideoProvider{}, filepath.Join(root, "meaningless.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if video.Captured == nil || video.Captured.Source != "qt:mvhd.creation_time" {
		t.Fatalf("unexpected video time: %#v", video.Captured)
	}
}

func extractFixture(p Provider, path string) (*Record, error) {
	file, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	base, err := newRecord(file, path, nil)
	if err != nil {
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return p.Extract(context.Background(), base, file)
}
