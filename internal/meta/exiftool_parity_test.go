//go:build exiftool

package meta

import "testing"

// TestExiftoolParity is driven by scripts/verify-meta.sh. The production binary
// never imports os/exec; exiftool is only a development oracle.
func TestExiftoolParity(t *testing.T) {
	t.Skip("add licensed sample files under testdata/samples to enable parity fixtures")
}
