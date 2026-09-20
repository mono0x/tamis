package meta

import "testing"

func TestDetectFormatBytes(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		path string
		want Format
	}{
		{"jpeg", []byte{0xff, 0xd8, 0xff}, "x.bin", FormatJPEG},
		{"png", []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, "x", FormatPNG},
		{"mov", append([]byte{0, 0, 0, 20, 'f', 't', 'y', 'p'}, []byte("qt  ")...), "x", FormatMOV},
		{"heic", append([]byte{0, 0, 0, 20, 'f', 't', 'y', 'p'}, []byte("heic")...), "x", FormatHEIC},
		{"m4v", append([]byte{0, 0, 0, 20, 'f', 't', 'y', 'p'}, []byte("M4V ")...), "x", FormatMP4},
		{"arw", []byte{'I', 'I', 42, 0, 0, 0, 0, 0}, "x.arw", FormatARW},
		{"rw2", []byte{'I', 'I', 0x55, 0}, "x.bin", FormatRW2},
		{"crw", []byte{'I', 'I', 0x1a, 0, 0, 0, 'H', 'E', 'A', 'P', 'C', 'C', 'D', 'R'}, "x.bin", FormatCRW},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectFormatBytes(tc.data, tc.path); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}
