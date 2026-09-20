//go:build !darwin

package meta

import (
	"syscall"
	"time"
)

func birthTime(*syscall.Stat_t) time.Time { return time.Time{} }
