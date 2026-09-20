// Package meta extracts image and video metadata through pure-Go providers.
package meta

import (
	"context"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"time"
)

type Kind string

const (
	KindImage Kind = "image"
	KindVideo Kind = "video"
	KindOther Kind = "other"
)

type Format string

const (
	FormatUnknown Format = "UNKNOWN"
	FormatJPEG    Format = "JPEG"
	FormatTIFF    Format = "TIFF"
	FormatPNG     Format = "PNG"
	FormatWebP    Format = "WEBP"
	FormatHEIC    Format = "HEIC"
	FormatAVIF    Format = "AVIF"
	FormatDNG     Format = "DNG"
	FormatCR2     Format = "CR2"
	FormatCR3     Format = "CR3"
	FormatCRW     Format = "CRW"
	FormatNEF     Format = "NEF"
	FormatPEF     Format = "PEF"
	FormatARW     Format = "ARW"
	FormatRW2     Format = "RW2"
	FormatMOV     Format = "MOV"
	FormatMP4     Format = "MP4"
)

type Timestamp struct {
	Time      time.Time `json:"time"`
	HasOffset bool      `json:"has_offset"`
	Source    string    `json:"source"`
}

// parseTimestamp tries offsetLayouts against value joined with offset, which
// yields a time that carries a real zone, then falls back to localLayouts
// against value alone, interpreted in loc. Metadata strings routinely arrive
// NUL-padded, so every source is trimmed the same way.
func parseTimestamp(value, offset string, loc *time.Location, offsetLayouts, localLayouts []string) (time.Time, bool, bool) {
	if loc == nil {
		loc = time.Local
	}
	value = strings.TrimSpace(strings.TrimRight(value, "\x00"))
	for _, layout := range offsetLayouts {
		if t, err := time.Parse(layout, value+offset); err == nil {
			return t, true, true
		}
	}
	for _, layout := range localLayouts {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t, false, true
		}
	}
	return time.Time{}, false, false
}

type GPS struct {
	Lat float64  `json:"lat" expr:"lat"`
	Lon float64  `json:"lon" expr:"lon"`
	Alt *float64 `json:"alt,omitempty" expr:"alt"`
}

type Record struct {
	fields           Fields
	SourceInfo       os.FileInfo    `json:"-"`
	Path             string         `json:"path"`
	RelPath          string         `json:"relpath,omitempty"`
	Size             int64          `json:"size"`
	FileMTime        time.Time      `json:"mtime"`
	FileCTime        time.Time      `json:"ctime"`
	DateAdded        *time.Time     `json:"date_added,omitempty"`
	Kind             Kind           `json:"kind"`
	Format           Format         `json:"format"`
	MIMEType         string         `json:"mime"`
	Captured         *Timestamp     `json:"captured,omitempty"`
	Make             string         `json:"make,omitempty"`
	Model            string         `json:"model,omitempty"`
	LensModel        string         `json:"lens_model,omitempty"`
	ISO              *int           `json:"iso,omitempty"`
	FNumber          *float64       `json:"f_number,omitempty"`
	ExposureTime     *float64       `json:"exposure_time,omitempty"`
	FocalLength      *float64       `json:"focal_length,omitempty"`
	Width            int            `json:"width,omitempty"`
	Height           int            `json:"height,omitempty"`
	Orientation      int            `json:"orientation,omitempty"`
	GPS              *GPS           `json:"gps,omitempty"`
	Duration         *time.Duration `json:"duration,omitempty"`
	Raw              map[string]any `json:"raw,omitempty"`
	Warnings         []string       `json:"warnings,omitempty"`
	ExtractionFailed bool           `json:"extraction_failed,omitempty"`
}

// Provider derives metadata from base, the filesystem-level record the chain
// builds once per file. Providers must treat base as read-only and work on
// clone(). The reader is positioned at the start of the source for each call;
// providers must not close it or retain it after returning.
type Provider interface {
	Supports(format Format) bool
	// Provides names the record fields the provider can populate. The chain
	// skips a provider when the caller requested none of them.
	Provides() []string
	Extract(ctx context.Context, base *Record, reader io.ReadSeeker) (*Record, error)
}

func (r *Record) clone() *Record {
	c := *r
	c.Raw = maps.Clone(r.Raw)
	c.Warnings = slices.Clone(r.Warnings)
	return &c
}

func (f Format) Kind() Kind {
	switch f {
	case FormatJPEG, FormatTIFF, FormatPNG, FormatWebP, FormatHEIC, FormatAVIF, FormatDNG, FormatCR2, FormatCR3, FormatCRW, FormatNEF, FormatPEF, FormatARW, FormatRW2:
		return KindImage
	case FormatMOV, FormatMP4:
		return KindVideo
	default:
		return KindOther
	}
}
