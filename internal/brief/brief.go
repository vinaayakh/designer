// Package brief holds the Design Brief: its built-in schema, validation,
// the brief.yaml file format and the compact line the AI reads.
package brief

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed schema.yaml
var schemaYAML []byte

const Unknown = "unknown"

type Value struct {
	ID    string `yaml:"id" json:"id"`
	Label string `yaml:"label" json:"label"`
	Hint  string `yaml:"hint,omitempty" json:"hint,omitempty"`
}

type Field struct {
	ID        string  `yaml:"id" json:"id"`
	Short     string  `yaml:"short,omitempty" json:"short,omitempty"`
	Label     string  `yaml:"label" json:"label"`
	Question  string  `yaml:"question" json:"question"`
	Help      string  `yaml:"help,omitempty" json:"help,omitempty"`
	Required  bool    `yaml:"required,omitempty" json:"required,omitempty"`
	Multi     bool    `yaml:"multi,omitempty" json:"multi,omitempty"`
	Exclusive string  `yaml:"exclusive,omitempty" json:"exclusive,omitempty"`
	Screen    string  `yaml:"screen,omitempty" json:"screen,omitempty"`
	Values    []Value `yaml:"values" json:"values"`
}

type Schema struct {
	Version int     `yaml:"version" json:"version"`
	Fields  []Field `yaml:"fields" json:"fields"`
	Notes   struct {
		Label    string `yaml:"label" json:"label"`
		Question string `yaml:"question" json:"question"`
		Max      int    `yaml:"max" json:"max"`
	} `yaml:"notes" json:"notes"`
}

var schema = func() *Schema {
	var s Schema
	if err := yaml.Unmarshal(schemaYAML, &s); err != nil {
		panic("brief schema: " + err.Error())
	}
	return &s
}()

// TheSchema returns the built-in schema.
func TheSchema() *Schema { return schema }

func (s *Schema) Field(id string) *Field {
	for i := range s.Fields {
		if s.Fields[i].ID == id {
			return &s.Fields[i]
		}
	}
	return nil
}

func (f *Field) has(v string) bool {
	return slices.ContainsFunc(f.Values, func(x Value) bool { return x.ID == v })
}

// Brief is one Project's answers. Single-choice fields hold a value ID or
// "unknown"; compliance holds a list. A required field that is still empty
// means the wizard hasn't finished.
type Brief struct {
	Answers    map[string]string `json:"answers"`
	Compliance []string          `json:"compliance"`
	Notes      string            `json:"notes"`
}

// New returns a Brief with every optional field unknown and required fields empty.
func New() *Brief {
	b := &Brief{Answers: map[string]string{}}
	for _, f := range schema.Fields {
		if f.Multi {
			continue
		}
		if f.Required {
			b.Answers[f.ID] = ""
		} else {
			b.Answers[f.ID] = Unknown
		}
	}
	return b
}

// Complete reports whether every required field is answered.
func (b *Brief) Complete() bool {
	for _, f := range schema.Fields {
		if !f.Required {
			continue
		}
		if f.Multi {
			if len(b.Compliance) == 0 {
				return false
			}
		} else if b.Answers[f.ID] == "" {
			return false
		}
	}
	return true
}

// Validate checks every value against the schema. Required fields may be
// empty (wizard not finished); Complete says whether they are all set.
func (b *Brief) Validate() error {
	for k := range b.Answers {
		if f := schema.Field(k); f == nil || f.Multi {
			return fmt.Errorf("unknown brief field %q", k)
		}
	}
	for _, f := range schema.Fields {
		if f.Multi {
			for _, v := range b.Compliance {
				if !f.has(v) {
					return fmt.Errorf("%s: unknown value %q", f.ID, v)
				}
			}
			if f.Exclusive != "" && slices.Contains(b.Compliance, f.Exclusive) && len(b.Compliance) > 1 {
				return fmt.Errorf("%s: %q can't be combined with other values", f.ID, f.Exclusive)
			}
			continue
		}
		v, ok := b.Answers[f.ID]
		if !ok {
			return fmt.Errorf("%s: missing", f.ID)
		}
		switch {
		case v == "" && f.Required:
		case v == Unknown && !f.Required:
		case f.has(v):
		default:
			return fmt.Errorf("%s: unknown value %q", f.ID, v)
		}
	}
	if n := len([]rune(b.Notes)); n > schema.Notes.Max {
		return fmt.Errorf("notes: %d characters (max %d)", n, schema.Notes.Max)
	}
	return nil
}

