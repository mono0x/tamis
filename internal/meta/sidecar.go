package meta

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type VideoSidecarProvider struct{ AssumeLocation *time.Location }

const maxSidecarSize = 8 << 20

func (VideoSidecarProvider) Supports(format Format) bool { return format.Kind() == KindVideo }

func (VideoSidecarProvider) Provides() []string { return capturedFields }

func (p VideoSidecarProvider) Extract(ctx context.Context, base *Record, _ io.ReadSeeker) (*Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	stem := strings.TrimSuffix(base.Path, filepath.Ext(base.Path))
	r := base.clone()
	for _, sidecar := range []string{stem + ".xmp", stem + ".XMP"} {
		file, readErr := openFile(sidecar)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			return nil, fmt.Errorf("sidecar %s: %w", sidecar, readErr)
		}
		data, readErr := io.ReadAll(io.LimitReader(contextReader{ctx, file}, maxSidecarSize+1))
		_ = file.Close()
		if readErr != nil {
			return nil, fmt.Errorf("sidecar %s: %w", sidecar, readErr)
		}
		if len(data) > maxSidecarSize {
			return nil, fmt.Errorf("sidecar exceeds %d bytes: %s", maxSidecarSize, sidecar)
		}
		match := xmpCreateDate.FindSubmatch(data)
		if len(match) == 2 {
			if tm, hasOffset, ok := parseVideoTime(string(match[1]), p.AssumeLocation); ok {
				r.Captured = &Timestamp{Time: tm, HasOffset: hasOffset, Source: "sidecar:xmp:CreateDate"}
				return r, nil
			}
		}
	}
	for _, extension := range []string{".THM", ".thm", ".JPG", ".jpg", ".JPEG", ".jpeg"} {
		sidecar := stem + extension
		record, extractErr := p.extractImage(ctx, sidecar)
		if extractErr == nil && record.Captured != nil {
			captured := *record.Captured
			captured.Source = fmt.Sprintf("sidecar:%s:%s", filepath.Base(sidecar), captured.Source)
			r.Captured = &captured
			return r, nil
		}
	}
	return r, nil
}

func (p VideoSidecarProvider) extractImage(ctx context.Context, path string) (*Record, error) {
	file, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	base, err := newRecord(file, path, Fields{FieldCaptured: true})
	if err != nil {
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return ImageProvider(p).Extract(ctx, base, file)
}

var xmpCreateDate = regexp.MustCompile(`(?i)(?:xmp:CreateDate=["']|<xmp:CreateDate>)([^"'<]+)`)
