// Package rule loads and compiles YAML rules and evaluates metadata into plans.
package rule

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/mono0x/tamis/internal/meta"
	planpkg "github.com/mono0x/tamis/internal/plan"
	"github.com/mono0x/tamis/internal/scan"
)

type Env struct {
	Path           string            `expr:"path"`
	RelPath        string            `expr:"relpath"`
	Basename       string            `expr:"basename"`
	Ext            string            `expr:"ext"`
	Dir            string            `expr:"dir"`
	Size           int64             `expr:"size"`
	Kind           string            `expr:"kind"`
	Format         string            `expr:"format"`
	MIME           string            `expr:"mime"`
	Captured       *time.Time        `expr:"captured"`
	CapturedSource string            `expr:"captured_source"`
	MTime          time.Time         `expr:"mtime"`
	CTime          time.Time         `expr:"ctime"`
	DateAdded      *time.Time        `expr:"date_added"`
	Age            time.Duration     `expr:"age"`
	AgeHours       float64           `expr:"age_hours"`
	Now            time.Time         `expr:"now"`
	Make           string            `expr:"make"`
	Model          string            `expr:"model"`
	Lens           string            `expr:"lens"`
	ISO            *int              `expr:"iso"`
	FNumber        *float64          `expr:"fnumber"`
	Focal          *float64          `expr:"focal"`
	Width          int               `expr:"width"`
	Height         int               `expr:"height"`
	GPS            *meta.GPS         `expr:"gps"`
	Duration       *time.Duration    `expr:"duration"`
	Raw            map[string]any    `expr:"raw"`
	Capture        map[string]string `expr:"capture"`
	Seq            int               `expr:"seq"`
}
type Compiled struct {
	Config       *Config
	rules        []compiledRule
	Location     *time.Location
	MVHDLocation *time.Location
}
type compiledRule struct {
	rule    Rule
	matcher scan.Matcher
	fields  meta.Fields
	when    *vm.Program
	capture *regexp.Regexp
	actions []compiledAction
}
type compiledAction struct {
	command          *compiledCommand
	kind, onConflict string
	dest             Template
	rename           *Template
}

type compiledCommand struct {
	run string
	env map[string]Template
}

