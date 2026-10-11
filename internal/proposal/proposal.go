// Package proposal validates the AI's submit_proposal payload against the
// active diagram, resolves Aliases to element IDs, works out which changes
// depend on which, and simulates the result. Geometry (placement, layout)
// is the browser's job; this package only deals with meaning.
package proposal

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"net/url"
	"slices"
	"strings"

	"github.com/vinaayakh/designer/internal/catalog"
	"github.com/vinaayakh/designer/internal/scene"
)

// Change is one operation, as the AI sends it (aliases, +new refs) and as
// the browser receives it (element IDs filled in).
type Change struct {
	ID        string         `json:"id"`
	Op        string         `json:"op"`
	Ref       string         `json:"ref,omitempty"`
	Label     string         `json:"label,omitempty"`
	Kind      string         `json:"kind,omitempty"`
	Service   string         `json:"service,omitempty"`
	ServiceN  string         `json:"service_name,omitempty"`
	Classes   []string       `json:"classes,omitempty"`
	Tier      string         `json:"tier,omitempty"`
	Parent    *string        `json:"parent,omitempty"`
	From      string         `json:"from,omitempty"`
	To        string         `json:"to,omitempty"`
	Style     string         `json:"style,omitempty"`
	Protocol  string         `json:"protocol,omitempty"`
	Set       map[string]any `json:"set,omitempty"`
	Why       string         `json:"why,omitempty"`
	Practices []string       `json:"practices,omitempty"`

	// Filled in by the runtime.
	ElementID string   `json:"elementId,omitempty"` // node/group the change creates or touches
	ParentID  *string  `json:"parentId,omitempty"`
	FromID    string   `json:"fromId,omitempty"`
	ToID      string   `json:"toId,omitempty"`
	Needs     []string `json:"needs,omitempty"` // change ids this one depends on
}

type Citation struct {
	Record string `json:"record,omitempty"`
	URL    string `json:"url,omitempty"`
	Title  string `json:"title,omitempty"`
}

type NodeRationale struct {
	Why       string     `json:"why"`
	Practices []string   `json:"practices,omitempty"`
	Cites     []Citation `json:"cites,omitempty"`
}

type Tradeoff struct {
	Decision string `json:"decision"`
	Chose    string `json:"chose"`
	Over     string `json:"over"`
	Because  string `json:"because"`
}

// Rationale is the Design Rationale. Nodes are keyed by alias when the AI
// submits it and by element ID once stored.
type Rationale struct {
	Summary     string                    `json:"summary"`
	Assumptions []string                  `json:"assumptions,omitempty"`
	Nodes       map[string]*NodeRationale `json:"nodes,omitempty"`
	Tradeoffs   []Tradeoff                `json:"tradeoffs,omitempty"`
}

// Payload is what the AI submits.
type Payload struct {
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	Kind        string     `json:"kind,omitempty"` // "" (a normal Proposal) | conversion
	View        string     `json:"view,omitempty"` // rough | production
	BaseVersion int        `json:"base_version,omitempty"`
	Addresses   []string   `json:"addresses,omitempty"`
	Changes     []*Change  `json:"changes"`
	Rationale   *Rationale `json:"rationale,omitempty"`
}

// Context is what validation checks against.
type Context struct {
	Graph   *scene.Graph
	Aliases *scene.Aliases
	Catalog *catalog.Catalog
	Cloud   string // generic | aws | azure
	// RecordExists and TrustedHost validate citations.
	RecordExists func(id string) bool
	TrustedHost  func(host string) bool
}

var ops = map[string]bool{
	"add_node": true, "add_group": true, "add_edge": true,
	"update_node": true, "update_group": true, "update_edge": true,
	"move_node": true, "remove_node": true, "remove_group": true, "remove_edge": true,
}

var styles = map[string]bool{"sync": true, "async": true, "stream": true}

