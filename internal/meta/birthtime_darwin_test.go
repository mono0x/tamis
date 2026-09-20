//go:build darwin

package meta

import (
	"syscall"
	"testing"
	"time"
)

func TestBirthTimeUnavailable(t *testing.T) {
	if got := birthTime(&syscall.Stat_t{}); !got.IsZero() {
		t.Fatalf("unavailable birthtime would suppress mtime fallback: %s", got)
	}
	for _, stamp := range []syscall.Timespec{{Sec: 1}, {Nsec: 1}, {Sec: 1000000000, Nsec: 123}} {
		got := birthTime(&syscall.Stat_t{Birthtimespec: stamp})
		if !got.Equal(time.Unix(stamp.Sec, stamp.Nsec)) {
			t.Fatalf("birthtime changed: %s", got)
		}
	}
}
