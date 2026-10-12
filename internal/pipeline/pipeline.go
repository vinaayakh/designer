// Package pipeline is the Document Pipeline (wayfinder/doc-pipeline): Steps that turn a Project
// into standard engineering documents, each built on the outputs of the Steps before it.
//
// Steps are data: one YAML file each in steps/, built into the binary. A Step declares what it
// needs (the questions its wizard asks, the upstream Steps it requires or uses, the derivers that
// compute facts without tokens) and the sections it produces. Answers become Register entries;
// documents are versions written once to documents/<step>/vNNN.json with a Markdown copy beside
// them, and each records what it was built from, so it can tell when it has gone stale.
package pipeline

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/vinaayakh/designer/internal/register"
	"gopkg.in/yaml.v3"
)

//go:embed steps/*.yaml
var stepFiles embed.FS

// Question is one wizard question. Its answer becomes Register entries of type Creates (one per
// line for lists and per value for multi-choice), confirmed, with Fields set.
type Question struct {
	ID       string            `yaml:"id" json:"id"`
	Label    string            `yaml:"label" json:"label"`
	Question string            `yaml:"question" json:"question"`
	Kind     string            `yaml:"kind" json:"kind"` // text | choice | multi | list
	Values   []string          `yaml:"values" json:"values,omitempty"`
	Blocking bool              `yaml:"blocking" json:"blocking,omitempty"`
	Creates  register.Type     `yaml:"creates" json:"creates,omitempty"` // "" = document control only
	Fields   map[string]string `yaml:"fields" json:"fields,omitempty"`
	// Prefix starts each entry's statement (e.g. "Rejected: "), and ValueField also stores the
	// answer in that field (e.g. a rejected alternative is the DEC's "over").
	Prefix     string `yaml:"prefix" json:"prefix,omitempty"`
	ValueField string `yaml:"value_field" json:"valueField,omitempty"`
}

// SectionDef is one section of a Step's document.
type SectionDef struct {
	ID       string              `yaml:"id" json:"id"`
	Title    string              `yaml:"title" json:"title"`
	Kind     string              `yaml:"kind" json:"kind"` // prose | list | table (written) · entries | derived (filled by the runtime)
	Guidance string              `yaml:"guidance" json:"guidance,omitempty"`
	Columns  []string            `yaml:"columns" json:"columns,omitempty"`
	Types    []register.Type     `yaml:"types" json:"types,omitempty"`
	Filter   map[string][]string `yaml:"filter" json:"filter,omitempty"` // entries: field → allowed values
	Deriver  string              `yaml:"deriver" json:"deriver,omitempty"`
	// Scope "relied" lists only the entries the document cites in its text or adds itself;
	// otherwise an entries section lists every live entry of its types.
	Scope string `yaml:"scope" json:"scope,omitempty"`
}

// Written reports whether the Assistant writes this section.
func (s SectionDef) Written() bool { return s.Kind == "prose" || s.Kind == "list" || s.Kind == "table" }

// Step is one document's definition.
type Step struct {
	ID           string          `yaml:"id" json:"id"`
	Title        string          `yaml:"title" json:"title"`
	Version      int             `yaml:"version" json:"version"`
	Template     string          `yaml:"template" json:"template"`
	Credit       string          `yaml:"credit" json:"credit,omitempty"`
	Diagram      string          `yaml:"diagram" json:"diagram"` // rough | production
	Requires     []string        `yaml:"requires" json:"requires,omitempty"`
	Uses         []string        `yaml:"uses" json:"uses,omitempty"`
	Derive       []string        `yaml:"derive" json:"derive,omitempty"` // derivers for the prompt only
	Creates      []register.Type `yaml:"creates" json:"creates"`
	Instructions string          `yaml:"instructions" json:"-"`
	Questions    []Question      `yaml:"questions" json:"questions"`
	Sections     []SectionDef    `yaml:"sections" json:"sections"`
}

var (
	steps   []*Step
	idRe    = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	order   = []string{"requirements", "design-doc", "threat-model", "reliability"}
	kinds   = []string{"text", "choice", "multi", "list"}
	secKind = []string{"prose", "list", "table", "entries", "derived"}
)

