//go:build darwin

package apply

import (
	"os"

	"golang.org/x/sys/unix"
)

func cloneFile(source *os.File, dest string) error {
	return unix.Fclonefileat(int(source.Fd()), unix.AT_FDCWD, dest, unix.CLONE_NOOWNERCOPY)
}
