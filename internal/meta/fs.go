package meta

import (
	"context"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type FSProvider struct{}

func (FSProvider) Supports(Format) bool { return true }

func (FSProvider) Provides() []string { return capturedFields }

// NewRecord gathers the filesystem facts every provider starts from. The chain
// builds it once per file so providers do not re-stat and re-sniff the format.
func NewRecord(path string) (*Record, error) {
	file, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return newRecord(file, path, nil)
}

func newRecord(file *os.File, path string, fields Fields) (*Record, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", path)
	}
	f := FormatUnknown
	if fields.media() || fields.HasAny(FieldKind, FieldFormat, FieldMIME) {
		f, err = detectFormat(file, path)
		if err != nil {
			return nil, err
		}
	}
	r := &Record{fields: fields, Path: path, SourceInfo: info, Size: info.Size(), FileMTime: info.ModTime(), FileCTime: birthTimeOf(info), Kind: f.Kind(), Format: f, MIMEType: mimeFor(f)}
	if fields.Has(FieldRaw) {
		r.Raw = map[string]any{}
	}
	if r.FileCTime.IsZero() {
		r.FileCTime = r.FileMTime
	}
	if fields.Has(FieldDateAdded) {
		r.DateAdded, err = readDateAdded(file)
		if err != nil {
			r.Warnings = append(r.Warnings, fmt.Sprintf("date_added: %v", err))
		}
	}
	return r, nil
}

func birthTimeOf(info os.FileInfo) time.Time {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}
	}
	return birthTime(stat)
}

// Nonblocking opens prevent a FIFO substituted after the walk from hanging a
// worker. Do not follow symlinks into files outside the scanned source.
func openFile(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	// Buffered reads can wait in Go's poller even with O_NONBLOCK. Reject
	// non-seekable inputs before reading, without re-statting in providers.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

// Extract is the last-resort timestamp: the birth time when the filesystem
// records one, the modification time otherwise.
func (FSProvider) Extract(ctx context.Context, base *Record, _ io.ReadSeeker) (*Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r := base.clone()
	if birth := birthTimeOf(r.SourceInfo); !birth.IsZero() {
		r.Captured = &Timestamp{Time: birth, HasOffset: true, Source: "fs:birthtime"}
	} else if !r.FileMTime.IsZero() {
		r.Captured = &Timestamp{Time: r.FileMTime, HasOffset: true, Source: "fs:mtime"}
	}
	return r, nil
}

func mimeFor(f Format) string {
	switch f {
	case FormatJPEG:
		return "image/jpeg"
	case FormatPNG:
		return "image/png"
	case FormatWebP:
		return "image/webp"
	case FormatHEIC:
		return "image/heic"
	case FormatAVIF:
		return "image/avif"
	case FormatMOV:
		return "video/quicktime"
	case FormatMP4:
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}