func init() {
	var err error
	steps, err = load()
	if err != nil {
		panic("pipeline: " + err.Error())
	}
}

func load() ([]*Step, error) {
	byID := map[string]*Step{}
	for _, id := range order {
		b, err := stepFiles.ReadFile("steps/" + id + ".yaml")
		if err != nil {
			return nil, err
		}
		var s Step
		if err := yaml.Unmarshal(b, &s); err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
		if err := s.check(); err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
		byID[s.ID] = &s
	}
	var out []*Step
	for _, id := range order {
		s := byID[id]
		for _, r := range append(slices.Clone(s.Requires), s.Uses...) {
			if byID[r] == nil || slices.Index(order, r) >= slices.Index(order, id) {
				return nil, fmt.Errorf("%s: %q must be an earlier Step", id, r)
			}
		}
		out = append(out, s)
	}
	return out, nil
}

func (s *Step) check() error {
	if !idRe.MatchString(s.ID) || s.Title == "" || s.Version < 1 || s.Template == "" {
		return errors.New("needs id, title, version and template")
	}
	if s.Diagram != "rough" && s.Diagram != "production" {
		return errors.New("diagram must be rough or production")
	}
	seen := map[string]bool{}
	for _, q := range s.Questions {
		if !idRe.MatchString(q.ID) || seen["q:"+q.ID] || q.Label == "" || q.Question == "" || !slices.Contains(kinds, q.Kind) {
			return fmt.Errorf("question %q: needs a unique id, label, question and a kind in %v", q.ID, kinds)
		}
		seen["q:"+q.ID] = true
		if (q.Kind == "choice" || q.Kind == "multi") && len(q.Values) == 0 {
			return fmt.Errorf("question %q: needs values", q.ID)
		}
		if q.Creates != "" {
			if err := register.Validate(register.New{Type: q.Creates, Statement: "x", Fields: q.Fields, Source: register.Source{Kind: "wizard"}}); err != nil {
				return fmt.Errorf("question %q: %w", q.ID, err)
			}
		}
	}
	for _, sec := range s.Sections {
		if !idRe.MatchString(sec.ID) || seen["s:"+sec.ID] || sec.Title == "" || !slices.Contains(secKind, sec.Kind) {
			return fmt.Errorf("section %q: needs a unique id, title and a kind in %v", sec.ID, secKind)
		}
		seen["s:"+sec.ID] = true
		switch sec.Kind {
		case "table":
			if len(sec.Columns) == 0 {
				return fmt.Errorf("section %q: a table needs columns", sec.ID)
			}
		case "entries":
			if len(sec.Types) == 0 {
				return fmt.Errorf("section %q: entries need types", sec.ID)
			}
		case "derived":
			if derivers[sec.Deriver] == nil {
				return fmt.Errorf("section %q: unknown deriver %q", sec.ID, sec.Deriver)
			}
		}
	}
	for _, d := range s.Derive {
		if derivers[d] == nil {
			return fmt.Errorf("unknown deriver %q", d)
		}
	}
	return nil
}

// Steps returns every Step in pipeline order.
func Steps() []*Step { return steps }

