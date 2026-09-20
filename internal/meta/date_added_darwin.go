//go:build darwin

package meta

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

func readDateAdded(file *os.File) (*time.Time, error) {
	attrs := unix.Attrlist{Bitmapcount: unix.ATTR_BIT_MAP_COUNT, Commonattr: unix.ATTR_CMN_RETURNED_ATTRS | unix.ATTR_CMN_ADDEDTIME, Fileattr: unix.ATTR_FILE_LINKCOUNT}
	// The packed result is a uint32 length, five uint32 attribute masks,
	// a 64-bit timespec, and a uint32 link count, all aligned to four bytes.
	var data [44]byte
	//lint:ignore SA1019 x/sys has no fgetattrlist wrapper; use the descriptor syscall without CGO.
	_, _, errno := syscall.Syscall6(unix.SYS_FGETATTRLIST, file.Fd(), uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), unix.FSOPT_REPORT_FULLSIZE, 0)
	runtime.KeepAlive(file)
	if errno == unix.ENOTSUP {
		return nil, nil
	}
	if errno != 0 {
		return nil, errno
	}
	return decodeDateAdded(data[:])
}

func decodeDateAdded(data []byte) (*time.Time, error) {
	if len(data) < 24 {
		return nil, fmt.Errorf("truncated attribute header")
	}
	length := binary.NativeEndian.Uint32(data[:4])
	if length < 24 || uint64(length) > uint64(len(data)) {
		return nil, fmt.Errorf("invalid attribute buffer length")
	}
	if binary.NativeEndian.Uint32(data[4:8])&unix.ATTR_CMN_ADDEDTIME == 0 || binary.NativeEndian.Uint32(data[16:20])&unix.ATTR_FILE_LINKCOUNT == 0 {
		return nil, nil
	}
	if length != 44 {
		return nil, fmt.Errorf("invalid date added attribute length")
	}
	// Unlinked and multiply linked files have no unambiguous containing directory.
	if binary.NativeEndian.Uint32(data[40:44]) != 1 {
		return nil, nil
	}
	seconds := int64(binary.NativeEndian.Uint64(data[24:32]))
	nanos := int64(binary.NativeEndian.Uint64(data[32:40]))
	if seconds == 0 && nanos == 0 {
		return nil, nil
	}
	if seconds < 0 || nanos < 0 || nanos >= int64(time.Second) {
		return nil, fmt.Errorf("invalid date added timestamp")
	}
	stamp := time.Unix(seconds, nanos)
	return &stamp, nil
}
