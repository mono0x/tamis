package apply

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/mono0x/tamis/internal/plan"
	"golang.org/x/sys/unix"
)

type Validation struct {
	Plan     plan.Plan
	Warnings []string
}

// volume is the bytes written to one device, along with an existing directory
// on it to run statfs against. Validate accumulates one per device.
type volume struct {
	parent string
	bytes  uint64
}

// pass says which of the two runs over a plan the validator is performing. The
// checks are identical; only the authority over destinations differs.
type pass bool

const (
	// assign fills in trash paths and resolves rename conflicts, producing the
	// destinations displayed to the user.
	assign pass = true
	// verify rechecks those destinations without changing them, so a conflict
	// that appeared since validation stops the run instead of silently
	// retargeting a file.
	verify pass = false
)

func Validate(input plan.Plan, opts Options) (Validation, error) {
	return validate(input, opts, assign)
}

func validate(input plan.Plan, opts Options, p pass) (Validation, error) {
	result := Validation{Plan: input}
	result.Plan.Items = slices.Clone(input.Items)
	destinations := map[string]bool{}
	sources := map[string]bool{}
	moved := map[string]bool{}
	copied := map[string]bool{}
	required := map[uint64]volume{}
	dirs := destDirs{}
	for _, item := range input.Items {
		sources[item.Source] = true
	}
	for itemIndex := range result.Plan.Items {
		item := &result.Plan.Items[itemIndex]
		item.Ops = slices.Clone(item.Ops)
		sourceInfo, err := checkSource(item.Source, item.SourceInfo)
		if err != nil {
			return result, fmt.Errorf("source %s: %w", item.Source, err)
		}
		item.SourceInfo = sourceInfo
		for opIndex := range item.Ops {
			op := &item.Ops[opIndex]
			switch op.Kind {
			case plan.Copy, plan.Move, plan.Trash, plan.Shell:
			default:
				return result, fmt.Errorf("unsupported operation %q", op.Kind)
			}
			if moved[item.Source] {
				return result, fmt.Errorf("source %s has an operation after move or trash", item.Source)
			}
			if op.Kind == plan.Shell {
				if copied[item.Source] || op.Command == nil || op.Dest != "" || op.OnConflict != "" {
					return result, fmt.Errorf("invalid shell operation for %s: shell must precede copies and have no destination or conflict policy", item.Source)
				}
				command := *op.Command
				command.Env = maps.Clone(command.Env)
				op.Command = &command
				if err := command.Validate(); err != nil {
					return result, err
				}
				continue
			}
			if op.Command != nil {
				return result, fmt.Errorf("shell command on non-shell operation")
			}
			copied[item.Source] = copied[item.Source] || op.Kind == plan.Copy
			if op.Kind == plan.Move || op.Kind == plan.Trash {
				moved[item.Source] = true
			}
			if op.Kind == plan.Trash && p == assign {
				op.Dest, err = trashPath(opts.TrashDir, item.Source)
				if err != nil {
					return result, err
				}
			}
			if op.Kind == plan.Trash {
				if err := validateTrash(item.Source, op.Dest); err != nil {
					return result, err
				}
				if op.Dest == "" {
					continue
				}
			}
			if op.Dest == "" || !filepath.IsAbs(op.Dest) {
				return result, fmt.Errorf("invalid destination %q", op.Dest)
			}
			if sources[op.Dest] {
				return result, fmt.Errorf("cycle: destination is another source: %s", op.Dest)
			}
			policy := op.OnConflict
			if p == verify && policy == "rename" {
				policy = "fail"
			}
			resolved, warnings, err := resolveConflict(filepath.Clean(op.Dest), policy, destinations)
			if err != nil {
				return result, err
			}
			if op.Kind == plan.Copy && policy == "skip" && len(warnings) > 0 {
				warnings[0] += "; any later move/trash for " + item.Source + " will also be skipped"
			}
			result.Warnings = append(result.Warnings, warnings...)
			op.Dest = resolved
			destinations[resolved] = true
			device, written, err := validateDestination(dirs, sourceInfo, op)
			if err != nil {
				if opts.Force {
					result.Warnings = append(result.Warnings, err.Error())
				} else {
					return result, err
				}
			}
			required[device] = volume{parent: written.parent, bytes: required[device].bytes + written.bytes}
		}
	}
	for dest := range destinations {
		for parent := filepath.Dir(dest); ; parent = filepath.Dir(parent) {
			if destinations[parent] {
				return result, fmt.Errorf("destination is also used as a directory: %s", parent)
			}
			if parent == filepath.Dir(parent) {
				break
			}
		}
	}
	for _, v := range required {
		if v.bytes == 0 {
			continue
		}
		var stat unix.Statfs_t
		if err := unix.Statfs(v.parent, &stat); err != nil {
			return result, err
		}
		available := uint64(stat.Bavail) * uint64(stat.Bsize)
		if v.bytes > available {
			err := fmt.Errorf("insufficient space on volume for %s: need %d, available %d", v.parent, v.bytes, available)
			if opts.Force {
				result.Warnings = append(result.Warnings, err.Error())
			} else {
				return result, err
			}
		}
	}
	return result, nil
}

