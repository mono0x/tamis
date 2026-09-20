package meta

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

func TestPNGTextProvider(t *testing.T) {
	var b bytes.Buffer
	b.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	writeChunk(&b, "tEXt", []byte("Creation Time\x002026-05-03T14:22:31+09:00"))
	writeChunk(&b, "IEND", nil)
	path := filepath.Join(t.TempDir(), "a.png")
	if err := os.WriteFile(path, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := extractFixture(PNGTextProvider{}, path)
	if err != nil {
		t.Fatal(err)
	}
	if r.Captured == nil || r.Captured.Source != "png:tEXt:Creation Time" {
		t.Fatalf("unexpected: %#v", r.Captured)
	}
}
func writeChunk(b *bytes.Buffer, typ string, data []byte) {
	_ = binary.Write(b, binary.BigEndian, uint32(len(data)))
	b.WriteString(typ)
	b.Write(data)
	_ = binary.Write(b, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(typ), data...)))
}
