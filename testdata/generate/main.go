// Command generate creates tiny, synthetic media fixtures. It never reads real media.
package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
)

func main() {
	root := filepath.Join("testdata", "samples")
	must(os.MkdirAll(root, 0o755))
	must(os.WriteFile(filepath.Join(root, "meaningless.jpg"), []byte{0xff, 0xd8, 0xff, 0xd9}, 0o644))
	must(os.WriteFile(filepath.Join(root, "meaningless.png"), pngFixture(), 0o644))
	must(os.WriteFile(filepath.Join(root, "meaningless.mp4"), mp4Fixture(), 0o644))
}

func pngFixture() []byte {
	var out bytes.Buffer
	out.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr, 1)
	binary.BigEndian.PutUint32(ihdr[4:], 1)
	ihdr[8] = 8
	ihdr[9] = 2
	chunk(&out, "IHDR", ihdr)
	chunk(&out, "tEXt", []byte("Creation Time\x00Sat, 03 Feb 2001 04:05:06 +0000"))
	var pixels bytes.Buffer
	zw := zlib.NewWriter(&pixels)
	_, _ = zw.Write([]byte{0, 0, 0, 0})
	must(zw.Close())
	chunk(&out, "IDAT", pixels.Bytes())
	chunk(&out, "IEND", nil)
	return out.Bytes()
}
func mp4Fixture() []byte {
	var out bytes.Buffer
	box(&out, "ftyp", func(b *bytes.Buffer) { b.WriteString("isom"); write(b, uint32(0)); b.WriteString("isom") })
	box(&out, "moov", func(b *bytes.Buffer) {
		box(b, "mvhd", func(b *bytes.Buffer) {
			write(b, uint32(0))
			write(b, uint32(3063917106))
			write(b, uint32(3063917106))
			write(b, uint32(1000))
			write(b, uint32(1000))
			write(b, int32(0x10000))
			write(b, int16(0x100))
			write(b, int16(0))
			write(b, uint32(0))
			write(b, uint32(0))
			for _, v := range []int32{0x10000, 0, 0, 0, 0x10000, 0, 0, 0, 0x40000000} {
				write(b, v)
			}
			for range 6 {
				write(b, int32(0))
			}
			write(b, uint32(2))
		})
	})
	return out.Bytes()
}
func box(out *bytes.Buffer, typ string, body func(*bytes.Buffer)) {
	var payload bytes.Buffer
	body(&payload)
	write(out, uint32(payload.Len()+8))
	out.WriteString(typ)
	out.Write(payload.Bytes())
}
func chunk(out *bytes.Buffer, typ string, data []byte) {
	write(out, uint32(len(data)))
	out.WriteString(typ)
	out.Write(data)
	write(out, crc32.ChecksumIEEE(append([]byte(typ), data...)))
}
func write(w *bytes.Buffer, value any) { must(binary.Write(w, binary.BigEndian, value)) }
func must(err error) {
	if err != nil {
		panic(err)
	}
}