// Prepare validates a payload and fills in element IDs and dependencies.
// The returned error lists every problem, so the AI can fix them in one go.
func Prepare(p *Payload, c *Context) error {
	var errs []string
	bad := func(ch *Change, f string, a ...any) {
		errs = append(errs, fmt.Sprintf("%s (%s): %s", ch.ID, ch.Op, fmt.Sprintf(f, a...)))
	}
	if strings.TrimSpace(p.Title) == "" {
		errs = append(errs, "title is required")
	}
	if len(p.Changes) == 0 {
		errs = append(errs, "changes is empty")
	}
	conversion := p.Kind == "conversion"
	if conversion && p.Rationale == nil {
		errs = append(errs, "a conversion must include its rationale")
	}

	newRefs := map[string]*Change{} // +ref -> the change creating it
	seenIDs := map[string]bool{}
	for i, ch := range p.Changes {
		if ch.ID == "" {
			ch.ID = fmt.Sprintf("c%d", i+1)
		}
		if seenIDs[ch.ID] {
			bad(ch, "duplicate change id")
		}
		seenIDs[ch.ID] = true
		if !ops[ch.Op] {
			bad(ch, "unknown op (use %s)", strings.Join(slices.Sorted(maps.Keys(ops)), ", "))
			continue
		}
		if ch.Op == "add_node" || ch.Op == "add_group" {
			if !strings.HasPrefix(ch.Ref, "+") || len(ch.Ref) < 2 {
				bad(ch, "new items need a ref like \"+cache\"")
				continue
			}
			if newRefs[ch.Ref] != nil {
				bad(ch, "ref %s is created twice", ch.Ref)
			}
			newRefs[ch.Ref] = ch
			ch.ElementID = newID()
		}
	}

	type target struct {
		id    string
		group bool
		by    *Change // set for +refs
	}
	resolve := func(ref string) (target, bool) {
		if strings.HasPrefix(ref, "+") {
			if ch := newRefs[ref]; ch != nil {
				return target{ch.ElementID, ch.Op == "add_group", ch}, true
			}
			return target{}, false
		}
		id, ok := c.Aliases.ID(ref)
		if !ok {
			return target{}, false
		}
		switch {
		case c.Graph.Node(id) != nil:
			return target{id, false, nil}, true
		case c.Graph.Group(id) != nil:
			return target{id, true, nil}, true
		}
		return target{}, false
	}
	need := func(ch *Change, t target) {
		if t.by != nil && t.by != ch && !slices.Contains(ch.Needs, t.by.ID) {
			ch.Needs = append(ch.Needs, t.by.ID)
		}
	}
	parentOf := func(ch *Change) {
		if ch.Parent == nil {
			return
		}
		if *ch.Parent == "" || *ch.Parent == "null" {
			empty := ""
			ch.ParentID = &empty
			return
		}
		t, ok := resolve(*ch.Parent)
		if !ok || !t.group {
			bad(ch, "parent %q is not a Group", *ch.Parent)
			return
		}
		ch.ParentID = &t.id
		need(ch, t)
	}

	checkKind := func(ch *Change, kind, service, serviceName string, classes []string) {
		if kind == "" {
			return
		}
		k := c.Catalog.Kind(kind)
		if k == nil {
			bad(ch, "unknown Component Kind %q", kind)
			return
		}
		if service != "" {
			s := c.Catalog.Service(service)
			switch {
			case c.Cloud == "generic":
				bad(ch, "the Generic target uses Component Kinds only, no service")
			case s == nil:
				bad(ch, "service %q is not in the %s catalog (use service_name for an off-catalog service)", service, c.Cloud)
			case !strings.HasPrefix(service, c.Cloud+"."):
				bad(ch, "service %q is not a %s service", service, c.Cloud)
			case s.Kind != kind:
				bad(ch, "service %s is a %s, not a %s", service, s.Kind, kind)
			}
		}
		for _, cl := range classes {
			if !c.Catalog.ValidDataClass(cl) {
				bad(ch, "unknown Data Classification %q", cl)
			} else if !k.HoldsData {
				bad(ch, "%s doesn't hold data, so it can't carry %s", kind, cl)
			}
		}
	}
	checkGroupKind := func(ch *Change, kind, tier string) {
		if kind == "" {
			return
		}
		gk := c.Catalog.GroupKind(kind)
		if gk == nil {
			bad(ch, "unknown group kind %q", kind)
			return
		}
		if tier != "" && !slices.Contains(gk.Tiers, tier) {
			bad(ch, "%s has no tier %q", kind, tier)
		}
	}

	for _, ch := range p.Changes {
		if !ops[ch.Op] {
			continue
		}
		switch ch.Op {
		case "add_node":
			if ch.Label == "" {
				bad(ch, "label is required")
			}
			if ch.Kind == "" && (p.View == "production" || conversion) {
				bad(ch, "kind is required on a Production Diagram")
			}
			checkKind(ch, ch.Kind, ch.Service, ch.ServiceN, ch.Classes)
			parentOf(ch)
		case "add_group":
			if p.View == "production" || conversion {
				if ch.Kind == "" {
					bad(ch, "group kind is required")
				}
			}
			checkGroupKind(ch, ch.Kind, ch.Tier)
			parentOf(ch)
		case "add_edge", "update_edge", "remove_edge":
			f, ok1 := resolve(ch.From)
			t, ok2 := resolve(ch.To)
			if !ok1 {
				bad(ch, "from %q doesn't exist", ch.From)
			}
			if !ok2 {
				bad(ch, "to %q doesn't exist", ch.To)
			}
			if !ok1 || !ok2 {
				continue
			}
			ch.FromID, ch.ToID = f.id, t.id
			need(ch, f)
			need(ch, t)
			exists := slices.ContainsFunc(c.Graph.Edges, func(e *scene.Edge) bool { return e.From == f.id && e.To == t.id })
			switch {
			case ch.Op == "add_edge" && exists:
				bad(ch, "%s -> %s already exists (use update_edge)", ch.From, ch.To)
			case ch.Op != "add_edge" && !exists:
				bad(ch, "there is no edge %s -> %s", ch.From, ch.To)
			}
			style := ch.Style
			if s, ok := ch.Set["style"].(string); ok {
				style = s
			}
			if style != "" && !styles[style] {
				bad(ch, "style must be sync, async or stream")
			}
			if ch.Op == "add_edge" && style == "" && (p.View == "production" || conversion) {
				bad(ch, "style is required on a Production Diagram")
			}
		case "update_node", "move_node", "remove_node":
			t, ok := resolve(ch.Ref)
			if !ok || t.group {
				bad(ch, "%q is not an existing Node", ch.Ref)
				continue
			}
			ch.ElementID = t.id
			need(ch, t)
			switch ch.Op {
			case "update_node":
				n := c.Graph.Node(t.id)
				kind, service := n.Kind, n.Service
				if s, ok := ch.Set["kind"].(string); ok {
					kind = s
				}
				if s, ok := ch.Set["service"].(string); ok {
					service = s
				}
				var classes []string
				if arr, ok := ch.Set["classes"].([]any); ok {
					for _, x := range arr {
						classes = append(classes, fmt.Sprint(x))
					}
				} else if kind != n.Kind {
					classes = n.Classes
				}
				checkKind(ch, kind, service, "", classes)
				if len(ch.Set) == 0 && ch.Parent == nil {
					bad(ch, "set is empty")
				}
				parentOf(ch)
			case "move_node":
				if ch.Parent == nil {
					bad(ch, "parent is required (null for the top level)")
				}
				parentOf(ch)
			}
		case "update_group", "remove_group":
			t, ok := resolve(ch.Ref)
			if !ok || !t.group {
				bad(ch, "%q is not an existing Group", ch.Ref)
				continue
			}
			ch.ElementID = t.id
			if ch.Op == "update_group" {
				kind, _ := ch.Set["kind"].(string)
				tier, _ := ch.Set["tier"].(string)
				checkGroupKind(ch, kind, tier)
				parentOf(ch)
			}
		}
	}

	// Removing a Node or Group also removes what hangs off it; an edge added
	// to a Node that is being removed makes no sense.
	removed := map[string]bool{}
	for _, ch := range p.Changes {
		if ch.Op == "remove_node" || ch.Op == "remove_group" {
			removed[ch.ElementID] = true
		}
	}
	for _, ch := range p.Changes {
		if ch.Op == "add_edge" && (removed[ch.FromID] || removed[ch.ToID]) {
			bad(ch, "connects to a Node this Proposal removes")
		}
	}

	if p.Rationale != nil {
		errs = append(errs, checkRationale(p, c, newRefs)...)
	}
	for _, ch := range p.Changes {
		for _, pr := range ch.Practices {
			if c.RecordExists != nil && !c.RecordExists(pr) {
				bad(ch, "practice %q is not a Knowledge Base Record", pr)
			}
		}
	}

	if conversion {
		if res, err := Simulate(c.Graph, p.Changes, nil); err != nil {
			errs = append(errs, err.Error())
		} else {
			for _, n := range res.Nodes {
				if n.Kind == "" {
					errs = append(errs, fmt.Sprintf("Node %s %q has no kind after the conversion", c.Aliases.Of(n.ID, false), n.Label))
				} else if c.Cloud != "generic" && n.Service == "" && n.ServiceName == "" && !strings.HasPrefix(n.Kind, "external.") {
					errs = append(errs, fmt.Sprintf("Node %s %q has no service (use a %s catalog service or service_name)", c.Aliases.Of(n.ID, false), n.Label, c.Cloud))
				}
			}
			for _, g := range res.Groups {
				if g.Kind == "" {
					errs = append(errs, fmt.Sprintf("Group %s %q has no group kind after the conversion", c.Aliases.Of(g.ID, true), g.Label))
				}
			}
			for _, e := range res.Edges {
				if e.Style == "" {
					errs = append(errs, fmt.Sprintf("Edge %s -> %s has no style after the conversion", c.Aliases.Of(e.From, res.Group(e.From) != nil), c.Aliases.Of(e.To, res.Group(e.To) != nil)))
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("the Proposal was not accepted for review:\n- %s", strings.Join(errs, "\n- "))
	}
	return nil
}

func checkRationale(p *Payload, c *Context, newRefs map[string]*Change) []string {
	var errs []string
	r := p.Rationale
	stored := map[string]*NodeRationale{}
	for key, nr := range r.Nodes {
		var id string
		if ch := newRefs[key]; ch != nil {
			id = ch.ElementID
		} else if x, ok := c.Aliases.ID(key); ok {
			id = x
		} else {
			errs = append(errs, fmt.Sprintf("rationale: %q is not a Node", key))
			continue
		}
		for _, cite := range nr.Cites {
			errs = append(errs, checkCite(cite, c)...)
		}
		for _, pr := range nr.Practices {
			if c.RecordExists != nil && !c.RecordExists(pr) {
				errs = append(errs, fmt.Sprintf("rationale %s: practice %q is not a Knowledge Base Record", key, pr))
			}
		}
		stored[id] = nr
	}
	r.Nodes = stored // keyed by element ID from now on
	return errs
}

func checkCite(cite Citation, c *Context) []string {
	switch {
	case cite.Record != "":
		if c.RecordExists != nil && !c.RecordExists(cite.Record) {
			return []string{fmt.Sprintf("citation %q is not a Record in this Knowledge Base", cite.Record)}
		}
	case cite.URL != "":
		u, err := url.Parse(cite.URL)
		if err != nil || u.Host == "" {
			return []string{fmt.Sprintf("citation %q is not a URL", cite.URL)}
		}
		if c.TrustedHost != nil && !c.TrustedHost(u.Hostname()) {
			return []string{fmt.Sprintf("citation host %s is not a Trusted Source", u.Hostname())}
		}
	default:
		return []string{"a citation needs a record or a url"}
	}
	return nil
}

// Closure returns the accepted change IDs after dependencies: a change whose
// prerequisite was skipped is skipped too.
func Closure(changes []*Change, accepted map[string]bool) map[string]bool {
	out := maps.Clone(accepted)
	for changed := true; changed; {
		changed = false
		for _, ch := range changes {
			if !out[ch.ID] {
				continue
			}
			for _, n := range ch.Needs {
				if !out[n] {
					delete(out, ch.ID)
					changed = true
					break
				}
			}
		}
	}
	return out
}

// Simulate applies changes (all, or only accepted ones) to a copy of the
// graph's meaning. It is used to check a Conversion and to compute the
// result the browser must draw.
func Simulate(g *scene.Graph, changes []*Change, accepted map[string]bool) (*scene.Graph, error) {
	out := &scene.Graph{Production: true}
	for _, n := range g.Nodes {
		c := *n
		c.Classes = slices.Clone(n.Classes)
		c.Attrs = maps.Clone(n.Attrs)
		out.Nodes = append(out.Nodes, &c)
	}
	for _, gr := range g.Groups {
		c := *gr
		out.Groups = append(out.Groups, &c)
	}
	for _, e := range g.Edges {
		c := *e
		out.Edges = append(out.Edges, &c)
	}
	out.Notes = g.Notes
	out.Reindex()
	for _, ch := range changes {
		if accepted != nil && !accepted[ch.ID] {
			continue
		}
		applyMeaning(out, ch)
	}
	out.Reindex()
	return out, nil
}

func applyMeaning(g *scene.Graph, ch *Change) {
	parent := func(cur string) string {
		if ch.ParentID != nil {
			return *ch.ParentID
		}
		return cur
	}
	switch ch.Op {
	case "add_node":
		n := &scene.Node{ID: ch.ElementID, Label: ch.Label, Kind: ch.Kind, Service: ch.Service, ServiceName: ch.ServiceN, Classes: ch.Classes, Parent: parent("")}
		ApplyNodeSet(n, ch.Set) // free attributes, e.g. {"encrypted": true}
		g.Nodes = append(g.Nodes, n)
	case "add_group":
		g.Groups = append(g.Groups, &scene.Group{ID: ch.ElementID, Label: ch.Label, Kind: ch.Kind, Tier: ch.Tier, Parent: parent("")})
	case "add_edge":
		g.Edges = append(g.Edges, &scene.Edge{ID: newID(), From: ch.FromID, To: ch.ToID, Style: ch.Style, Protocol: ch.Protocol, Label: ch.Label})
	case "update_node", "move_node":
		for _, n := range g.Nodes {
			if n.ID == ch.ElementID {
				ApplyNodeSet(n, ch.Set)
				n.Parent = parent(n.Parent)
			}
		}
	case "update_group":
		for _, gr := range g.Groups {
			if gr.ID == ch.ElementID {
				if s, ok := ch.Set["label"].(string); ok {
					gr.Label = s
				}
				if s, ok := ch.Set["kind"].(string); ok {
					gr.Kind = s
				}
				if s, ok := ch.Set["tier"].(string); ok {
					gr.Tier = s
				}
				gr.Parent = parent(gr.Parent)
			}
		}
	case "update_edge":
		for _, e := range g.Edges {
			if e.From == ch.FromID && e.To == ch.ToID {
				for k, v := range ch.Set {
					s := fmt.Sprint(v)
					switch k {
					case "style":
						e.Style = s
					case "protocol":
						e.Protocol = s
					case "label":
						e.Label = s
					}
				}
			}
		}
	case "remove_node":
		g.Nodes = slices.DeleteFunc(g.Nodes, func(n *scene.Node) bool { return n.ID == ch.ElementID })
		g.Edges = slices.DeleteFunc(g.Edges, func(e *scene.Edge) bool { return e.From == ch.ElementID || e.To == ch.ElementID })
	case "remove_group":
		var gone *scene.Group
		for _, gr := range g.Groups {
			if gr.ID == ch.ElementID {
				gone = gr
			}
		}
		if gone == nil {
			return
		}
		// Children move up to the removed Group's parent.
		for _, n := range g.Nodes {
			if n.Parent == gone.ID {
				n.Parent = gone.Parent
			}
		}
		for _, gr := range g.Groups {
			if gr.Parent == gone.ID {
				gr.Parent = gone.Parent
			}
		}
		g.Groups = slices.DeleteFunc(g.Groups, func(gr *scene.Group) bool { return gr == gone })
		g.Edges = slices.DeleteFunc(g.Edges, func(e *scene.Edge) bool { return e.From == gone.ID || e.To == gone.ID })
	case "remove_edge":
		g.Edges = slices.DeleteFunc(g.Edges, func(e *scene.Edge) bool { return e.From == ch.FromID && e.To == ch.ToID })
	}
}

// ApplyNodeSet applies an update_node `set`. Known keys change fields; any
// other key is a free attribute (e.g. encrypted: true); null removes it.
func ApplyNodeSet(n *scene.Node, set map[string]any) {
	for k, v := range set {
		switch k {
		case "label":
			n.Label = fmt.Sprint(v)
		case "kind":
			n.Kind = fmt.Sprint(v)
		case "service":
			n.Service = fmt.Sprint(v)
			n.ServiceName = ""
		case "service_name":
			n.ServiceName = fmt.Sprint(v)
			n.Service = ""
		case "classes":
			n.Classes = nil
			if arr, ok := v.([]any); ok {
				for _, x := range arr {
					n.Classes = append(n.Classes, fmt.Sprint(x))
				}
			}
		default:
			if n.Attrs == nil {
				n.Attrs = map[string]string{}
			}
			if v == nil {
				delete(n.Attrs, k)
			} else {
				n.Attrs[k] = fmt.Sprint(v)
			}
		}
	}
}

const idChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_-"

// newID makes an Excalidraw-style random element ID.
func newID() string {
	b := make([]byte, 21)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(idChars))))
		b[i] = idChars[n.Int64()]
	}
	return string(b)
}

// Decode parses the AI's payload, accepting changes as an array or a JSON string.
func Decode(raw json.RawMessage) (*Payload, error) {
	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("payload: %w", err)
	}
	return &p, nil
}

// Same reports whether two changes say the same thing (ignoring ids the
// runtime fills in, and the reason text).
func Same(a, b *Change) bool {
	norm := func(c *Change) string {
		x := *c
		x.ElementID, x.ParentID, x.FromID, x.ToID, x.Needs, x.Why, x.Practices = "", nil, "", "", nil, "", nil
		j, _ := json.Marshal(x)
		return string(j)
	}
	return norm(a) == norm(b)
}