// Get returns the answer for a single-choice field.
func (b *Brief) Get(id string) string { return b.Answers[id] }

// Matches reports whether this Brief satisfies a Record's `applies_to.brief`
// filter: every listed field must have one of the accepted values, and an
// unknown answer always matches.
func (b *Brief) Matches(filter map[string][]string) bool {
	for field, accepted := range filter {
		if field == "compliance" {
			if !slices.ContainsFunc(b.Compliance, func(c string) bool { return slices.Contains(accepted, c) }) {
				return false
			}
			continue
		}
		v := b.Answers[field]
		if v == Unknown || v == "" {
			continue
		}
		if !slices.Contains(accepted, v) {
			return false
		}
	}
	return true
}

// ValidFilter checks a Record's brief filter against the schema (used by the KB lint).
func ValidFilter(filter map[string][]string) error {
	for field, vals := range filter {
		f := schema.Field(field)
		if f == nil {
			return fmt.Errorf("brief filter: unknown field %q", field)
		}
		for _, v := range vals {
			if !f.has(v) {
				return fmt.Errorf("brief filter: %s has no value %q", field, v)
			}
		}
	}
	return nil
}

// ---- brief.yaml

// Marshal writes brief.yaml with every field present, in schema order.
func (b *Brief) Marshal() []byte {
	var sb strings.Builder
	fmt.Fprintf(&sb, "schema: %d\n", schema.Version)
	for _, f := range schema.Fields {
		if f.Multi {
			fmt.Fprintf(&sb, "%s: [%s]\n", f.ID, strings.Join(b.Compliance, ", "))
			continue
		}
		v := b.Answers[f.ID]
		if v == "" {
			v = `""`
		} else if strings.ContainsAny(v, ".:") || v[0] >= '0' && v[0] <= '9' {
			v = `"` + v + `"`
		}
		fmt.Fprintf(&sb, "%s: %s\n", f.ID, v)
	}
	notes, _ := yaml.Marshal(map[string]string{"notes": b.Notes})
	sb.Write(notes)
	return []byte(sb.String())
}

// Parse reads brief.yaml.
func Parse(data []byte) (*Brief, error) {
	raw := map[string]any{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("brief.yaml: %w", err)
	}
	if v, ok := raw["schema"]; ok && fmt.Sprint(v) != fmt.Sprint(schema.Version) {
		return nil, fmt.Errorf("brief.yaml: schema %v is not supported (want %d)", v, schema.Version)
	}
	b := New()
	for _, f := range schema.Fields {
		v, ok := raw[f.ID]
		if !ok {
			continue
		}
		if f.Multi {
			b.Compliance = nil
			if arr, ok := v.([]any); ok {
				for _, x := range arr {
					b.Compliance = append(b.Compliance, fmt.Sprint(x))
				}
			}
			continue
		}
		if v == nil {
			continue
		}
		b.Answers[f.ID] = fmt.Sprint(v)
	}
	if n, ok := raw["notes"].(string); ok {
		b.Notes = n
	}
	return b, b.Validate()
}

// ---- What the AI reads

// Compact renders the Brief as the AI sees it: known answers on one line,
// unknown fields by name, then notes.
func (b *Brief) Compact() string {
	var known, unknown []string
	for _, f := range schema.Fields {
		name := f.ID
		if f.Short != "" {
			name = f.Short
		}
		if f.Multi {
			known = append(known, name+"="+strings.Join(b.Compliance, ","))
			continue
		}
		v := b.Answers[f.ID]
		if v == Unknown || v == "" {
			unknown = append(unknown, f.ID)
			continue
		}
		known = append(known, name+"="+v)
	}
	s := "brief: " + strings.Join(known, " ")
	if len(unknown) > 0 {
		s += "\nunknown: " + strings.Join(unknown, " ")
	}
	if b.Notes != "" {
		s += fmt.Sprintf("\nnotes: %q", b.Notes)
	}
	return s
}
