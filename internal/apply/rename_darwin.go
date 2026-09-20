//go:build darwin

package apply

import "golang.org/x/sys/unix"

// renameExclusive publishes the destination without replacing an existing entry.
func renameExclusive(source, dest string) error {
	return unix.RenamexNp(source, dest, unix.RENAME_EXCL)
}