func checkSource(path string, expected os.FileInfo) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("source is not a regular file: %s", path)
	}
	if expected != nil && (!os.SameFile(expected, info) || expected.Size() != info.Size() || !expected.ModTime().Equal(info.ModTime())) {
		return nil, fmt.Errorf("source changed since scan or validation: %s", path)
	}
	return info, nil
}
func resolveConflict(dest, policy string, occupied map[string]bool) (string, []string, error) {
	exists := occupied[dest]
	if !exists {
		_, err := os.Lstat(dest)
		if err != nil && !os.IsNotExist(err) {
			return dest, nil, err
		}
		exists = err == nil
	}
	if !exists {
		return dest, nil, nil
	}
	switch policy {
	case "skip":
		return dest, []string{"skip existing destination: " + dest}, nil
	case "rename":
		ext := filepath.Ext(dest)
		base := strings.TrimSuffix(filepath.Base(dest), ext)
		for n := 2; ; n++ {
			suffix := fmt.Sprintf("-%d%s", n, ext)
			if len(suffix) >= plan.MaxNameBytes {
				return dest, nil, fmt.Errorf("filename extension too long: %s", dest)
			}
			candidate := filepath.Join(filepath.Dir(dest), plan.FitName(base, suffix)+suffix)
			if !occupied[candidate] {
				if _, err := os.Lstat(candidate); os.IsNotExist(err) {
					return candidate, nil, nil
				} else if err != nil {
					return dest, nil, err
				}
			}
		}
	default:
		return dest, nil, fmt.Errorf("destination exists: %s", dest)
	}
}

// destDir is what validate needs to know about a destination's parent: the
// nearest existing ancestor, its stat for the device id, and whether it is
// writable.
type destDir struct {
	path     string
	info     os.FileInfo
	writable error
}

// destDirs answers those questions once per directory. A library organized into
// a few hundred folders otherwise repeats the same stats once per file, and
// nothing is created while a pass is running, so the answers cannot go stale
// within one.
type destDirs map[string]destDir

func (d destDirs) lookup(dir string) (destDir, error) {
	if entry, ok := d[dir]; ok {
		return entry, nil
	}
	path, info, err := existingParent(dir)
	if err != nil {
		return destDir{}, err
	}
	entry := destDir{path: path, info: info, writable: unix.Access(path, unix.W_OK)}
	d[dir] = entry
	return entry, nil
}

// validateDestination reports the device the op writes to and how much it
// writes there, so the caller can accumulate free-space requirements per volume.
func validateDestination(dirs destDirs, sourceInfo os.FileInfo, op *plan.Op) (device uint64, written volume, err error) {
	dir, err := dirs.lookup(filepath.Dir(op.Dest))
	if err != nil {
		return 0, volume{}, err
	}
	written.parent = dir.path
	if dir.writable != nil {
		return 0, written, fmt.Errorf("destination is not writable: %s", dir.path)
	}
	srcStat, srcOK := sourceInfo.Sys().(*syscall.Stat_t)
	dstStat, dstOK := dir.info.Sys().(*syscall.Stat_t)
	same := false
	if dstOK {
		device = uint64(dstStat.Dev)
	}
	if srcOK && dstOK {
		same = srcStat.Dev == dstStat.Dev
	}
	switch op.Kind {
	case plan.Copy:
		// Cloning is optional; reserve enough space for a full copy.
		written.bytes = uint64(sourceInfo.Size())
	case plan.Move, plan.Trash:
		if !same {
			written.bytes = uint64(sourceInfo.Size())
		}
	}
	return device, written, nil
}

// existingParent walks up from path to the nearest directory that exists and
// returns it along with its stat, which the caller needs for the device id.
func existingParent(path string) (string, os.FileInfo, error) {
	for {
		info, err := os.Stat(path)
		if err == nil {
			if !info.IsDir() {
				return "", nil, fmt.Errorf("destination parent is not a directory: %s", path)
			}
			return path, info, nil
		}
		if !os.IsNotExist(err) {
			return "", nil, err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", nil, err
		}
		path = parent
	}
}
