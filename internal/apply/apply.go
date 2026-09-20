// Package apply validates and executes planned filesystem operations.
package apply

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mono0x/tamis/internal/plan"
	"github.com/mono0x/trash"
)

type Options struct {
	Force    bool
	TrashDir string
}
type Result struct {
	Completed int         `json:"completed"`
	Skipped   []SkippedOp `json:"skipped,omitempty"`
}
type SkippedOp struct {
	Source string      `json:"source"`
	Kind   plan.OpKind `json:"kind"`
	Dest   string      `json:"dest"`
	Reason string      `json:"reason"`
}

func Run(ctx context.Context, input plan.Plan, opts Options) (Result, error) {
	validated, err := Validate(input, opts)
	if err != nil {
		return Result{}, err
	}
	return validated.Run(ctx, opts)
}

// Run rechecks the complete plan without changing the displayed destinations. New rename conflicts must stop execution.
func (v Validation) Run(ctx context.Context, opts Options) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return v.run(ctx, opts, trash.Move)
}

func (v Validation) run(ctx context.Context, opts Options, trash func(string) error) (Result, error) {
	validated, err := validate(v.Plan, opts, verify)
	if err != nil {
		return Result{}, err
	}
	result := Result{}
	copySkipped := map[string]bool{}
	updated := map[string]os.FileInfo{}
	for _, item := range validated.Plan.Items {
		for _, op := range item.Ops {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			expected := item.SourceInfo
			if info := updated[item.Source]; info != nil {
				expected = info
			}
			if _, err := checkSource(item.Source, expected); err != nil {
				return result, err
			}
			if op.Kind == plan.Shell {
				if err := runShell(ctx, item.Source, *op.Command); err != nil {
					return result, fmt.Errorf("apply stopped after %d operations: %w", result.Completed, err)
				}
				info, err := checkSource(item.Source, nil)
				if err != nil {
					return result, err
				}
				updated[item.Source] = info
				result.Completed++
				continue
			}
			if op.Kind != plan.Copy && copySkipped[item.Source] {
				result.Skipped = append(result.Skipped, SkippedOp{item.Source, op.Kind, op.Dest, "earlier copy was skipped; source retained"})
				continue
			}
			if op.OnConflict == "skip" && op.Dest != "" {
				if _, err := os.Lstat(op.Dest); err == nil {
					if op.Kind == plan.Copy {
						copySkipped[item.Source] = true
					}
					result.Skipped = append(result.Skipped, SkippedOp{item.Source, op.Kind, op.Dest, "destination exists"})
					continue
				} else if !os.IsNotExist(err) {
					return result, err
				}
			}
			if op.Kind == plan.Trash && op.Dest == "" {
				err = trash(item.Source)
			} else {
				err = execute(item.Source, op)
			}
			if err != nil {
				return result, fmt.Errorf("apply stopped after %d operations: %w", result.Completed, err)
			}
			result.Completed++
		}
	}
	return result, nil
}
func execute(source string, op plan.Op) error {
	// The trash directory is private to the user; ordinary destinations are not.
	mode := os.FileMode(0o755)
	if op.Kind == plan.Trash {
		mode = 0o700
	}
	if err := makeDirs(filepath.Dir(op.Dest), mode); err != nil {
		return err
	}
	switch op.Kind {
	case plan.Copy:
		return copyFile(source, op.Dest)
	case plan.Move, plan.Trash:
		return moveFile(source, op.Dest)
	default:
		return fmt.Errorf("unsupported operation %q", op.Kind)
	}
}
func moveFile(source, dest string) error {
	if err := renameExclusive(source, dest); err == nil {
		if err := syncDir(filepath.Dir(dest)); err != nil {
			return err
		}
		if filepath.Dir(source) != filepath.Dir(dest) {
			return syncDir(filepath.Dir(source))
		}
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	if err := copyFile(source, dest); err != nil {
		return err
	}
	if err := os.Remove(source); err != nil {
		return err
	}
	return syncDir(filepath.Dir(source))
}
func copyFile(source, dest string) error {
	return copyFileWith(source, dest, cloneFile, syncDir)
}

func copyFileWith(source, dest string, clone func(*os.File, string) error, syncParent func(string) error) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	info, err := src.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file: %s", source)
	}
	// clonefile requires a nonexistent destination. Keep it private until synced.
	tmpDir, err := os.MkdirTemp(filepath.Dir(dest), ".tamis-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	tmpName := filepath.Join(tmpDir, "file")
	var tmp *os.File
	cloneErr := clone(src, tmpName)
	if cloneErr == nil {
		tmp, err = os.Open(tmpName)
	} else {
		if !errors.Is(cloneErr, syscall.ENOTSUP) && !errors.Is(cloneErr, syscall.EXDEV) {
			return cloneErr
		}
		tmp, err = os.OpenFile(tmpName, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	}
	if err != nil {
		return err
	}
	defer func() { _ = tmp.Close() }()
	if cloneErr != nil {
		if _, err := io.Copy(tmp, src); err != nil {
			return err
		}
	}
	// A clone may inherit a read-only mode; preserve attributes before restoring it.
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if err := copyXattrs(source, tmpName); err != nil {
		return fmt.Errorf("preserve extended attributes: %w", err)
	}
	if err := os.Chtimes(tmpName, info.ModTime(), info.ModTime()); err != nil {
		return err
	}
	if err := tmp.Chmod(info.Mode()); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := renameExclusive(tmpName, dest); err != nil {
		return err
	}
	return syncParent(filepath.Dir(dest))
}
func trashPath(root, source string) (string, error) {
	if root == "" {
		return "", nil
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, fmt.Sprintf("tamis-%d-%s", time.Now().UnixNano(), filepath.Base(source))), nil
}
