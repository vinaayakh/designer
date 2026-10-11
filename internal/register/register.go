// Package register is a Project's Register: the shared, append-only audit trail of requirements,
// assumptions, constraints, non-goals, decisions, risks and open questions (wayfinder/doc-pipeline
// ticket 001).
//
// It is an event log, register/log.jsonl: one JSON event per line, only ever appended. Each event
// carries the SHA-256 of the line before it, so edits made outside Designer are detectable. The
// current state of every entry is computed from the log. An entry's text never changes: a
// correction is a new entry that supersedes the old one.
package register

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

type Type string

const (
	REQ  Type = "REQ"
	ASM  Type = "ASM"
	CON  Type = "CON"
	NG   Type = "NG"
	DEC  Type = "DEC"
	RISK Type = "RISK"
	Q    Type = "Q"
)

// Types in the order documents and the UI list them.
var Types = []Type{REQ, CON, ASM, NG, DEC, RISK, Q}

type Status string

const (
	Open       Status = "open"
	Confirmed  Status = "confirmed"
	Rejected   Status = "rejected"
	Superseded Status = "superseded"
)

// Who did something.
const (
	ActorUser      = "user"
	ActorAssistant = "assistant"
	ActorRuntime   = "runtime"
)

// Source says where an entry came from.
type Source struct {
	Kind string `json:"kind"`           // brief | wizard | chat | assistant | diagram | kb | gap | import | user
	Ref  string `json:"ref,omitempty"`  // Brief field, Step·question, Record ID, element IDs…
	Text string `json:"text,omitempty"` // the chat message an entry came from (chat history isn't kept otherwise)
}

var sourceKinds = []string{"brief", "wizard", "chat", "assistant", "diagram", "kb", "gap", "import", "user"}

// fields are the type-specific fields each entry type may carry, with their allowed values
// (nil = free text).
var fields = map[Type]map[string][]string{
	REQ:  {"kind": {"goal", "functional", "non-functional"}, "priority": {"must", "should", "could"}, "acceptance": nil, "rationale": nil},
	ASM:  {"assumed": nil, "if_wrong": nil},
	CON:  {"origin": {"brief", "regulatory", "technical", "organisational"}},
	NG:   {"kind": {"non-goal", "out-of-scope"}, "reason": nil},
	DEC:  {"chose": nil, "over": nil, "because": nil},
	RISK: {"likelihood": {"low", "medium", "high"}, "impact": {"low", "medium", "high"}, "mitigation": nil, "owner": nil},
	Q:    {"answer": nil},
}

// New is what a caller asks to create.
type New struct {
	Type      Type              `json:"type"`
	Statement string            `json:"statement"`
	Fields    map[string]string `json:"fields,omitempty"`
	Source    Source            `json:"source"`
	Links     []string          `json:"links,omitempty"`
	Status    Status            `json:"status,omitempty"` // the user's own statements start confirmed; the Assistant's always open
}

// Entry is the current state of one entry.
type Entry struct {
	ID           string            `json:"id"`
	Type         Type              `json:"type"`
	Statement    string            `json:"statement"`
	Fields       map[string]string `json:"fields,omitempty"`
	Source       Source            `json:"source"`
	Links        []string          `json:"links,omitempty"`
	Status       Status            `json:"status"`
	Step         string            `json:"step,omitempty"` // the Step that introduced it
	Doc          string            `json:"doc,omitempty"`  // the document version that introduced it
	Created      time.Time         `json:"created"`
	CreatedBy    string            `json:"createdBy"`
	Supersedes   string            `json:"supersedes,omitempty"`
	SupersededBy string            `json:"supersededBy,omitempty"`
	History      []Change          `json:"history"`
}

// Change is one status change in an entry's history.
type Change struct {
	At     time.Time `json:"at"`
	Actor  string    `json:"actor"`
	Status Status    `json:"status"`
	Reason string    `json:"reason,omitempty"`
}

// Event is one line of the log.
type Event struct {
	Seq    int       `json:"seq"`
	At     time.Time `json:"at"`
	Actor  string    `json:"actor"`
	Step   string    `json:"step,omitempty"`
	Doc    string    `json:"doc,omitempty"`
	Op     string    `json:"op"` // create | status | supersede | link | note
	ID     string    `json:"id,omitempty"`
	Entry  *New      `json:"entry,omitempty"`  // create
	Status Status    `json:"status,omitempty"` // status
	By     string    `json:"by,omitempty"`     // supersede: the replacing entry
	Links  []string  `json:"links,omitempty"`  // link
	Reason string    `json:"reason,omitempty"`
	Prev   string    `json:"prev"` // SHA-256 of the previous line ("" for the first)
}

