package rule

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/mono0x/tamis/internal/plan"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	Version  int               `yaml:"version"`
	Settings Settings          `yaml:"settings"`
	Vars     map[string]string `yaml:"vars"`
	Rules    []Rule            `yaml:"rules"`
	Hash     string            `yaml:"-"`
}
type Settings struct {
	AssumeTimezone string        `yaml:"assume_timezone"`
	Video          VideoSettings `yaml:"video"`
	OnConflict     string        `yaml:"on_conflict"`
	Jobs           int           `yaml:"jobs"`
	WindowsNames   bool          `yaml:"windows_names"`
}
type VideoSettings struct {
	MVHDTimezone string `yaml:"mvhd_timezone"`
}
type Rule struct {
	Name      string   `yaml:"name"`
	Location  string   `yaml:"location"`
	Recursive bool     `yaml:"recursive"`
	Include   []string `yaml:"include"`
	Exclude   []string `yaml:"exclude"`
	When      string   `yaml:"when"`
	Capture   string   `yaml:"capture"`
	Actions   []Action `yaml:"actions"`
	Stop      bool     `yaml:"stop"`
}
type Action struct {
	Shell      *plan.Command `yaml:"shell"`
	Move       string        `yaml:"move"`
	Copy       string        `yaml:"copy"`
	Trash      bool          `yaml:"trash"`
	Rename     string        `yaml:"rename"`
	OnConflict string        `yaml:"on_conflict"`
}

func Load(path string) (*Compiled, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	cfg.Hash = hex.EncodeToString(sum[:])
	return New(&cfg)
}

// New defaults, validates, and compiles cfg. Callers that synthesize a config
// instead of loading one go through here, so they inherit the same defaults.
func New(cfg *Config) (*Compiled, error) {
	if err := cfg.defaultsAndValidate(); err != nil {
		return nil, err
	}
	return compile(cfg)
}

func (c *Config) defaultsAndValidate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	if c.Settings.AssumeTimezone == "" {
		c.Settings.AssumeTimezone = "local"
	}
	if c.Settings.Video.MVHDTimezone == "" {
		c.Settings.Video.MVHDTimezone = "local"
	}
	if c.Settings.OnConflict == "" {
		c.Settings.OnConflict = "rename"
	}
	if c.Settings.Jobs == 0 {
		c.Settings.Jobs = runtime.NumCPU()
	}
	if _, err := parseLocation(c.Settings.AssumeTimezone); err != nil {
		return fmt.Errorf("settings.assume_timezone: %w", err)
	}
	if c.Settings.Video.MVHDTimezone != "local" && c.Settings.Video.MVHDTimezone != "utc" {
		return fmt.Errorf("settings.video.mvhd_timezone must be local or utc")
	}
	if !validConflict(c.Settings.OnConflict) {
		return fmt.Errorf("invalid on_conflict %q", c.Settings.OnConflict)
	}
	seen := map[string]bool{}
	for i := range c.Rules {
		r := &c.Rules[i]
		if r.Name == "" || seen[r.Name] {
			return fmt.Errorf("rules[%d].name must be non-empty and unique: %q", i, r.Name)
		}
		seen[r.Name] = true
		if strings.TrimSpace(r.Location) == "" {
			return fmt.Errorf("rule %q: location is required", r.Name)
		}
		expanded, err := expandPath(r.Location)
		if err != nil {
			return fmt.Errorf("rule %q location: %w", r.Name, err)
		}
		r.Location = expanded
		if r.Capture != "" {
			if _, err := regexp.Compile(r.Capture); err != nil {
				return fmt.Errorf("rule %q capture: %w", r.Name, err)
			}
		}
		copied := false
		for j, a := range r.Actions {
			kind, _, err := a.kindAndDest()
			if err != nil {
				return fmt.Errorf("rule %q action %d: %w", r.Name, j, err)
			}
			if (kind == "move" || kind == "trash") && j != len(r.Actions)-1 {
				return fmt.Errorf("rule %q: move or trash must be the last action", r.Name)
			}
			if kind == "shell" {
				if copied || a.Rename != "" || a.OnConflict != "" {
					return fmt.Errorf("rule %q: shell must precede copies and cannot use rename or on_conflict", r.Name)
				}
				if err := a.Shell.Validate(); err != nil {
					return fmt.Errorf("rule %q: %w", r.Name, err)
				}
			}
			copied = copied || kind == "copy"
			if a.OnConflict != "" && !validConflict(a.OnConflict) {
				return fmt.Errorf("rule %q: invalid on_conflict %q", r.Name, a.OnConflict)
			}
		}
	}
	for key, value := range c.Vars {
		expanded, err := expandPath(value)
		if err != nil {
			return fmt.Errorf("vars.%s: %w", key, err)
		}
		c.Vars[key] = expanded
	}
	return nil
}

func (a Action) kindAndDest() (string, string, error) {
	kind, dest, count := "", "", 0
	if a.Move != "" {
		kind, dest, count = "move", a.Move, count+1
	}
	if a.Copy != "" {
		kind, dest, count = "copy", a.Copy, count+1
	}
	if a.Trash {
		kind, count = "trash", count+1
	}
	if a.Shell != nil {
		kind, count = "shell", count+1
	}
	if count != 1 {
		return "", "", fmt.Errorf("exactly one action kind is required")
	}
	return kind, dest, nil
}
func validConflict(v string) bool {
	switch v {
	case "rename", "skip", "fail":
		return true
	}
	return false
}
func expandHome(value string) (string, error) {
	if value != "~" && !strings.HasPrefix(value, "~/") {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home + strings.TrimPrefix(value, "~"), nil
}

func expandPath(value string) (string, error) {
	expanded, err := expandHome(value)
	if err != nil {
		return "", err
	}
	return filepath.Abs(expanded)
}
func parseLocation(value string) (*time.Location, error) {
	switch value {
	case "local":
		return time.Local, nil
	case "utc":
		return time.UTC, nil
	default:
		return time.LoadLocation(value)
	}
}
