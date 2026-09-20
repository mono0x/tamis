//go:build !darwin

package meta

import "testing"

func TestDateAddedUnavailable(t *testing.T) {
	r, err := NewRecord(chainFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if r.DateAdded != nil {
		t.Fatal("date added must not fall back to birthtime or mtime")
	}
}
