//go:build !darwin

package apply

import (
	"os"
	"syscall"
)

func cloneFile(source *os.File, dest string) error { return syscall.ENOTSUP }
