package meta

import "slices"

// The metadata vocabulary: what rule expressions and templates call each value,
// what Fields is keyed by, and what filters ask providers for. meta owns the
// names because Fields lives here and rule depends on meta, not the other way
// around; rule.Env's expr: tags must spell exactly this set, which
// TestEnvTagsMatchVocabulary enforces.
const (
	FieldPath           = "path"
	FieldRelPath        = "relpath"
	FieldBasename       = "basename"
	FieldExt            = "ext"
	FieldDir            = "dir"
	FieldSize           = "size"
	FieldKind           = "kind"
	FieldFormat         = "format"
	FieldMIME           = "mime"
	FieldCaptured       = "captured"
	FieldCapturedSource = "captured_source"
	FieldMTime          = "mtime"
	FieldCTime          = "ctime"
	FieldDateAdded      = "date_added"
	FieldAge            = "age"
	FieldAgeHours       = "age_hours"
	FieldNow            = "now"
	FieldMake           = "make"
	FieldModel          = "model"
	FieldLens           = "lens"
	FieldISO            = "iso"
	FieldFNumber        = "fnumber"
	FieldFocal          = "focal"
	FieldWidth          = "width"
	FieldHeight         = "height"
	FieldGPS            = "gps"
	FieldDuration       = "duration"
	FieldRaw            = "raw"
	FieldCapture        = "capture"
	FieldSeq            = "seq"
)

// Vocabulary is every field name above.
var Vocabulary = []string{
	FieldPath, FieldRelPath, FieldBasename, FieldExt, FieldDir, FieldSize,
	FieldKind, FieldFormat, FieldMIME, FieldCaptured, FieldCapturedSource,
	FieldMTime, FieldCTime, FieldDateAdded, FieldAge, FieldAgeHours, FieldNow,
	FieldMake, FieldModel, FieldLens, FieldISO, FieldFNumber, FieldFocal,
	FieldWidth, FieldHeight, FieldGPS, FieldDuration, FieldRaw, FieldCapture,
	FieldSeq,
}

// The field sets providers declare through Provides. exifFields is the part of
// imageFields that needs the EXIF and XMP sources decoded, and mediaFields is
// the union, which decides whether the file format has to be sniffed at all.
var (
	capturedFields = []string{FieldCaptured, FieldCapturedSource, FieldRaw}
	exifFields     = slices.Concat(capturedFields, []string{FieldMake, FieldModel, FieldLens, FieldISO, FieldFNumber, FieldFocal, FieldGPS})
	imageFields    = slices.Concat(exifFields, []string{FieldWidth, FieldHeight})
	videoFields    = slices.Concat(capturedFields, []string{FieldDuration})
	mediaFields    = slices.Concat(imageFields, videoFields)
)

// Fields names the metadata requested by filters. Nil requests all metadata for diagnostics.
type Fields map[string]bool

func (f Fields) Has(name string) bool { return f == nil || f[name] }

// HasAny reports whether any of names was requested.
func (f Fields) HasAny(names ...string) bool {
	if f == nil {
		return true
	}
	return slices.ContainsFunc(names, func(name string) bool { return f[name] })
}

func (f Fields) captured() bool { return f.HasAny(capturedFields...) }

func (f Fields) media() bool { return f.HasAny(mediaFields...) }
