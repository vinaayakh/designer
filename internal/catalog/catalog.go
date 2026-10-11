// Package catalog holds the Component Kind vocabulary, group kinds, Data
// Classification values and the per-cloud Cloud Catalogs, all built in.
package catalog

import (
	"embed"
	"fmt"
	"path"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed data/*.yaml
var files embed.FS

type Kind struct {
	ID        string `yaml:"id" json:"id"`
	Gloss     string `yaml:"gloss" json:"gloss"`
	Icon      string `yaml:"icon" json:"icon"`
	HoldsData bool   `yaml:"holds_data" json:"holdsData,omitempty"`
}

// Category is the Kind's prefix, e.g. "data" for "data.relational".
func (k Kind) Category() string { c, _, _ := strings.Cut(k.ID, "."); return c }

type GroupKind struct {
	ID    string   `yaml:"id" json:"id"`
	Gloss string   `yaml:"gloss" json:"gloss"`
	Icon  string   `yaml:"icon" json:"icon"`
	Tiers []string `yaml:"tiers" json:"tiers,omitempty"`
}

type Service struct {
	ID      string   `yaml:"id" json:"id"`
	Name    string   `yaml:"name" json:"name"`
	Kind    string   `yaml:"kind" json:"kind"`
	Icon    string   `yaml:"icon" json:"icon,omitempty"`
	HIPAA   bool     `yaml:"hipaa" json:"hipaa"`
	Docs    string   `yaml:"docs" json:"docs,omitempty"`
	Aliases []string `yaml:"aliases" json:"aliases,omitempty"`
}

type IconSet struct {
	Version string `yaml:"version" json:"version"`
	URL     string `yaml:"url" json:"url"`
	SHA256  string `yaml:"sha256" json:"sha256"`
}

type Cloud struct {
	Provider   string            `yaml:"provider" json:"provider"`
	Display    string            `yaml:"display" json:"display"`
	GroupNames map[string]string `yaml:"group_names" json:"groupNames"`
	HIPAAAsOf  *string           `yaml:"hipaa_as_of" json:"hipaaAsOf"`
	IconSet    *IconSet          `yaml:"icon_set" json:"iconSet"`
	Services   []Service         `yaml:"services" json:"services"`
}

type Catalog struct {
	Categories  []string          `yaml:"categories" json:"categories"`
	Kinds       []Kind            `yaml:"kinds" json:"kinds"`
	GroupKinds  []GroupKind       `yaml:"group_kinds" json:"groupKinds"`
	DataClasses []string          `yaml:"data_classes" json:"dataClasses"`
	Clouds      map[string]*Cloud `yaml:"-" json:"clouds"`

	kindByID    map[string]*Kind
	serviceByID map[string]*Service
}

// Load reads the built-in catalog and checks it.
func Load() (*Catalog, error) {
	var c Catalog
	b, err := files.ReadFile("data/kinds.yaml")
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("kinds.yaml: %w", err)
	}
	c.Clouds = map[string]*Cloud{}
	entries, _ := files.ReadDir("data")
	for _, e := range entries {
		if e.Name() == "kinds.yaml" {
			continue
		}
		b, _ := files.ReadFile(path.Join("data", e.Name()))
		var cl Cloud
		if err := yaml.Unmarshal(b, &cl); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		c.Clouds[cl.Provider] = &cl
	}
	c.index()
	return &c, c.Check()
}

func (c *Catalog) index() {
	c.kindByID = map[string]*Kind{}
	for i := range c.Kinds {
		c.kindByID[c.Kinds[i].ID] = &c.Kinds[i]
	}
	c.serviceByID = map[string]*Service{}
	for _, cl := range c.Clouds {
		for i := range cl.Services {
			c.serviceByID[cl.Services[i].ID] = &cl.Services[i]
		}
	}
}

// Check enforces the catalog rules from ticket 017.
func (c *Catalog) Check() error {
	var errs []string
	for _, k := range c.Kinds {
		if !slices.Contains(c.Categories, k.Category()) {
			errs = append(errs, fmt.Sprintf("kind %s: unknown category", k.ID))
		}
		if k.Icon == "" {
			errs = append(errs, fmt.Sprintf("kind %s: no generic icon", k.ID))
		}
	}
	for _, g := range c.GroupKinds {
		if g.Icon == "" {
			errs = append(errs, fmt.Sprintf("group kind %s: no generic icon", g.ID))
		}
	}
	seen := map[string]bool{}
	for p, cl := range c.Clouds {
		covered := map[string]bool{}
		for _, s := range cl.Services {
			if seen[s.ID] {
				errs = append(errs, fmt.Sprintf("service %s: duplicate id", s.ID))
			}
			seen[s.ID] = true
			if !strings.HasPrefix(s.ID, p+".") {
				errs = append(errs, fmt.Sprintf("service %s: id must start with %s.", s.ID, p))
			}
			if c.kindByID[s.Kind] == nil {
				errs = append(errs, fmt.Sprintf("service %s: unknown kind %s", s.ID, s.Kind))
			}
			if s.Name == "" {
				errs = append(errs, fmt.Sprintf("service %s: no official name", s.ID))
			}
			covered[s.Kind] = true
		}
		for _, k := range c.Kinds {
			if k.Category() != "external" && !covered[k.ID] {
				errs = append(errs, fmt.Sprintf("%s: no Service for kind %s", p, k.ID))
			}
		}
		if t := EstimateTokens(c.Export(p)); t > 1200 {
			errs = append(errs, fmt.Sprintf("%s: id → kind export is ~%d tokens (max 1200)", p, t))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("catalog:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

func (c *Catalog) Kind(id string) *Kind       { return c.kindByID[id] }
func (c *Catalog) Service(id string) *Service { return c.serviceByID[id] }
func (c *Catalog) GroupKind(id string) *GroupKind {
	for i := range c.GroupKinds {
		if c.GroupKinds[i].ID == id {
			return &c.GroupKinds[i]
		}
	}
	return nil
}

func (c *Catalog) ValidDataClass(v string) bool { return slices.Contains(c.DataClasses, v) }

// KindsPrompt is the Kind list for the static system prompt.
func (c *Catalog) KindsPrompt() string {
	var b strings.Builder
	cat := ""
	for _, k := range c.Kinds {
		if k.Category() != cat {
			cat = k.Category()
			b.WriteString("\n" + cat + ":")
		}
		_, name, _ := strings.Cut(k.ID, ".")
		fmt.Fprintf(&b, " %s (%s)", name, k.Gloss)
		if k.HoldsData {
			b.WriteString("*")
		}
		b.WriteString(";")
	}
	b.WriteString("\ngroups:")
	for _, g := range c.GroupKinds {
		fmt.Fprintf(&b, " %s (%s);", g.ID, g.Gloss)
	}
	return strings.TrimSpace(b.String())
}

// Export is the compact `id → kind` list for one cloud, grouped by Kind,
// which travels in the Design task message.
func (c *Catalog) Export(provider string) string {
	cl := c.Clouds[provider]
	if cl == nil {
		return ""
	}
	by := map[string][]string{}
	for _, s := range cl.Services {
		by[s.Kind] = append(by[s.Kind], strings.TrimPrefix(s.ID, provider+"."))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "catalog %s (Service ids are %s.<name>; kind: names):\n", provider, provider)
	for _, k := range c.Kinds {
		if ids := by[k.ID]; len(ids) > 0 {
			fmt.Fprintf(&b, "%s: %s\n", k.ID, strings.Join(ids, " "))
		}
	}
	return b.String()
}

// EstimateTokens is a rough chars/4 estimate, used only for CI budgets.
func EstimateTokens(s string) int { return (len(s) + 3) / 4 }

// MatchKind reports whether a Kind ID matches a Record glob like "data.*".
func MatchKind(glob, kind string) bool {
	ok, _ := path.Match(glob, kind)
	return ok
}
