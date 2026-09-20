package rule

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/mono0x/tamis/internal/meta"
	"github.com/mono0x/tamis/internal/plan"
	"golang.org/x/text/unicode/norm"
)

type Template struct{ parts []templatePart }

// A part is either a literal or an expression. For an expression, name is the
// head of the filter pipeline and filters are the segments after it, both split
// once at load time.
type templatePart struct {
	literal    string
	expression bool
	name       string
	filters    []string
}

var interpolation = regexp.MustCompile(`\{\{\s*(.*?)\s*\}\}`)

func compileTemplate(value string, vars map[string]string) Template {
	for key, replacement := range vars {
		value = strings.ReplaceAll(value, "{{"+key+"}}", replacement)
		value = strings.ReplaceAll(value, "{{vars."+key+"}}", replacement)
	}
	var parts []templatePart
	last := 0
	for _, loc := range interpolation.FindAllStringSubmatchIndex(value, -1) {
		segments := strings.Split(value[loc[2]:loc[3]], "|")
		parts = append(parts, templatePart{literal: value[last:loc[0]]}, templatePart{expression: true, name: strings.TrimSpace(segments[0]), filters: segments[1:]})
		last = loc[1]
	}
	parts = append(parts, templatePart{literal: value[last:]})
	return Template{parts: parts}
}

func (t Template) Render(env Env) (string, error) {
	return t.render(env, false)
}

func (t Template) RenderPath(env Env) (string, error) {
	return t.render(env, true)
}

func (t Template) render(env Env, path bool) (string, error) {
	var out strings.Builder
	for _, part := range t.parts {
		if !part.expression {
			out.WriteString(part.literal)
			continue
		}
		value, err := evalPipeline(part, env)
		if err != nil {
			return "", err
		}
		if path {
			if strings.ContainsAny(value, "\\\x00") || (!multiComponent[part.name] && strings.Contains(value, "/")) {
				return "", fmt.Errorf("template variable %q contains a path separator or NUL; use slug for a path component", part.name)
			}
			for _, component := range strings.Split(value, "/") {
				if component == "." || component == ".." {
					return "", fmt.Errorf("template variable %q contains an unsafe path component", part.name)
				}
			}
		}
		out.WriteString(value)
	}
	return out.String(), nil
}

// Paths and date layouts may span directories; metadata values may not.
var multiComponent = map[string]bool{meta.FieldPath: true, meta.FieldRelPath: true, meta.FieldCaptured: true, meta.FieldMTime: true, meta.FieldCTime: true, meta.FieldDateAdded: true}

func evalPipeline(part templatePart, env Env) (string, error) {
	value, err := envValue(part.name, env)
	if err != nil {
		return "", err
	}
	for _, segment := range part.filters {
		fields, err := splitFields(strings.TrimSpace(segment))
		if err != nil {
			return "", err
		}
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "date":
			if len(fields) != 2 {
				return "", fmt.Errorf("date requires layout")
			}
			tm, ok := value.(interface{ Format(string) string })
			if !ok {
				return "", fmt.Errorf("date input is nil or not time")
			}
			value = tm.Format(fields[1])
		case "slug":
			value = slug(fmt.Sprint(value))
		case "lower":
			value = strings.ToLower(fmt.Sprint(value))
		case "upper":
			value = strings.ToUpper(fmt.Sprint(value))
		case "pad":
			if len(fields) != 2 {
				return "", fmt.Errorf("pad requires width")
			}
			width, e := strconv.Atoi(fields[1])
			if e != nil {
				return "", e
			}
			value = fmt.Sprintf("%0*s", width, fmt.Sprint(value))
		case "default":
			if len(fields) != 2 {
				return "", fmt.Errorf("default requires value")
			}
			if value == nil || fmt.Sprint(value) == "" {
				value = fields[1]
			}
		case "trunc":
			if len(fields) != 2 {
				return "", fmt.Errorf("trunc requires length")
			}
			n, e := strconv.Atoi(fields[1])
			if e != nil {
				return "", e
			}
			if n < 0 {
				return "", fmt.Errorf("trunc length must be non-negative")
			}
			r := []rune(fmt.Sprint(value))
			if len(r) > n {
				r = r[:n]
			}
			value = string(r)
		case "slice":
			if len(fields) != 3 {
				return "", fmt.Errorf("slice requires start and end")
			}
			start, e := strconv.Atoi(fields[1])
			if e != nil {
				return "", e
			}
			end, e := strconv.Atoi(fields[2])
			if e != nil {
				return "", e
			}
			runes := []rune(fmt.Sprint(value))
			if start < 0 || end < start || end > len(runes) {
				return "", fmt.Errorf("slice %d:%d out of range", start, end)
			}
			value = string(runes[start:end])
		default:
			return "", fmt.Errorf("unknown filter %q", fields[0])
		}
	}
	if value == nil {
		return "", nil
	}
	return fmt.Sprint(value), nil
}

func splitFields(value string) ([]string, error) {
	var fields []string
	for len(strings.TrimSpace(value)) > 0 {
		value = strings.TrimSpace(value)
		if value[0] == '"' {
			end := 1
			for end < len(value) && value[end] != '"' {
				end++
			}
			if end == len(value) {
				return nil, fmt.Errorf("unterminated quote")
			}
			fields = append(fields, value[1:end])
			value = value[end+1:]
		} else {
			end := strings.IndexByte(value, ' ')
			if end < 0 {
				fields = append(fields, value)
				break
			}
			fields = append(fields, value[:end])
			value = value[end+1:]
		}
	}
	return fields, nil
}
func envValue(name string, env Env) (any, error) {
	switch name {
	case meta.FieldPath:
		return env.Path, nil
	case meta.FieldRelPath:
		return env.RelPath, nil
	case meta.FieldBasename:
		return env.Basename, nil
	case meta.FieldExt:
		return env.Ext, nil
	case meta.FieldDir:
		return env.Dir, nil
	case meta.FieldCaptured:
		if env.Captured == nil {
			return nil, nil
		}
		return *env.Captured, nil
	case meta.FieldMTime:
		return env.MTime, nil
	case meta.FieldCTime:
		return env.CTime, nil
	case meta.FieldDateAdded:
		if env.DateAdded == nil {
			return nil, nil
		}
		return *env.DateAdded, nil
	case meta.FieldMake:
		return env.Make, nil
	case meta.FieldModel:
		return env.Model, nil
	case meta.FieldLens:
		return env.Lens, nil
	case meta.FieldSeq:
		return env.Seq, nil
	}
	if capture, ok := strings.CutPrefix(name, meta.FieldCapture+"."); ok {
		return env.Capture[capture], nil
	}
	return nil, fmt.Errorf("unknown template variable %q", name)
}

func slug(value string) string {
	value = strings.ToLower(norm.NFKD.String(value))
	var out strings.Builder
	dash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
			dash = false
		} else if !dash && out.Len() > 0 {
			out.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

var reservedWindowsNames = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[1-9]|lpt[1-9])$`)

func normalizeFilename(name string, windows bool) string {
	name = norm.NFC.String(strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 {
			return '-'
		}
		return r
	}, name))
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if len(ext) > plan.MaxNameBytes {
		base, ext = name, ""
	}
	if windows && reservedWindowsNames.MatchString(base) {
		base = "_" + base
	}
	return plan.FitName(base, ext) + ext
}
