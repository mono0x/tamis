// Package plan defines media operation plans and their NDJSON representation.
package plan

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

type OpKind string

const (
	Move  OpKind = "move"
	Copy  OpKind = "copy"
	Trash OpKind = "trash"
	Shell OpKind = "shell"
)

type Op struct {
	Kind       OpKind   `json:"kind"`
	Dest       string   `json:"dest,omitempty"`
	Reason     string   `json:"reason"`
	OnConflict string   `json:"on_conflict,omitempty"`
	Command    *Command `json:"shell,omitempty"`
}

type Command struct {
	Run string            `yaml:"run" json:"run"`
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (c Command) Validate() error {
	if strings.TrimSpace(c.Run) == "" || strings.ContainsRune(c.Run, 0) {
		return fmt.Errorf("shell.run must be nonempty and contain no NUL")
	}
	for name, value := range c.Env {
		if !environmentName.MatchString(name) || name == "TAMIS_FILE" || strings.ContainsRune(value, 0) {
			return fmt.Errorf("invalid shell environment variable %q (TAMIS_FILE is reserved)", name)
		}
	}
	return nil
}

type Item struct {
	Source string `json:"source"`
	Rule   string `json:"rule"`
	Ops    []Op   `json:"ops"`
	// SourceInfo retains the scanned identity for in-process execution.
	SourceInfo os.FileInfo `json:"-"`
}
type SkipRecord struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}
type Plan struct {
	CreatedAt  time.Time    `json:"created_at"`
	ConfigHash string       `json:"config_hash"`
	Items      []Item       `json:"items"`
	Skipped    []SkipRecord `json:"skipped,omitempty"`
}

type line struct {
	Type       string      `json:"type"`
	CreatedAt  time.Time   `json:"created_at,omitempty"`
	ConfigHash string      `json:"config_hash,omitempty"`
	Item       *Item       `json:"item,omitempty"`
	Skip       *SkipRecord `json:"skip,omitempty"`
}

func WriteNDJSON(w io.Writer, p Plan) error {
	enc := json.NewEncoder(w)
	if err := enc.Encode(line{Type: "plan", CreatedAt: p.CreatedAt, ConfigHash: p.ConfigHash}); err != nil {
		return err
	}
	for i := range p.Items {
		if err := enc.Encode(line{Type: "item", Item: &p.Items[i]}); err != nil {
			return err
		}
	}
	for i := range p.Skipped {
		if err := enc.Encode(line{Type: "skip", Skip: &p.Skipped[i]}); err != nil {
			return err
		}
	}
	return nil
}
func ReadNDJSON(r io.Reader) (Plan, error) {
	var p Plan
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	for scanner.Scan() {
		var row line
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return p, err
		}
		switch row.Type {
		case "plan":
			p.CreatedAt = row.CreatedAt
			p.ConfigHash = row.ConfigHash
		case "item":
			if row.Item == nil {
				return p, fmt.Errorf("item line has no item")
			}
			p.Items = append(p.Items, *row.Item)
		case "skip":
			if row.Skip == nil {
				return p, fmt.Errorf("skip line has no skip")
			}
			p.Skipped = append(p.Skipped, *row.Skip)
		default:
			return p, fmt.Errorf("unknown plan line type %q", row.Type)
		}
	}
	return p, scanner.Err()
}
