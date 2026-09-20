package rule

import (
	"fmt"
	"maps"
	"path/filepath"

	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"github.com/mono0x/tamis/internal/meta"
)

type fieldVisitor struct {
	fields  meta.Fields
	dynamic bool
}

func (v *fieldVisitor) Visit(node *ast.Node) {
	if n, ok := (*node).(*ast.IdentifierNode); ok {
		if n.Value == "$env" {
			v.dynamic = true
		}
		v.fields[n.Value] = true
	}
}

func filterFields(condition string) (meta.Fields, error) {
	tree, err := parser.Parse(condition)
	if err != nil {
		return nil, err
	}
	v := fieldVisitor{fields: meta.Fields{}}
	ast.Walk(&tree.Node, &v)
	if v.dynamic {
		return nil, fmt.Errorf("reference metadata by name instead of $env")
	}
	return v.fields, nil
}

func (t Template) validateFields(fields meta.Fields) error {
	for _, part := range t.parts {
		switch part.name {
		case meta.FieldCaptured, meta.FieldMTime, meta.FieldCTime, meta.FieldDateAdded, meta.FieldMake, meta.FieldModel, meta.FieldLens:
			if !fields.Has(part.name) {
				return fmt.Errorf("template metadata %q must be referenced in when", part.name)
			}
		}
	}
	return nil
}

// RequiredFields unions filters whose path scope and filename capture match.
// It does not evaluate metadata conditions or perform filesystem I/O.
func (c *Compiled) RequiredFields(path string) meta.Fields {
	fields := meta.Fields{}
	for _, r := range c.rules {
		if _, ok := r.matcher.Match(path); !ok {
			continue
		}
		if r.capture != nil && !r.capture.MatchString(filepath.Base(path)) {
			continue
		}
		maps.Copy(fields, r.fields)
	}
	return fields
}
