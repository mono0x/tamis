// Package scan walks source directories and extracts metadata with bounded concurrency.
package scan

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mono0x/tamis/internal/meta"
)

type Source struct {
	Path             string
	Recursive        bool
	Exclude, Include []string
}
type Candidate struct{ Path, RelPath string }

func Walk(source Source) ([]Candidate, error) {
	root, err := filepath.Abs(source.Path)
	if err != nil {
		return nil, err
	}
	if isHidden(root) {
		return nil, nil
	}
	source.Path = root
	matcher := NewMatcher(source)
	includeDirs := compileIncludeDirs(source.Include)
	var out []Candidate
	var walkErrors []error
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == root && entry == nil && errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			walkErrors = append(walkErrors, err)
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if rel == "." && entry.IsDir() {
			return nil
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") || matchesAny(matcher.exclude, rel) || !source.Recursive || (len(matcher.include) > 0 && !matchesAny(includeDirs, rel)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, ok := matcher.Match(path)
		if !ok {
			return nil
		}
		out = append(out, Candidate{Path: path, RelPath: rel})
		return nil
	})
	return out, errors.Join(append(walkErrors, err)...)
}

// Directory prefixes let the walk skip trees that cannot contain a match.
// After **, keep the whole subtree: ** can span any number of directories.
func compileIncludeDirs(patterns []string) []*regexp.Regexp {
	var prefixes []string
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)
		for i := 0; i < len(pattern); i++ {
			if strings.HasPrefix(pattern[i:], "**") {
				prefixes = append(prefixes, pattern[:i]+"**")
				break
			}
			if pattern[i] == '/' {
				prefixes = append(prefixes, pattern[:i])
			}
		}
	}
	return compileGlobs(prefixes)
}

func compileGlobs(patterns []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		out = append(out, compileGlob(pattern))
	}
	return out
}

func matchesAny(matchers []*regexp.Regexp, path string) bool {
	for _, matcher := range matchers {
		if matcher.MatchString(path) {
			return true
		}
	}
	return false
}

// compileGlob translates a glob into an anchored regexp once per walk; matching
// runs for every entry, so the pattern must not be rebuilt per path.
func compileGlob(pattern string) *regexp.Regexp {
	pattern = filepath.ToSlash(pattern)
	var expression strings.Builder
	expression.WriteByte('^')
	for i := 0; i < len(pattern); {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i += 2
				if i < len(pattern) && pattern[i] == '/' {
					expression.WriteString("(?:.*/)?")
					i++
				} else {
					expression.WriteString(".*")
				}
			} else {
				expression.WriteString("[^/]*")
				i++
			}
		case '?':
			expression.WriteString("[^/]")
			i++
		default:
			expression.WriteString(regexp.QuoteMeta(string(pattern[i])))
			i++
		}
	}
	expression.WriteByte('$')
	return regexp.MustCompile(expression.String())
}

// Extract fans candidates out over jobs workers. Both jobs and timeout must be
// positive; the caller owns the defaults.
func Extract(ctx context.Context, candidates []Candidate, chain meta.Chain, jobs int, timeout time.Duration) ([]meta.Record, error) {
	type result struct {
		index  int
		record *meta.Record
		err    error
	}
	in := make(chan int)
	out := make(chan result)
	var workers sync.WaitGroup
	for range jobs {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range in {
				if ctx.Err() != nil {
					return
				}
				child, cancel := context.WithTimeout(ctx, timeout)
				r, e := chain.Extract(child, candidates[i].Path)
				cancel()
				if r != nil {
					r.RelPath = candidates[i].RelPath
				}
				out <- result{i, r, e}
			}
		}()
	}
	go func() {
	schedule:
		for i := range candidates {
			select {
			case in <- i:
			case <-ctx.Done():
				break schedule
			}
		}
		close(in)
		workers.Wait()
		close(out)
	}()
	records := make([]meta.Record, 0, len(candidates))
	var errs []string
	for result := range out {
		if result.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", candidates[result.index].Path, result.err))
			continue
		}
		records = append(records, *result.record)
	}
	slices.SortFunc(records, func(a, b meta.Record) int { return strings.Compare(a.Path, b.Path) })
	if err := ctx.Err(); err != nil {
		return records, err
	}
	if len(errs) > 0 {
		return records, fmt.Errorf("%d extraction errors: %s", len(errs), strings.Join(errs, "; "))
	}
	return records, nil
}
