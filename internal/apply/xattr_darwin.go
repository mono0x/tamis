//go:build darwin

package apply

import (
	"errors"
	"strings"

	"golang.org/x/sys/unix"
)

func copyXattrs(source, dest string) error {
	size, err := unix.Listxattr(source, nil)
	if errors.Is(err, unix.ENOTSUP) {
		return nil
	}
	if err != nil {
		return err
	}
	if size == 0 {
		return nil
	}
	names := make([]byte, size)
	size, err = unix.Listxattr(source, names)
	if err != nil {
		return err
	}
	for _, name := range strings.FieldsFunc(string(names[:size]), func(r rune) bool { return r == 0 }) {
		valueSize, err := unix.Getxattr(source, name, nil)
		if err != nil {
			return err
		}
		value := make([]byte, valueSize)
		if _, err = unix.Getxattr(source, name, value); err != nil {
			return err
		}
		if err = unix.Setxattr(dest, name, value, 0); err != nil {
			return err
		}
	}
	return nil
}
