package meta

import (
	"context"
	"io"
)

// contextReader keeps decoder reads and seeks within the file's deadline,
// without starting a decoder goroutine that could outlive the worker.
type contextReader struct {
	ctx    context.Context
	reader io.ReadSeeker
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if err == nil {
		err = r.ctx.Err()
	}
	return n, err
}

func (r contextReader) Seek(offset int64, whence int) (int64, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	position, err := r.reader.Seek(offset, whence)
	if err == nil {
		err = r.ctx.Err()
	}
	return position, err
}