func compile(cfg *Config) (*Compiled, error) {
	location, err := parseLocation(cfg.Settings.AssumeTimezone)
	if err != nil {
		return nil, err
	}
	mvhd := time.Local
	if cfg.Settings.Video.MVHDTimezone == "utc" {
		mvhd = time.UTC
	}
	out := &Compiled{Config: cfg, Location: location, MVHDLocation: mvhd}
	for _, r := range cfg.Rules {
		cr := compiledRule{rule: r, matcher: scan.NewMatcher(r.ScanSource())}
		condition := r.When
		if condition == "" {
			condition = "true"
		}
		cr.when, err = expr.Compile(condition, expr.Env(Env{}), expr.AsBool(), expr.MaxNodes(200))
		if err != nil {
			return nil, fmt.Errorf("rule %q when: %w", r.Name, err)
		}
		cr.fields, err = filterFields(condition)
		if err != nil {
			return nil, fmt.Errorf("rule %q when: %w", r.Name, err)
		}
		if r.Capture != "" {
			cr.capture = regexp.MustCompile(r.Capture)
		}
		for _, a := range r.Actions {
			kind, dest, _ := a.kindAndDest()
			if kind == "move" || kind == "trash" {
				cr.rule.Stop = true
			}
			dest, err := expandHome(dest)
			if err != nil {
				return nil, err
			}
			conflict := a.OnConflict
			if conflict == "" {
				conflict = cfg.Settings.OnConflict
			}
			action := compiledAction{kind: kind, onConflict: conflict, dest: compileTemplate(dest, cfg.Vars)}
			templates := []Template{action.dest}
			if a.Rename != "" {
				rename := compileTemplate(a.Rename, cfg.Vars)
				action.rename = &rename
				templates = append(templates, rename)
			}
			if a.Shell != nil {
				action.command = &compiledCommand{run: a.Shell.Run, env: map[string]Template{}}
				for name, value := range a.Shell.Env {
					t := compileTemplate(value, cfg.Vars)
					action.command.env[name] = t
					templates = append(templates, t)
				}
			}
			for _, t := range templates {
				if err := t.validateFields(cr.fields); err != nil {
					return nil, fmt.Errorf("rule %q: %w", r.Name, err)
				}
			}
			cr.actions = append(cr.actions, action)
		}
		out.rules = append(out.rules, cr)
	}
	return out, nil
}
func (c *Compiled) Evaluate(records []meta.Record, now time.Time) (planpkg.Plan, error) {
	result := planpkg.Plan{CreatedAt: now, ConfigHash: c.Config.Hash}
	for index := range records {
		record := &records[index]
		matched := false
		if record.ExtractionFailed {
			result.Skipped = append(result.Skipped, planpkg.SkipRecord{Source: record.Path, Reason: "metadata extraction failed; file left untouched"})
			continue
		}
		name := filepath.Base(record.Path)
		base := newEnv(record, name, now, index+1)
		for _, r := range c.rules {
			rel, ok := r.matcher.Match(record.Path)
			if !ok {
				continue
			}
			env := base
			env.RelPath = rel
			if r.capture != nil {
				match := r.capture.FindStringSubmatch(name)
				if match == nil {
					continue
				}
				env.Capture = map[string]string{}
				for i, name := range r.capture.SubexpNames() {
					if i > 0 && name != "" {
						env.Capture[name] = match[i]
					}
				}
			}
			value, err := expr.Run(r.when, env)
			if err != nil {
				return result, fmt.Errorf("rule %q: %w", r.rule.Name, err)
			}
			if !value.(bool) {
				continue
			}
			matched = true
			item := planpkg.Item{Source: record.Path, SourceInfo: record.SourceInfo, Rule: r.rule.Name}
			for _, action := range r.actions {
				op, err := renderAction(action, env, record, c.Config.Settings.WindowsNames)
				if err != nil {
					return result, fmt.Errorf("rule %q: %w", r.rule.Name, err)
				}
				item.Ops = append(item.Ops, op)
			}
			result.Items = append(result.Items, item)
			if r.rule.Stop {
				break
			}
		}
		if !matched {
			result.Skipped = append(result.Skipped, planpkg.SkipRecord{Source: record.Path, Reason: "no matching rule"})
		}
	}
	return result, nil
}
func newEnv(r *meta.Record, name string, now time.Time, seq int) Env {
	ext := filepath.Ext(name)
	var captured *time.Time
	source := ""
	if r.Captured != nil {
		tm := r.Captured.Time
		captured = &tm
		source = r.Captured.Source
	}
	return Env{Path: r.Path, RelPath: r.RelPath, Basename: strings.TrimSuffix(name, ext), Ext: strings.ToLower(ext), Dir: filepath.Base(filepath.Dir(r.Path)), Size: r.Size, Kind: string(r.Kind), Format: string(r.Format), MIME: r.MIMEType, Captured: captured, CapturedSource: source, MTime: r.FileMTime, CTime: r.FileCTime, DateAdded: r.DateAdded, Age: now.Sub(r.FileCTime), AgeHours: now.Sub(r.FileCTime).Hours(), Now: now, Make: r.Make, Model: r.Model, Lens: r.LensModel, ISO: r.ISO, FNumber: r.FNumber, Focal: r.FocalLength, Width: r.Width, Height: r.Height, GPS: r.GPS, Duration: r.Duration, Raw: r.Raw, Capture: map[string]string{}, Seq: seq}
}
func renderAction(a compiledAction, env Env, record *meta.Record, windows bool) (planpkg.Op, error) {
	kind := planpkg.OpKind(a.kind)
	if kind == planpkg.Shell {
		command := &planpkg.Command{Run: a.command.run, Env: map[string]string{}}
		for name, template := range a.command.env {
			value, err := template.Render(env)
			if err != nil {
				return planpkg.Op{}, err
			}
			command.Env[name] = value
		}
		if err := command.Validate(); err != nil {
			return planpkg.Op{}, err
		}
		return planpkg.Op{Kind: kind, Command: command, Reason: "matched rule"}, nil
	}
	if kind == planpkg.Trash {
		return planpkg.Op{Kind: kind, Reason: "matched rule", OnConflict: a.onConflict}, nil
	}
	dest, err := a.dest.RenderPath(env)
	if err != nil {
		return planpkg.Op{}, err
	}
	if strings.HasSuffix(dest, "/") || strings.HasSuffix(dest, string(filepath.Separator)) {
		name := filepath.Base(record.Path)
		if a.rename != nil {
			name, err = a.rename.Render(env)
			if err != nil {
				return planpkg.Op{}, err
			}
			if name == "" || name == "." || name == ".." {
				return planpkg.Op{}, fmt.Errorf("rename must produce a filename")
			}
		}
		dest = filepath.Join(dest, normalizeFilename(name, windows))
	}
	if !filepath.IsAbs(dest) {
		return planpkg.Op{}, fmt.Errorf("destination must be absolute: %s", dest)
	}
	return planpkg.Op{Kind: kind, Dest: filepath.Clean(dest), Reason: "matched rule", OnConflict: a.onConflict}, nil
}

func (r Rule) ScanSource() scan.Source {
	return scan.Source{Path: r.Location, Recursive: r.Recursive, Include: r.Include, Exclude: r.Exclude}
}

func (c *Compiled) RelativePath(path string) (string, bool) {
	for _, r := range c.rules {
		if rel, ok := r.matcher.Match(path); ok {
			return rel, true
		}
	}
	return "", false
}
