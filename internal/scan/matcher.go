package scan

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Matcher checks a file against a scan scope without filesystem I/O.
type Matcher struct {
	source           Source
	include, exclude []*regexp.Regexp
}

func NewMatcher(source Source) Matcher {
	return Matcher{source: source, include: compileGlobs(source.Include), exclude: compileGlobs(source.Exclude)}
}

func (m Matcher) Match(path string) (string, bool) {
	if isHidden(path) {
		return "", false
	}
	rel, err := filepath.Rel(m.source.Path, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		rel = filepath.Base(path)
	}
	if !m.source.Recursive && strings.Contains(rel, "/") {
		return "", false
	}
	for ancestor := rel; len(m.exclude) > 0 && ancestor != "."; ancestor = filepath.ToSlash(filepath.Dir(ancestor)) {
		if matchesAny(m.exclude, ancestor) {
			return "", false
		}
	}
	return rel, len(m.include) == 0 || matchesAny(m.include, rel)
}

func isHidden(path string) bool {
	for rest := filepath.ToSlash(filepath.Clean(path)); rest != ""; {
		var name string
		name, rest, _ = strings.Cut(rest, "/")
		if strings.HasPrefix(name, ".") && name != "." && name != ".." {
			return true
		}
	}
	return false
}
