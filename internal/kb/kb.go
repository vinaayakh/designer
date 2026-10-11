// Package kb loads the built-in Knowledge Base, searches it (hand-written
// BM25) and works out which Records apply to a diagram.
package kb

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Source struct {
	Title       string `yaml:"title" json:"title"`
	URL         string `yaml:"url" json:"url"`
	AsOf        string `yaml:"as_of" json:"asOf"`
	LicenceTier string `yaml:"licence_tier" json:"licenceTier"`
	Attribution string `yaml:"attribution" json:"attribution,omitempty"`
	Copy        string `yaml:"copy" json:"-"`
}

type AppliesTo struct {
	Kinds       []string            `yaml:"kinds" json:"kinds,omitempty"`
	DataClasses []string            `yaml:"data_classes" json:"dataClasses,omitempty"`
	Clouds      []string            `yaml:"clouds" json:"clouds,omitempty"`
	Brief       map[string][]string `yaml:"brief" json:"brief,omitempty"`
}

type Record struct {
	ID          string    `yaml:"id" json:"id"`
	Type        string    `yaml:"type" json:"type"` // control | practice
	Domain      string    `yaml:"domain" json:"domain,omitempty"`
	Regime      string    `yaml:"regime" json:"regime,omitempty"`
	Ref         string    `yaml:"ref" json:"ref,omitempty"`
	Obligation  string    `yaml:"obligation" json:"obligation,omitempty"`
	Title       string    `yaml:"title" json:"title"`
	Summary     string    `yaml:"summary" json:"summary"`
	Aliases     []string  `yaml:"aliases" json:"aliases,omitempty"`
	Coverage    string    `yaml:"coverage" json:"coverage"`
	AppliesTo   AppliesTo `yaml:"applies_to" json:"appliesTo"`
	SatisfiedBy []string  `yaml:"satisfied_by" json:"satisfiedBy,omitempty"`
	Sources     []Source  `yaml:"sources" json:"sources"`
	Revised     string    `yaml:"revised" json:"revised"`
	Body        string    `yaml:"-" json:"body"`
	Path        string    `yaml:"-" json:"-"`

	Satisfies []string `yaml:"-" json:"satisfies,omitempty"` // reverse of satisfied_by, built at load
}

type KB struct {
	Records []*Record
	Version string // date version: the newest `revised`
	byID    map[string]*Record
	ix      *index
}

// Load reads every Record under dir in fsys.
func Load(fsys fs.FS, dir string) (*KB, error) {
	k := &KB{byID: map[string]*Record{}}
	err := fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		r, err := parse(b)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		r.Path = p
		k.Records = append(k.Records, r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(k.Records, func(i, j int) bool { return k.Records[i].ID < k.Records[j].ID })
	for _, r := range k.Records {
		k.byID[r.ID] = r
		if r.Revised > k.Version {
			k.Version = r.Revised
		}
	}
	for _, r := range k.Records {
		for _, p := range r.SatisfiedBy {
			if pr := k.byID[p]; pr != nil {
				pr.Satisfies = append(pr.Satisfies, r.ID)
			}
		}
	}
	k.ix = build(k.Records)
	return k, nil
}

func parse(b []byte) (*Record, error) {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return nil, fmt.Errorf("no frontmatter")
	}
	front, body, ok := strings.Cut(s[4:], "\n---\n")
	if !ok {
		return nil, fmt.Errorf("unterminated frontmatter")
	}
	r := &Record{}
	if err := yaml.Unmarshal([]byte(front), r); err != nil {
		return nil, err
	}
	r.Body = strings.TrimSpace(body)
	return r, nil
}

func (k *KB) Get(id string) *Record { return k.byID[id] }

// ---- Lint (run by CI through the tests)

var headRe = regexp.MustCompile(`(?m)^## (.+)$`)

// Lint checks the record format rules. kindOK and briefOK validate Kind globs
// and brief filters against the catalog and brief schema.
func (k *KB) Lint(kindOK func(glob string) bool, briefOK func(map[string][]string) error, dataClassOK func(string) bool) []string {
	var probs []string
	add := func(r *Record, f string, a ...any) { probs = append(probs, r.ID+": "+fmt.Sprintf(f, a...)) }
	for _, r := range k.Records {
		dir := path.Base(path.Dir(r.Path))
		if want := dir + "." + strings.TrimSuffix(path.Base(r.Path), ".md"); want != r.ID {
			add(r, "id does not mirror path (%s)", want)
		}
		if r.Type != "control" && r.Type != "practice" {
			add(r, "type %q", r.Type)
		}
		if n := len(strings.Fields(r.Summary)); n > 25 {
			add(r, "summary %d words (>25)", n)
		}
		var heads []string
		for _, m := range headRe.FindAllStringSubmatch(r.Body, -1) {
			heads = append(heads, m[1])
		}
		want := []string{"Requirement", "How to check on a diagram", "Suggested fixes"}
		if r.Type == "practice" {
			want = []string{"Practice", "How to check on a diagram", "Suggested fixes", "Trade-offs"}
		}
		if !slices.Equal(heads, want) {
			add(r, "sections %v, want %v", heads, want)
		}
		switch r.Coverage {
		case "checkable", "partial", "process":
		default:
			add(r, "coverage %q", r.Coverage)
		}
		for _, s := range r.SatisfiedBy {
			if k.byID[s] == nil {
				add(r, "satisfied_by %s does not resolve", s)
			}
		}
		if r.Type == "practice" && len(r.SatisfiedBy) > 0 {
			add(r, "satisfied_by on a practice")
		}
		for _, g := range r.AppliesTo.Kinds {
			if !kindOK(g) {
				add(r, "applies_to.kinds %q matches no Component Kind", g)
			}
		}
		for _, d := range r.AppliesTo.DataClasses {
			if !dataClassOK(d) {
				add(r, "applies_to.data_classes %q is not a Data Classification", d)
			}
		}
		if err := briefOK(r.AppliesTo.Brief); err != nil {
			add(r, "%v", err)
		}
		for _, s := range r.Sources {
			switch s.LicenceTier {
			case "public-domain", "facts-only":
			case "attribution":
				if s.Attribution == "" {
					add(r, "attribution source without attribution text")
				}
			default:
				add(r, "licence_tier %q", s.LicenceTier)
			}
		}
	}
	return probs
}
