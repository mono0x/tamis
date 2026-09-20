package rule

import (
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/mono0x/tamis/internal/meta"
)

// The expr: tags are what users write in when, and filterFields collects them
// verbatim into the meta.Fields the chain receives. A tag that drifts from the
// name meta asks for still compiles, and would silently stop the field from
// being extracted for preview and apply while scan kept working.
func TestEnvTagsMatchVocabulary(t *testing.T) {
	tags := map[string]bool{}
	env := reflect.TypeFor[Env]()
	for i := range env.NumField() {
		field := env.Field(i)
		tag, ok := field.Tag.Lookup("expr")
		if !ok {
			t.Fatalf("Env.%s has no expr tag", field.Name)
		}
		tags[tag] = true
	}
	vocabulary := map[string]bool{}
	for _, name := range meta.Vocabulary {
		vocabulary[name] = true
	}
	if !maps.Equal(tags, vocabulary) {
		t.Fatalf("expr tags %v do not match meta.Vocabulary %v", slices.Sorted(maps.Keys(tags)), slices.Sorted(maps.Keys(vocabulary)))
	}
}

// The template layer accepts a subset of the vocabulary, so its own lists have
// to name variables envValue actually resolves.
func TestTemplateListsNameResolvableVariables(t *testing.T) {
	for name := range multiComponent {
		if _, err := envValue(name, Env{}); err != nil {
			t.Fatalf("multiComponent %q: %v", name, err)
		}
	}
	for _, name := range []string{meta.FieldCaptured, meta.FieldMTime, meta.FieldCTime, meta.FieldDateAdded, meta.FieldMake, meta.FieldModel, meta.FieldLens} {
		if err := compileTemplate("{{"+name+"}}", nil).validateFields(meta.Fields{}); err == nil {
			t.Fatalf("%q may be used in a template without being declared in when", name)
		}
		if _, err := envValue(name, Env{}); err != nil {
			t.Fatalf("declared template metadata %q: %v", name, err)
		}
	}
}
