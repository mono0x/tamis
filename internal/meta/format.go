package meta

import (
	"bytes"
	"encoding/binary"
	"io"
	"path/filepath"
	"strings"
)

func detectFormat(reader io.Reader, path string) (Format, error) {
	buf := make([]byte, 512)
	n, err := io.ReadFull(reader, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return FormatUnknown, err
	}
	return DetectFormatBytes(buf[:n], path), nil
}

func DetectFormatBytes(b []byte, path string) Format {
	if len(b) >= 12 && string(b[4:8]) == "ftyp" {
		brands := []string{string(b[8:12])}
		for i := 16; i+4 <= len(b) && i < 128; i += 4 {
			brands = append(brands, string(b[i:i+4]))
		}
		for _, brand := range brands {
			switch brand {
			case "crx ":
				return FormatCR3
			case "avif", "avis":
				return FormatAVIF
			case "heic", "heix", "mif1", "msf1":
				return FormatHEIC
			case "qt  ":
				return FormatMOV
			case "isom", "mp42", "mp41", "iso2", "M4V ", "M4VH", "M4VP":
				return FormatMP4
			}
		}
	}
	if bytes.HasPrefix(b, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return FormatPNG
	}
	if bytes.HasPrefix(b, []byte{0xff, 0xd8, 0xff}) {
		return FormatJPEG
	}
	if bytes.HasPrefix(b, []byte{'I', 'I', 0x1a, 0, 0, 0, 'H', 'E', 'A', 'P', 'C', 'C', 'D', 'R'}) {
		return FormatCRW
	}
	if bytes.HasPrefix(b, []byte{'I', 'I', 0x55, 0}) {
		return FormatRW2
	}
	if len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return FormatWebP
	}
	if bytes.HasPrefix(b, []byte{'I', 'I', 42, 0}) || bytes.HasPrefix(b, []byte{'M', 'M', 0, 42}) {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
		switch ext {
		case "dng":
			return FormatDNG
		case "cr2":
			return FormatCR2
		case "nef":
			return FormatNEF
		case "pef":
			return FormatPEF
		case "arw":
			return FormatARW
		case "rw2":
			return FormatRW2
		}
		if len(b) >= 10 && binary.LittleEndian.Uint16(b[8:10]) == 0x4352 {
			return FormatCR2
		}
		return FormatTIFF
	}
	return FormatUnknown
}