// Register is an open log. Methods are safe for concurrent use.
type Register struct {
	path    string
	mu      sync.Mutex
	events  []Event
	entries map[string]*Entry
	order   []string
	seq     map[Type]int
	last    string // hash of the last line
	broken  string // why the chain doesn't verify, if it doesn't
	notes   []string
	now     func() time.Time
}

// Load reads (or starts) the log at path, checking its hash chain.
func Load(path string) (*Register, error) {
	r := &Register{path: path, entries: map[string]*Entry{}, seq: map[Type]int{}, now: func() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	line := 0
	for sc.Scan() {
		line++
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(b, &ev); err != nil {
			return nil, fmt.Errorf("register line %d: %w", line, err)
		}
		if ev.Prev != r.last && r.broken == "" {
			r.broken = fmt.Sprintf("line %d doesn't follow line %d (edited outside Designer?)", line, line-1)
		}
		if err := r.apply(ev); err != nil && r.broken == "" {
			r.broken = fmt.Sprintf("line %d: %v", line, err)
		}
		r.events = append(r.events, ev)
		r.last = hash(b)
	}
	return r, sc.Err()
}

func hash(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Integrity is "" when the hash chain verifies, else what's wrong.
func (r *Register) Integrity() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.broken
}

// apply folds one event into the current state.
func (r *Register) apply(ev Event) error {
	switch ev.Op {
	case "create":
		if ev.Entry == nil {
			return errors.New("create without an entry")
		}
		t := ev.Entry.Type
		var n int
		if _, err := fmt.Sscanf(strings.TrimPrefix(ev.ID, string(t)+"-"), "%d", &n); err != nil {
			return fmt.Errorf("bad ID %q", ev.ID)
		}
		r.seq[t] = max(r.seq[t], n)
		st := ev.Entry.Status
		if st == "" {
			st = Open
		}
		e := &Entry{ID: ev.ID, Type: t, Statement: ev.Entry.Statement, Fields: ev.Entry.Fields, Source: ev.Entry.Source, Links: ev.Entry.Links,
			Status: st, Step: ev.Step, Doc: ev.Doc, Created: ev.At, CreatedBy: ev.Actor,
			History: []Change{{At: ev.At, Actor: ev.Actor, Status: st, Reason: ev.Reason}}}
		r.entries[ev.ID] = e
		r.order = append(r.order, ev.ID)
	case "status":
		e := r.entries[ev.ID]
		if e == nil {
			return fmt.Errorf("status for unknown %s", ev.ID)
		}
		e.Status = ev.Status
		e.History = append(e.History, Change{At: ev.At, Actor: ev.Actor, Status: ev.Status, Reason: ev.Reason})
	case "supersede":
		e, by := r.entries[ev.ID], r.entries[ev.By]
		if e == nil || by == nil {
			return fmt.Errorf("supersede %s by %s: unknown entry", ev.ID, ev.By)
		}
		e.Status, e.SupersededBy = Superseded, ev.By
		by.Supersedes = ev.ID
		e.History = append(e.History, Change{At: ev.At, Actor: ev.Actor, Status: Superseded, Reason: ev.Reason})
	case "link":
		e := r.entries[ev.ID]
		if e == nil {
			return fmt.Errorf("link for unknown %s", ev.ID)
		}
		for _, l := range ev.Links {
			if !slices.Contains(e.Links, l) {
				e.Links = append(e.Links, l)
			}
		}
	case "note":
		r.notes = append(r.notes, ev.Reason)
	default:
		return fmt.Errorf("unknown op %q", ev.Op)
	}
	return nil
}

// append writes one event (under r.mu) and applies it.
func (r *Register) append(ev Event) error {
	ev.Seq = len(r.events) + 1
	ev.At = r.now()
	ev.Prev = r.last
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if err := r.apply(ev); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	r.events = append(r.events, ev)
	r.last = hash(b)
	return nil
}

// Validate checks a new entry without creating it.
func Validate(n New) error {
	allowed, ok := fields[n.Type]
	if !ok {
		return fmt.Errorf("type must be one of %v", Types)
	}
	if strings.TrimSpace(n.Statement) == "" {
		return errors.New("statement is empty")
	}
	if len(n.Statement) > 600 {
		return errors.New("statement is too long (at most 600 characters)")
	}
	for k, v := range n.Fields {
		values, ok := allowed[k]
		if !ok {
			return fmt.Errorf("%s has no field %q (allowed: %s)", n.Type, k, strings.Join(sortedKeys(allowed), ", "))
		}
		if values != nil && !slices.Contains(values, v) {
			return fmt.Errorf("%s.%s must be one of %s", n.Type, k, strings.Join(values, ", "))
		}
	}
	if !slices.Contains(sourceKinds, n.Source.Kind) {
		return fmt.Errorf("source kind must be one of %s", strings.Join(sourceKinds, ", "))
	}
	if n.Status != "" && n.Status != Open && n.Status != Confirmed {
		return errors.New("a new entry is open or confirmed")
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Create adds an entry and returns it with its new ID. The Assistant's entries are always open;
// only the user's own statements (and runtime copies of them, like Brief answers) start confirmed.
func (r *Register) Create(actor, step, doc string, n New, reason string) (Entry, error) {
	if err := Validate(n); err != nil {
		return Entry{}, err
	}
	if actor == ActorAssistant || n.Status == "" {
		n.Status = Open
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	id := fmt.Sprintf("%s-%03d", n.Type, r.seq[n.Type]+1)
	if err := r.append(Event{Actor: actor, Step: step, Doc: doc, Op: "create", ID: id, Entry: &n, Reason: reason}); err != nil {
		return Entry{}, err
	}
	return *r.entries[id], nil
}

// SetStatus confirms or rejects an entry. Only the user decides.
func (r *Register) SetStatus(actor, id string, s Status, reason string) error {
	if actor != ActorUser {
		return errors.New("only the user confirms or rejects entries")
	}
	if s != Confirmed && s != Rejected {
		return errors.New("status must be confirmed or rejected")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.entries[id]
	if e == nil {
		return fmt.Errorf("no entry %s", id)
	}
	if e.Status == Superseded {
		return fmt.Errorf("%s was superseded by %s", id, e.SupersededBy)
	}
	if e.Status == s {
		return nil
	}
	return r.append(Event{Actor: actor, Op: "status", ID: id, Status: s, Reason: reason})
}

// Supersede creates a replacement for an entry and marks the old one superseded.
func (r *Register) Supersede(actor, step, doc, id string, n New, reason string) (Entry, error) {
	r.mu.Lock()
	old := r.entries[id]
	r.mu.Unlock()
	if old == nil {
		return Entry{}, fmt.Errorf("no entry %s", id)
	}
	if old.Status == Superseded {
		return Entry{}, fmt.Errorf("%s was already superseded by %s", id, old.SupersededBy)
	}
	ne, err := r.Create(actor, step, doc, n, reason)
	if err != nil {
		return Entry{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.append(Event{Actor: actor, Step: step, Doc: doc, Op: "supersede", ID: id, By: ne.ID, Reason: reason}); err != nil {
		return Entry{}, err
	}
	return *r.entries[ne.ID], nil
}

// Link adds links (element IDs, Record IDs, other entries) to an entry.
func (r *Register) Link(actor, id string, links []string, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries[id] == nil {
		return fmt.Errorf("no entry %s", id)
	}
	return r.append(Event{Actor: actor, Op: "link", ID: id, Links: links, Reason: reason})
}

// Note records a runtime marker (e.g. "imported the Design Rationale of v1").
func (r *Register) Note(text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.append(Event{Actor: ActorRuntime, Op: "note", Reason: text})
}

// HasNote reports whether a marker was recorded.
func (r *Register) HasNote(text string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Contains(r.notes, text)
}

// Get returns one entry.
func (r *Register) Get(id string) (Entry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.entries[id]
	if e == nil {
		return Entry{}, false
	}
	return *e, true
}

// Entries returns every entry in creation order.
func (r *Register) Entries() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Entry, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, *r.entries[id])
	}
	return out
}

// Live reports whether an entry still counts (not rejected or superseded).
func (e Entry) Live() bool { return e.Status == Open || e.Status == Confirmed }

// Events returns the raw log (for export).
func (r *Register) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}
