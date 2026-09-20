//go:build linux

package apply

import "golang.org/x/sys/unix"

// renameExclusive publishes the destination without replacing an existing entry.
func renameExclusive(source, dest string) error {
	return unix.Renameat2(unix.AT_FDCWD, source, unix.AT_FDCWD, dest, unix.RENAME_NOREPLACE)
}
