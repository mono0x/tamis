//go:build darwin

package meta

import (
	"syscall"
	"time"
)

func birthTime(stat *syscall.Stat_t) time.Time {
	if stat.Birthtimespec.Sec == 0 && stat.Birthtimespec.Nsec == 0 {
		return time.Time{}
	}
	return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
}
