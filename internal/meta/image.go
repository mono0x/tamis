package meta

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/bep/imagemeta"
)

type ImageProvider struct{ AssumeLocation *time.Location }

func (ImageProvider) Supports(f Format) bool {
	return f.Kind() == KindImage && f != FormatCR3 && f != FormatCRW && f != FormatRW2
}

func (ImageProvider) Provides() []string { return imageFields }

func (p ImageProvider) Extract(ctx context.Context, base *Record, reader io.ReadSeeker) (*Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fmtID, ok := imageFormats[base.Format]
	if !ok {
		return nil, fmt.Errorf("unsupported image format %s", base.Format)
	}
	base = base.clone()
	fields := base.fields
	sources := imagemeta.Source(0)
	if fields.HasAny(exifFields...) {
		sources = imagemeta.EXIF | imagemeta.XMP
	}
	if fields.Has(FieldRaw) {
		sources |= imagemeta.IPTC
	}
	if fields.HasAny(FieldWidth, FieldHeight) {
		sources |= imagemeta.CONFIG
	}
	if sources == 0 {
		return base, nil
	}
	tags := imagemeta.Tags{}
	result, err := imagemeta.Decode(imagemeta.Options{R: contextReader{ctx, reader}, ImageFormat: fmtID, Sources: sources, ShouldHandleTag: fields.imageTag, Timeout: 10 * time.Second, HandleTag: func(t imagemeta.TagInfo) error { tags.Add(t); return nil }, Warnf: func(format string, args ...any) { base.Warnings = append(base.Warnings, fmt.Sprintf(format, args...)) }})
	if err != nil {
		return nil, err
	}
	base.Width, base.Height = result.ImageConfig.Width, result.ImageConfig.Height
	// Tags.All rebuilds and copies the whole tag map on every call, so take it once.
	all := tags.All()
	if fields.Has(FieldRaw) {
		for name, tag := range all {
			base.Raw[name] = tag.Value
		}
	}
	base.Make = stringTag(all, "Make")
	base.Model = stringTag(all, "Model")
	base.LensModel = stringTag(all, "LensModel")
	base.ISO = intTag(all, "ISO")
	base.FNumber = floatTag(all, "FNumber")
	base.ExposureTime = floatTag(all, "ExposureTime")
	base.FocalLength = floatTag(all, "FocalLength")
	if orientation := intTag(all, "Orientation"); orientation != nil {
		base.Orientation = *orientation
	}
	if lat, lon, e := tags.GetLatLong(); e == nil && (lat != 0 || lon != 0) {
		base.GPS = &GPS{Lat: lat, Lon: lon, Alt: floatTag(all, "GPSAltitude")}
	}
	if captured := resolveImageTime(tags, p.AssumeLocation); captured != nil {
		base.Captured = captured
	}
	return base, nil
}

var imageFormats = map[Format]imagemeta.ImageFormat{FormatJPEG: imagemeta.JPEG, FormatTIFF: imagemeta.TIFF, FormatPNG: imagemeta.PNG, FormatWebP: imagemeta.WebP, FormatHEIC: imagemeta.HEIF, FormatAVIF: imagemeta.AVIF, FormatDNG: imagemeta.DNG, FormatCR2: imagemeta.CR2, FormatNEF: imagemeta.NEF, FormatARW: imagemeta.ARW, FormatPEF: imagemeta.PEF}

var (
	exifOffsetLayouts = []string{"2006:01:02 15:04:05.999999999Z07:00", "2006:01:02 15:04:05Z07:00", time.RFC3339Nano, "2006-01-02T15:04:05-0700"}
	exifLocalLayouts  = []string{"2006:01:02 15:04:05.999999999", "2006:01:02 15:04:05", "2006-01-02T15:04:05"}
)

func resolveImageTime(tags imagemeta.Tags, loc *time.Location) *Timestamp {
	type candidate struct{ group, tag, offset, source string }
	items := []candidate{{"exif", "DateTimeOriginal", "OffsetTimeOriginal", "exif:DateTimeOriginal"}, {"exif", "CreateDate", "OffsetTimeDigitized", "exif:CreateDate"}, {"xmp", "CreateDate", "", "xmp:CreateDate"}, {"xmp", "DateCreated", "", "photoshop:DateCreated"}}
	for _, item := range items {
		var values map[string]imagemeta.TagInfo
		if item.group == "exif" {
			values = tags.EXIF()
		} else {
			values = tags.XMP()
		}
		tag, ok := values[item.tag]
		if !ok {
			continue
		}
		source := item.source
		offset := ""
		if item.offset != "" {
			if v, found := tags.EXIF()[item.offset]; found {
				offset = fmt.Sprint(v.Value)
			}
		}
		if t, hasOffset, ok := parseTimestamp(fmt.Sprint(tag.Value), offset, loc, exifOffsetLayouts, exifLocalLayouts); ok {
			if offset != "" {
				source += " + " + item.offset
			}
			return &Timestamp{Time: t, HasOffset: hasOffset, Source: source}
		}
	}
	return nil
}

func stringTag(tags map[string]imagemeta.TagInfo, name string) string {
	if v, ok := tags[name]; ok {
		return fmt.Sprint(v.Value)
	}
	return ""
}
func intTag(tags map[string]imagemeta.TagInfo, name string) *int {
	if v, ok := tags[name]; ok {
		if n, e := strconv.Atoi(fmt.Sprint(v.Value)); e == nil {
			return &n
		}
	}
	return nil
}
func floatTag(tags map[string]imagemeta.TagInfo, name string) *float64 {
	if v, ok := tags[name]; ok {
		switch n := v.Value.(type) {
		case interface{ Float64() float64 }:
			f := n.Float64()
			return &f
		case float64:
			return &n
		case float32:
			f := float64(n)
			return &f
		case int:
			f := float64(n)
			return &f
		}
		if f, e := strconv.ParseFloat(fmt.Sprint(v.Value), 64); e == nil {
			return &f
		}
	}
	return nil
}

// exifTags maps each tag ImageProvider reads to the vocabulary fields it feeds.
// GPS is a prefix match rather than a fixed set of tags, so it stays out.
var exifTags = map[string][]string{
	"DateTimeOriginal":    capturedFields,
	"OffsetTimeOriginal":  capturedFields,
	"CreateDate":          capturedFields,
	"OffsetTimeDigitized": capturedFields,
	"DateCreated":         capturedFields,
	"Make":                {FieldMake},
	"Model":               {FieldModel},
	"LensModel":           {FieldLens},
	"ISO":                 {FieldISO},
	"FNumber":             {FieldFNumber},
	"FocalLength":         {FieldFocal},
}

func (f Fields) imageTag(tag imagemeta.TagInfo) bool {
	if tag.Source == imagemeta.EXIF && !strings.HasPrefix(tag.Namespace, "IFD0") {
		return false
	}
	if f.Has(FieldRaw) {
		return true
	}
	if wanted, ok := exifTags[tag.Tag]; ok {
		return f.HasAny(wanted...)
	}
	return f.Has(FieldGPS) && strings.HasPrefix(tag.Tag, "GPS")
}
