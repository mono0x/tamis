//go:build darwin

package apply

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCopyUsesNativeClone(t *testing.T) {
	testCopy(t, func(source *os.File, dest string) error {
		err := cloneFile(source, dest)
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EXDEV) {
			t.Skipf("test filesystem does not support cloning: %v", err)
		}
		if err != nil {
			t.Fatal(err)
		}
		return nil
	})
}