// Get returns one Step, or nil.
func Get(id string) *Step {
	for _, s := range steps {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// Question returns one of the Step's questions, or nil.
func (s *Step) Question(id string) *Question {
	for i := range s.Questions {
		if s.Questions[i].ID == id {
			return &s.Questions[i]
		}
	}
	return nil
}

// ---- answers

// Answer is what the user said to one question.
type Answer struct {
	Values  []string  `json:"values,omitempty"`
	Unknown bool      `json:"unknown,omitempty"`
	At      time.Time `json:"at"`
	Entries []string  `json:"entries,omitempty"` // the Register entries the answer created
}

// Answers are by Step, then question.
type Answers map[string]map[string]Answer

// Store reads and writes a Project's pipeline files under its root folder.
type Store struct{ Dir string }

func (st Store) path(parts ...string) string {
	return filepath.Join(append([]string{st.Dir, "documents"}, parts...)...)
}

func writeAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadAnswers reads documents/answers.json (empty if there is none).
func (st Store) LoadAnswers() (Answers, error) {
	a := Answers{}
	b, err := os.ReadFile(st.path("answers.json"))
	if errors.Is(err, os.ErrNotExist) {
		return a, nil
	}
	if err != nil {
		return nil, err
	}
	return a, json.Unmarshal(b, &a)
}

// SaveAnswers writes documents/answers.json.
func (st Store) SaveAnswers(a Answers) error {
	b, _ := json.MarshalIndent(a, "", "  ")
	return writeAtomic(st.path("answers.json"), b)
}

// Values cleans a raw answer for a question: lines for lists, one of the values for choices.
func (q *Question) Clean(raw []string) ([]string, error) {
	var out []string
	for _, r := range raw {
		for _, line := range strings.Split(r, "\n") {
			if l := strings.TrimSpace(line); l != "" {
				out = append(out, l)
			}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("the answer is empty")
	}
	switch q.Kind {
	case "text":
		out = []string{strings.Join(out, "\n")}
	case "choice":
		if len(out) != 1 || !slices.Contains(q.Values, out[0]) {
			return nil, fmt.Errorf("choose one of: %s", strings.Join(q.Values, ", "))
		}
	case "multi":
		for _, v := range out {
			if !slices.Contains(q.Values, v) {
				return nil, fmt.Errorf("%q isn't one of: %s", v, strings.Join(q.Values, ", "))
			}
		}
	}
	return out, nil
}

// ---- document versions

// SectionOut is one section of a written document.
type SectionOut struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Kind    string     `json:"kind"`
	Body    string     `json:"body,omitempty"`
	Items   []string   `json:"items,omitempty"`
	Columns []string   `json:"columns,omitempty"`
	Rows    [][]string `json:"rows,omitempty"`
	Entries []string   `json:"entries,omitempty"` // entries sections: the Register entries listed
	Cites   []string   `json:"cites,omitempty"`
}

// Inputs record what a version was built from.
type Inputs struct {
	BriefHash  string         `json:"briefHash"`
	Production int            `json:"production,omitempty"` // 0 when the Step reads the Rough Draft
	Upstream   map[string]int `json:"upstream,omitempty"`   // Step → version number used
	AnswersAt  time.Time      `json:"answersAt"`
}

// Version is one written document, never edited after it's saved.
type Version struct {
	Step        string                     `json:"step"`
	N           int                        `json:"n"`
	Created     time.Time                  `json:"created"`
	StepVersion int                        `json:"stepVersion"`
	Model       string                     `json:"model,omitempty"`
	Inputs      Inputs                     `json:"inputs"`
	Cites       map[string]register.Status `json:"cites"`                // every entry it relies on, with its status when written
	Records     []string                   `json:"records,omitempty"`    // Knowledge Base Records it draws on
	NewEntries  []string                   `json:"newEntries,omitempty"` // entries this version proposed
	Summary     string                     `json:"summary"`              // a few lines for downstream Steps
	Sections    []SectionOut               `json:"sections"`
}

var verRe = regexp.MustCompile(`^v(\d{3,})\.json$`)

// Versions lists a Step's version numbers, oldest first.
func (st Store) Versions(step string) ([]int, error) {
	ents, err := os.ReadDir(st.path(step))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []int
	for _, e := range ents {
		if m := verRe.FindStringSubmatch(e.Name()); m != nil {
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out, nil
}

// Version reads one version.
func (st Store) Version(step string, n int) (*Version, error) {
	b, err := os.ReadFile(st.path(step, fmt.Sprintf("v%03d.json", n)))
	if err != nil {
		return nil, err
	}
	var v Version
	return &v, json.Unmarshal(b, &v)
}

// Latest reads a Step's newest version, or nil when it has none.
func (st Store) Latest(step string) (*Version, error) {
	ns, err := st.Versions(step)
	if err != nil || len(ns) == 0 {
		return nil, err
	}
	return st.Version(step, ns[len(ns)-1])
}

// Save writes a new version (numbered after the last) and its Markdown copy.
func (st Store) Save(v *Version, markdown string) error {
	ns, err := st.Versions(v.Step)
	if err != nil {
		return err
	}
	v.N = 1
	if len(ns) > 0 {
		v.N = ns[len(ns)-1] + 1
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	if err := writeAtomic(st.path(v.Step, fmt.Sprintf("v%03d.json", v.N)), b); err != nil {
		return err
	}
	return writeAtomic(st.path(v.Step, fmt.Sprintf("v%03d.md", v.N)), []byte(markdown))
}

// ---- status (ticket 002)

// Status is where a Step stands.
type Status struct {
	Kind       string   `json:"kind"` // waiting | needs | ready | current | stale (the app adds writing)
	Text       string   `json:"text"`
	Reasons    []string `json:"reasons,omitempty"`
	Unanswered int      `json:"unanswered"`
	Blocking   []string `json:"blocking,omitempty"` // blocking questions without an answer
}

// Now is the current state a Step's status is judged against.
type Now struct {
	BriefHash  string
	Production int
	Latest     map[string]*Version // every Step's newest version
	Entries    map[string]register.Entry
	Answers    Answers
}

// StatusOf works out a Step's status.
func StatusOf(s *Step, now Now) Status {
	var missing []string
	for _, r := range s.Requires {
		if now.Latest[r] == nil {
			missing = append(missing, Get(r).Title)
		}
	}
	if len(missing) > 0 {
		return Status{Kind: "waiting", Text: "Waits on " + strings.Join(missing, ", ")}
	}
	st := Status{}
	for _, q := range s.Questions {
		a, ok := now.Answers[s.ID][q.ID]
		if !ok {
			st.Unanswered++
		}
		if q.Blocking && (!ok || a.Unknown) {
			st.Blocking = append(st.Blocking, q.Label)
		}
	}
	v := now.Latest[s.ID]
	if v == nil {
		switch {
		case len(st.Blocking) > 0 || st.Unanswered > 0:
			st.Kind = "needs"
			st.Text = fmt.Sprintf("%d question%s to answer", st.Unanswered, plural(st.Unanswered))
			if len(st.Blocking) > 0 {
				st.Text += fmt.Sprintf(" (%d required)", len(st.Blocking))
			}
		default:
			st.Kind, st.Text = "ready", "Ready to write"
		}
		return st
	}
	if v.StepVersion < s.Version {
		st.Reasons = append(st.Reasons, "the document's structure has changed since it was written")
	}
	if s.Diagram == "production" && now.Production > v.Inputs.Production {
		st.Reasons = append(st.Reasons, fmt.Sprintf("Production is now v%d (built on v%d)", now.Production, v.Inputs.Production))
	}
	if now.BriefHash != v.Inputs.BriefHash {
		st.Reasons = append(st.Reasons, "the Design Brief changed")
	}
	for _, up := range append(slices.Clone(s.Requires), s.Uses...) {
		if l := now.Latest[up]; l != nil && l.N > v.Inputs.Upstream[up] {
			st.Reasons = append(st.Reasons, fmt.Sprintf("%s is now v%d (built on v%d)", Get(up).Title, l.N, v.Inputs.Upstream[up]))
		}
	}
	for _, id := range sortedKeys(v.Cites) {
		if e, ok := now.Entries[id]; ok && !e.Live() {
			st.Reasons = append(st.Reasons, fmt.Sprintf("%s was %s", id, e.Status))
		}
	}
	for _, a := range now.Answers[s.ID] {
		if a.At.After(v.Inputs.AnswersAt) { // answered after the answers it was written from
			st.Reasons = append(st.Reasons, "answers changed since it was written")
			break
		}
	}
	if len(st.Reasons) > 0 {
		st.Kind = "stale"
		st.Text = "Stale: " + st.Reasons[0]
		if len(st.Reasons) > 1 {
			st.Text += fmt.Sprintf(" (+%d more)", len(st.Reasons)-1)
		}
		return st
	}
	st.Kind, st.Text = "current", fmt.Sprintf("v%d · current", v.N)
	return st
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// EntriesFor returns the live entries a section of kind "entries" lists, in Register order.
func EntriesFor(sec SectionDef, all []register.Entry) []register.Entry {
	var out []register.Entry
	for _, e := range all {
		if !e.Live() || !slices.Contains(sec.Types, e.Type) {
			continue
		}
		ok := true
		for f, vals := range sec.Filter {
			if !slices.Contains(vals, e.Fields[f]) {
				ok = false
			}
		}
		if ok {
			out = append(out, e)
		}
	}
	return out
}
