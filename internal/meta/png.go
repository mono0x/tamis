package meta

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"
)

type PNGTextProvider struct{ AssumeLocation *time.Location }

const maxPNGTextSize = 1 << 20

func (PNGTextProvider) Supports(f Format) bool { return f == FormatPNG }

func (PNGTextProvider) Provides() []string { return capturedFields }

func (p PNGTextProvider) Extract(ctx context.Context, base *Record, reader io.ReadSeeker) (*Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r := base.clone()
	reader = contextReader{ctx, reader}
	signature := make([]byte, 8)
	if _, err := io.ReadFull(reader, signature); err != nil {
		return nil, err
	}
	remaining := base.Size - 8
	for chunks := 0; chunks < 10000; chunks++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if remaining < 12 {
			return nil, io.ErrUnexpectedEOF
		}
		var header [8]byte
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			return nil, err
		}
		length := binary.BigEndian.Uint32(header[:4])
		typ := string(header[4:])
		remaining -= 8
		if int64(length) > remaining-4 {
			return nil, io.ErrUnexpectedEOF
		}
		remaining -= int64(length) + 4
		if typ != "tEXt" && typ != "iTXt" {
			if _, err := reader.Seek(int64(length)+4, io.SeekCurrent); err != nil {
				return nil, err
			}
			if typ == "IEND" {
				return r, nil
			}
			continue
		}
		if length > maxPNGTextSize {
			return nil, fmt.Errorf("PNG text chunk exceeds %d bytes", maxPNGTextSize)
		}
		data := make([]byte, length)
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil, err
		}
		if _, err := reader.Seek(4, io.SeekCurrent); err != nil {
			return nil, err
		}
		keyword, value := pngText(typ, data)
		if strings.EqualFold(keyword, "Creation Time") {
			if tm, has, ok := parseTimestamp(value, "", p.AssumeLocation, pngOffsetLayouts, pngLocalLayouts); ok {
				r.Captured = &Timestamp{Time: tm, HasOffset: has, Source: "png:" + typ + ":Creation Time"}
				if base.fields.Has(FieldRaw) {
					r.Raw["Creation Time"] = value
				}
				return r, nil
			}
		}
	}
	return nil, fmt.Errorf("PNG exceeds 10000 chunks")
}

func pngText(typ string, data []byte) (string, string) {
	switch typ {
	case "tEXt":
		parts := strings.SplitN(string(data), "\x00", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	case "iTXt":
		keyword, rest, ok := bytes.Cut(data, []byte{0})
		if !ok || len(rest) < 2 || rest[0] > 1 || rest[1] != 0 {
			return "", ""
		}
		compressed := rest[0] == 1
		_, rest, ok = bytes.Cut(rest[2:], []byte{0})
		if !ok {
			return "", ""
		}
		_, text, ok := bytes.Cut(rest, []byte{0})
		if !ok {
			return "", ""
		}
		if !compressed {
			return string(keyword), string(text)
		}
		reader, err := zlib.NewReader(bytes.NewReader(text))
		if err != nil {
			return "", ""
		}
		defer func() { _ = reader.Close() }()
		value, err := io.ReadAll(io.LimitReader(reader, 1<<20))
		if err == nil {
			return string(keyword), string(value)
		}
	}
	return "", ""
}

var (
	pngOffsetLayouts = []string{time.RFC1123Z, time.RFC1123, time.RFC3339Nano, "2006-01-02T15:04:05-0700"}
	pngLocalLayouts  = []string{"2006-01-02 15:04:05", "2006:01:02 15:04:05"}
)
