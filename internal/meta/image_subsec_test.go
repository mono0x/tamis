package meta

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestImageIgnoresSeparateEXIFSubseconds(t *testing.T) {
	for _, format := range []string{"jpg", "tiff", "png", "webp"} {
		for _, digitized := range []bool{false, true} {
			for _, fraction := range []string{"123", "001", "000000001"} {
				t.Run(format+"/"+map[bool]string{true: "digitized/", false: "original/"}[digitized]+fraction, func(t *testing.T) {
					data := subsecondImage(t, format, fraction, digitized)
					path := filepath.Join(t.TempDir(), "synthetic."+format)
					if err := os.WriteFile(path, data, 0o600); err != nil {
						t.Fatal(err)
					}
					r, err := extractFixture(ImageProvider{AssumeLocation: time.UTC}, path)
					if err != nil {
						t.Fatal(err)
					}
					want, err := time.Parse(time.RFC3339, "2001-02-03T04:05:06-04:00")
					if err != nil {
						t.Fatal(err)
					}
					if r.Captured == nil || !r.Captured.Time.Equal(want) || !r.Captured.HasOffset {
						t.Fatalf("captured=%+v want=%s", r.Captured, want)
					}
					source := "exif:DateTimeOriginal + OffsetTimeOriginal"
					if digitized {
						source = "exif:CreateDate + OffsetTimeDigitized"
					}
					if r.Captured.Source != source {
						t.Fatalf("source=%s", r.Captured.Source)
					}
				})
			}
		}
	}
}

func subsecondImage(t *testing.T, format, fraction string, digitized bool) []byte {
	t.Helper()
	var tiff bytes.Buffer
	put := func(value any) {
		if err := binary.Write(&tiff, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	tiff.WriteString("II")
	put(uint16(42))
	put(uint32(8))
	put(uint16(1))
	put(uint16(0x8769))
	put(uint16(4))
	put(uint32(1))
	put(uint32(26))
	put(uint32(0))
	date, subsec, offset := uint16(0x9003), uint16(0x9291), uint16(0x9011)
	if digitized {
		date, subsec, offset = 0x9004, 0x9292, 0x9012
	}
	tags := []struct {
		id    uint16
		value string
	}{{date, "2001:02:03 04:05:06\x00"}, {offset, "-04:00\x00"}, {subsec, fraction + "\x00"}}
	put(uint16(len(tags)))
	var values bytes.Buffer
	start := uint32(26 + 2 + len(tags)*12 + 4)
	for _, tag := range tags {
		put(tag.id)
		put(uint16(2))
		put(uint32(len(tag.value)))
		if len(tag.value) <= 4 {
			var inline [4]byte
			copy(inline[:], tag.value)
			tiff.Write(inline[:])
		} else {
			put(start + uint32(values.Len()))
			values.WriteString(tag.value)
		}
	}
	put(uint32(0))
	tiff.Write(values.Bytes())
	var image bytes.Buffer
	switch format {
	case "tiff":
		return tiff.Bytes()
	case "jpg":
		image.Write([]byte{0xff, 0xd8, 0xff, 0xe1})
		if err := binary.Write(&image, binary.BigEndian, uint16(tiff.Len()+8)); err != nil {
			t.Fatal(err)
		}
		image.WriteString("Exif\x00\x00")
		image.Write(tiff.Bytes())
		image.Write([]byte{0xff, 0xd9})
	case "png":
		image.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
		header := make([]byte, 13)
		binary.BigEndian.PutUint32(header, 1)
		binary.BigEndian.PutUint32(header[4:], 1)
		header[8], header[9] = 8, 2
		writeChunk(&image, "IHDR", header)
		writeChunk(&image, "eXIf", tiff.Bytes())
		writeChunk(&image, "IEND", nil)
	case "webp":
		image.WriteString("RIFF")
		size := uint32(4 + 18 + 8 + tiff.Len() + tiff.Len()%2)
		if err := binary.Write(&image, binary.LittleEndian, size); err != nil {
			t.Fatal(err)
		}
		image.WriteString("WEBPVP8X")
		image.Write([]byte{10, 0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0})
		image.WriteString("EXIF")
		if err := binary.Write(&image, binary.LittleEndian, uint32(tiff.Len())); err != nil {
			t.Fatal(err)
		}
		image.Write(tiff.Bytes())
		if tiff.Len()%2 != 0 {
			image.WriteByte(0)
		}
	default:
		t.Fatalf("unknown synthetic format %s", format)
	}
	return image.Bytes()
}
