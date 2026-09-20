#!/bin/sh
set -eu

command -v exiftool >/dev/null
command -v jq >/dev/null

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

go run ./cmd/tamis scan testdata/samples >"$tmp_dir/tamis.ndjson"
exiftool -j -G -n -r testdata/samples >"$tmp_dir/exiftool.json"

jq -s 'map({path: .path, captured: .captured.time, make, model, iso, f_number, exposure_time, focal_length, gps, width, height}) | sort_by(.path)' \
  "$tmp_dir/tamis.ndjson" >"$tmp_dir/tamis.json"
jq 'map({path: .SourceFile, captured: (.EXIF.DateTimeOriginal // .QuickTime.CreationDate // null), make: (.EXIF.Make // null), model: (.EXIF.Model // null), iso: (.EXIF.ISO // null), f_number: (.EXIF.FNumber // null), exposure_time: (.EXIF.ExposureTime // null), focal_length: (.EXIF.FocalLength // null), gps: (if .EXIF.GPSLatitude then {lat: .EXIF.GPSLatitude, lon: .EXIF.GPSLongitude} else null end), width: (.File.ImageWidth // null), height: (.File.ImageHeight // null)}) | sort_by(.path)' \
  "$tmp_dir/exiftool.json" >"$tmp_dir/exiftool-normalized.json"

diff -u "$tmp_dir/exiftool-normalized.json" "$tmp_dir/tamis.json"
