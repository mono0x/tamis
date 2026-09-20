package meta

import (
	"context"
	"encoding/json"
	"io"
	"time"

	ext "github.com/evanoberholster/imagemeta"
)

type ImageExtProvider struct{ AssumeLocation *time.Location }

func (ImageExtProvider) Supports(format Format) bool {
	return format == FormatCR3 || format == FormatCRW || format == FormatRW2
}

func (ImageExtProvider) Provides() []string { return imageFields }

func (p ImageExtProvider) Extract(ctx context.Context, base *Record, reader io.ReadSeeker) (*Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := ext.Decode(contextReader{ctx, reader})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	base = base.clone()
	if base.fields.Has(FieldMake) {
		base.Make = data.CameraMake()
	}
	if base.fields.Has(FieldModel) {
		base.Model = data.IFD0.Model
	}
	if base.fields.Has(FieldLens) {
		base.LensModel = data.ExifIFD.LensModel
	}
	if base.fields.Has(FieldWidth) {
		base.Width = int(data.ExifIFD.PixelXDimension)
	}
	if base.fields.Has(FieldHeight) {
		base.Height = int(data.ExifIFD.PixelYDimension)
	}
	if base.fields.Has(FieldRaw) {
		base.Orientation = int(data.IFD0.Orientation)
	}
	if base.fields.Has(FieldISO) && data.ExifIFD.ISOSpeedRatings != 0 {
		value := int(data.ExifIFD.ISOSpeedRatings)
		base.ISO = &value
	}
	if base.fields.Has(FieldFNumber) && data.ExifIFD.FNumber != 0 {
		value := float64(data.ExifIFD.FNumber)
		base.FNumber = &value
	}
	if base.fields.Has(FieldRaw) && data.ExifIFD.ExposureTime != 0 {
		value := float64(data.ExifIFD.ExposureTime)
		base.ExposureTime = &value
	}
	if base.fields.Has(FieldFocal) && data.ExifIFD.FocalLength != 0 {
		value := float64(data.ExifIFD.FocalLength)
		base.FocalLength = &value
	}
	if base.fields.Has(FieldGPS) {
		if lat, lon := data.GPS.Latitude(), data.GPS.Longitude(); lat != 0 || lon != 0 {
			altitude := float64(data.GPS.Altitude())
			base.GPS = &GPS{Lat: lat, Lon: lon, Alt: &altitude}
		}
	}
	if base.fields.captured() {
		if captured := imageExtTimestamp(data.OriginalDate(), data.ExifIFD.OffsetTimeOriginal, p.AssumeLocation, "exif:DateTimeOriginal", "OffsetTimeOriginal"); captured != nil {
			if data.ExifIFD.HasSubSecTimeOriginal() {
				captured.Source = "exif:SubSecDateTimeOriginal"
				if captured.HasOffset {
					captured.Source += " + OffsetTimeOriginal"
				}
			}
			base.Captured = captured
		} else if captured := imageExtTimestamp(data.DigitizedDate(), data.ExifIFD.OffsetTimeDigitized, p.AssumeLocation, "exif:CreateDate", "OffsetTimeDigitized"); captured != nil {
			base.Captured = captured
		}
	}
	if base.fields.Has(FieldRaw) {
		if raw, marshalErr := json.Marshal(data); marshalErr == nil {
			_ = json.Unmarshal(raw, &base.Raw)
		}
	}
	return base, nil
}

func imageExtTimestamp(value time.Time, offset, assumed *time.Location, source, offsetSource string) *Timestamp {
	if value.IsZero() {
		return nil
	}
	if offset != nil {
		return &Timestamp{Time: value, HasOffset: true, Source: source + " + " + offsetSource}
	}
	if assumed == nil {
		assumed = time.Local
	}
	value = time.Date(value.Year(), value.Month(), value.Day(), value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), assumed)
	return &Timestamp{Time: value, HasOffset: false, Source: source}
}
