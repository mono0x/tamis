//go:build darwin

package meta

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/mono0x/tamis/internal/apply"
	"github.com/mono0x/tamis/internal/plan"

	"golang.org/x/sys/unix"
)

func TestDecodeDateAdded(t *testing.T) {
	stamp := time.Date(2001, 2, 3, 4, 5, 6, 123, time.UTC)
	for _, tc := range []struct {
		name    string
		length  uint32
		mask    uint32
		seconds int64
		nanos   int64
		present bool
		wantErr bool
	}{
		{"available", 44, unix.ATTR_CMN_ADDEDTIME, stamp.Unix(), 123, true, false},
		{"unsupported", 24, 0, stamp.Unix(), 123, false, false},
		{"zero", 44, unix.ATTR_CMN_ADDEDTIME, 0, 0, false, false},
		{"short header", 4, 0, 0, 0, false, true},
		{"oversized", 45, unix.ATTR_CMN_ADDEDTIME, stamp.Unix(), 123, false, true},
		{"short timestamp", 32, unix.ATTR_CMN_ADDEDTIME, stamp.Unix(), 123, false, true},
		{"negative seconds", 44, unix.ATTR_CMN_ADDEDTIME, -1, 0, false, true},
		{"negative nanos", 44, unix.ATTR_CMN_ADDEDTIME, stamp.Unix(), -1, false, true},
		{"overflow nanos", 44, unix.ATTR_CMN_ADDEDTIME, stamp.Unix(), 1e9, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var data [44]byte
			binary.NativeEndian.PutUint32(data[:4], tc.length)
			binary.NativeEndian.PutUint32(data[4:8], tc.mask)
			binary.NativeEndian.PutUint32(data[16:20], unix.ATTR_FILE_LINKCOUNT)
			binary.NativeEndian.PutUint64(data[24:32], uint64(tc.seconds))
			binary.NativeEndian.PutUint64(data[32:40], uint64(tc.nanos))
			binary.NativeEndian.PutUint32(data[40:44], 1)
			got, err := decodeDateAdded(data[:])
			if (err != nil) != tc.wantErr || (got != nil) != tc.present {
				t.Fatalf("got %v, %v", got, err)
			}
			if got != nil && !got.Equal(stamp) {
				t.Fatalf("timestamp changed: %s", got)
			}
		})
	}
	if _, err := decodeDateAdded(make([]byte, 23)); err == nil {
		t.Fatal("truncated buffer accepted")
	}
}

func TestDateAddedSurvivesMetadataExtraction(t *testing.T) {
	path := chainFixture(t)
	stamp := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	setSyntheticDateAdded(t, path, stamp)
	r, err := NewChain(time.UTC, time.UTC).Extract(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if r.DateAdded == nil || !r.DateAdded.Equal(stamp) {
		t.Fatalf("date added unavailable or changed: %v; warnings: %v", r.DateAdded, r.Warnings)
	}
	if r.Captured == nil || r.Captured.Time.Equal(stamp) {
		t.Fatal("date added replaced the capture timestamp")
	}
	// A provider that supplies a capture timestamp must retain the shared field.
	chain := Chain{Providers: []Provider{testProvider(func(_ context.Context, base *Record) (*Record, error) {
		r := base.clone()
		r.Captured = &Timestamp{Time: stamp.Add(time.Hour), Source: "synthetic"}
		return r, nil
	})}}
	r, err = chain.Extract(context.Background(), path)
	if err != nil || r.DateAdded == nil || !r.DateAdded.Equal(stamp) {
		t.Fatalf("provider lost date added: %v, %v", r, err)
	}
}

func TestDateAddedDoesNotFollowReplacedPath(t *testing.T) {
	path := chainFixture(t)
	stamp := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	setSyntheticDateAdded(t, path, stamp)
	file, err := openFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	setSyntheticDateAdded(t, path, stamp.Add(time.Hour))
	oldInfo, oldErr := file.Stat()
	newInfo, newErr := os.Stat(path)
	if oldErr != nil || newErr != nil || os.SameFile(oldInfo, newInfo) {
		t.Fatalf("replacement fixture did not change identity: %v, %v", oldErr, newErr)
	}
	got, err := readDateAdded(file)
	if err != nil || got != nil {
		t.Fatalf("read replacement metadata instead of open file: %v, %v", got, err)
	}
}

func TestDateAddedUnavailableForHardLinks(t *testing.T) {
	path := chainFixture(t)
	if err := os.Link(path, filepath.Join(filepath.Dir(path), "linked")); err != nil {
		t.Fatal(err)
	}
	r, err := NewRecord(path)
	if err != nil {
		t.Fatal(err)
	}
	if r.DateAdded != nil {
		t.Fatal("ambiguous hard link date added exposed")
	}
}

func TestCopyUsesDestinationDateAdded(t *testing.T) {
	path := chainFixture(t)
	old := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	setSyntheticDateAdded(t, path, old)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(filepath.Dir(path), "uploads", "synthetic")
	before := time.Now().Truncate(time.Second)
	p := plan.Plan{Items: []plan.Item{{Source: path, Ops: []plan.Op{{Kind: plan.Copy, Dest: dest, OnConflict: "fail"}}}}}
	if _, err := apply.Run(context.Background(), p, apply.Options{}); err != nil {
		t.Fatal(err)
	}
	r, err := NewRecord(dest)
	if err != nil {
		t.Fatal(err)
	}
	if r.DateAdded == nil || r.DateAdded.Before(before) || r.DateAdded.After(time.Now()) {
		t.Fatalf("copy inherited the source date added: %v; warnings: %v", r.DateAdded, r.Warnings)
	}
	if !r.FileMTime.Equal(old) {
		t.Fatal("copy changed mtime instead of using date added")
	}
}

func setSyntheticDateAdded(t *testing.T, path string, stamp time.Time) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	attrs := unix.Attrlist{Bitmapcount: unix.ATTR_BIT_MAP_COUNT, Commonattr: unix.ATTR_CMN_ADDEDTIME}
	value := unix.Timespec{Sec: stamp.Unix(), Nsec: int64(stamp.Nanosecond())}
	//lint:ignore SA1019 x/sys has no fsetattrlist wrapper; only set attributes on synthetic fixtures.
	_, _, errno := syscall.Syscall6(unix.SYS_FSETATTRLIST, file.Fd(), uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value), 0, 0)
	if errno == unix.ENOTSUP {
		t.Skip("test filesystem does not support date added")
	}
	if errno != 0 {
		t.Fatal(errno)
	}
}
