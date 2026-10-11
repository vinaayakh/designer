// Package scene calculates the component graph from an Excalidraw scene
// (ADR 0001): Nodes, Edges, Groups and Notes, plus the problems a drawing has.
// Meaning lives in each element's customData.designer.
package scene

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Meta is what Designer stores in an element's customData.designer.
type Meta struct {
	Role        string            `json:"role,omitempty"`        // node | group | edge | icon | label | decor
	Kind        string            `json:"kind,omitempty"`        // Component Kind (node) or group kind (group)
	Service     string            `json:"service,omitempty"`     // Cloud Catalog Service id
	ServiceName string            `json:"serviceName,omitempty"` // off-catalog service name
	Classes     []string          `json:"classes,omitempty"`     // Data Classification
	Attrs       map[string]string `json:"attrs,omitempty"`       // free facts, e.g. encrypted=true
	Parent      string            `json:"parent,omitempty"`      // explicit Group link (production)
	Tier        string            `json:"tier,omitempty"`        // subnet tier
	Style       string            `json:"style,omitempty"`       // edge: sync | async | stream
	Protocol    string            `json:"protocol,omitempty"`    // edge protocol
	Node        string            `json:"node,omitempty"`        // icon: the Node it belongs to
	Label       string            `json:"label,omitempty"`       // edge/group label when not drawn as bound text
}

type Binding struct {
	ElementID string `json:"elementId"`
}

type Element struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	X           float64         `json:"x"`
	Y           float64         `json:"y"`
	Width       float64         `json:"width"`
	Height      float64         `json:"height"`
	IsDeleted   bool            `json:"isDeleted"`
	Text        string          `json:"text"`
	Original    string          `json:"originalText"` // text before wrapping
	ContainerID *string         `json:"containerId"`
	Start       *Binding        `json:"startBinding"`
	End         *Binding        `json:"endBinding"`
	Points      [][]float64     `json:"points"`
	CustomData  json.RawMessage `json:"customData,omitempty"`
	meta        Meta
}

type File struct {
	Type     string            `json:"type"`
	Version  int               `json:"version"`
	Elements []json.RawMessage `json:"elements"`
	AppState json.RawMessage   `json:"appState,omitempty"`
	Files    json.RawMessage   `json:"files,omitempty"`
}

type Rect struct{ X, Y, W, H float64 }

func (r Rect) contains(o Rect) bool {
	return o.X >= r.X && o.Y >= r.Y && o.X+o.W <= r.X+r.W && o.Y+o.H <= r.Y+r.H
}
func (r Rect) overlaps(o Rect) bool {
	return r.X < o.X+o.W && o.X < r.X+r.W && r.Y < o.Y+o.H && o.Y < r.Y+r.H
}
func (r Rect) area() float64              { return r.W * r.H }
func (r Rect) center() (float64, float64) { return r.X + r.W/2, r.Y + r.H/2 }

type Node struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	Kind        string            `json:"kind,omitempty"`
	Service     string            `json:"service,omitempty"`
	ServiceName string            `json:"serviceName,omitempty"`
	Classes     []string          `json:"classes,omitempty"`
	Attrs       map[string]string `json:"attrs,omitempty"`
	Parent      string            `json:"parent,omitempty"`
	Rect        Rect              `json:"-"`
}

type Group struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Kind   string `json:"kind,omitempty"`
	Tier   string `json:"tier,omitempty"`
	Parent string `json:"parent,omitempty"`
	Rect   Rect   `json:"-"`
}

type Edge struct {
	ID       string `json:"id"`
	From     string `json:"from"`
	To       string `json:"to"`
	Label    string `json:"label,omitempty"`
	Style    string `json:"style,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}

type Note struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Near  string `json:"near,omitempty"`
	Loose bool   `json:"loose,omitempty"` // an arrow not connected at both ends
}

// Issue is a structural problem: the Conversion pre-check, or a Production
// parent link that disagrees with the drawing.
type Issue struct {
	Code    string   `json:"code"` // overlap | unlabeled | loose-arrow | dangling-edge | parent-mismatch
	IDs     []string `json:"ids"`
	Message string   `json:"message"`
}

type Graph struct {
	Production bool     `json:"production"`
	Nodes      []*Node  `json:"nodes"`
	Groups     []*Group `json:"groups"`
	Edges      []*Edge  `json:"edges"`
	Notes      []*Note  `json:"notes"`
	Issues     []Issue  `json:"issues"`

	nodeByID  map[string]*Node
	groupByID map[string]*Group
}

func (g *Graph) Node(id string) *Node   { return g.nodeByID[id] }
func (g *Graph) Group(id string) *Group { return g.groupByID[id] }

var shapeTypes = map[string]bool{"rectangle": true, "ellipse": true, "diamond": true, "image": true}

// Parse reads .excalidraw JSON and calculates the graph.
func Parse(data []byte, production bool) (*Graph, error) {
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("scene: %w", err)
	}
	els := make([]*Element, 0, len(f.Elements))
	for _, raw := range f.Elements {
		e := &Element{}
		if err := json.Unmarshal(raw, e); err != nil {
			return nil, fmt.Errorf("scene element: %w", err)
		}
		if e.IsDeleted {
			continue
		}
		if e.Original != "" {
			e.Text = e.Original
		}
		if len(e.CustomData) > 0 {
			var cd struct {
				Designer Meta `json:"designer"`
			}
			_ = json.Unmarshal(e.CustomData, &cd)
			e.meta = cd.Designer
		}
		els = append(els, e)
	}
	return build(els, production), nil
}

func build(els []*Element, production bool) *Graph {
	g := &Graph{Production: production, nodeByID: map[string]*Node{}, groupByID: map[string]*Group{}}
	byID := map[string]*Element{}
	bound := map[string]string{} // container id -> its text
	for _, e := range els {
		byID[e.ID] = e
	}
	for _, e := range els {
		if e.Type == "text" && e.ContainerID != nil && byID[*e.ContainerID] != nil {
			bound[*e.ContainerID] = strings.TrimSpace(e.Text)
		}
		// Production cards draw their label as a separate text element.
		if e.Type == "text" && e.meta.Role == "label" && e.meta.Node != "" {
			bound[e.meta.Node] = strings.TrimSpace(e.Text)
		}
	}
	rect := func(e *Element) Rect { return Rect{e.X, e.Y, e.Width, e.Height} }

	// Shapes: candidate Nodes and Groups.
	var shapes []*Element
	for _, e := range els {
		if !shapeTypes[e.Type] || e.meta.Role == "icon" || e.meta.Role == "decor" {
			continue
		}
		if production && e.meta.Role == "" {
			continue // decoration on a Production Diagram
		}
		shapes = append(shapes, e)
	}

	isGroup := map[string]bool{}
	if production {
		for _, s := range shapes {
			isGroup[s.ID] = s.meta.Role == "group"
		}
	} else {
		// Rough Draft: a shape that fully contains another shape is a Group.
		for _, s := range shapes {
			if s.meta.Role == "group" {
				isGroup[s.ID] = true
				continue
			}
			for _, o := range shapes {
				if o != s && rect(s).contains(rect(o)) && rect(s).area() > rect(o).area() {
					isGroup[s.ID] = true
					break
				}
			}
		}
	}

	label := func(e *Element) string {
		t, ok := bound[e.ID]
		if !ok {
			return e.meta.Label
		}
		// A card shows a shortened label; the full one is kept in customData.
		if cut, found := strings.CutSuffix(t, "…"); found && strings.HasPrefix(e.meta.Label, cut) {
			return e.meta.Label
		}
		if t == "(unnamed)" && e.meta.Label == "" {
			return ""
		}
		return t
	}
	for _, s := range shapes {
		if isGroup[s.ID] {
			gr := &Group{ID: s.ID, Label: label(s), Kind: s.meta.Kind, Tier: s.meta.Tier, Parent: s.meta.Parent, Rect: rect(s)}
			g.Groups = append(g.Groups, gr)
			g.groupByID[s.ID] = gr
		} else {
			n := &Node{ID: s.ID, Label: label(s), Kind: s.meta.Kind, Service: s.meta.Service, ServiceName: s.meta.ServiceName,
				Classes: s.meta.Classes, Attrs: s.meta.Attrs, Parent: s.meta.Parent, Rect: rect(s)}
			g.Nodes = append(g.Nodes, n)
			g.nodeByID[s.ID] = n
		}
	}

	// Membership by position: the smallest Group that fully contains it.
	drawnParent := func(r Rect, self string) string {
		best, bestArea := "", math.Inf(1)
		for _, gr := range g.Groups {
			if gr.ID != self && gr.Rect.contains(r) && gr.Rect.area() < bestArea {
				best, bestArea = gr.ID, gr.Rect.area()
			}
		}
		return best
	}
	if production {
		for _, n := range g.Nodes {
			if d := drawnParent(n.Rect, ""); d != n.Parent {
				g.Issues = append(g.Issues, Issue{"parent-mismatch", []string{n.ID}, fmt.Sprintf("%q is linked to %s but drawn in %s", n.Label, orTop(g, n.Parent), orTop(g, d))})
			}
		}
		for _, gr := range g.Groups {
			if d := drawnParent(gr.Rect, gr.ID); d != gr.Parent {
				g.Issues = append(g.Issues, Issue{"parent-mismatch", []string{gr.ID}, fmt.Sprintf("%q is linked to %s but drawn in %s", gr.Label, orTop(g, gr.Parent), orTop(g, d))})
			}
		}
	} else {
		for _, n := range g.Nodes {
			n.Parent = drawnParent(n.Rect, "")
		}
		for _, gr := range g.Groups {
			gr.Parent = drawnParent(gr.Rect, gr.ID)
		}
		// Partly overlapping: counts as outside, flagged.
		for _, n := range g.Nodes {
			for _, gr := range g.Groups {
				if gr.Rect.overlaps(n.Rect) && !gr.Rect.contains(n.Rect) {
					g.Issues = append(g.Issues, Issue{"overlap", []string{n.ID, gr.ID}, fmt.Sprintf("%q partly overlaps group %q", n.Label, gr.Label)})
				}
			}
		}
		for i, a := range g.Nodes {
			for _, b := range g.Nodes[i+1:] {
				if a.Rect.overlaps(b.Rect) {
					g.Issues = append(g.Issues, Issue{"overlap", []string{a.ID, b.ID}, fmt.Sprintf("%q and %q overlap", a.Label, b.Label)})
				}
			}
		}
	}
	for _, n := range g.Nodes {
		if n.Label == "" && (!production || n.Kind == "") {
			g.Issues = append(g.Issues, Issue{"unlabeled", []string{n.ID}, "a shape has no label"})
		}
	}

	// Arrows: Edges when bound to Nodes (or Groups) at both ends, otherwise Notes.
	endpoint := func(b *Binding) string {
		if b == nil {
			return ""
		}
		if g.nodeByID[b.ElementID] != nil || g.groupByID[b.ElementID] != nil {
			return b.ElementID
		}
		return ""
	}
	for _, e := range els {
		if e.Type != "arrow" {
			continue
		}
		from, to := endpoint(e.Start), endpoint(e.End)
		lbl := bound[e.ID]
		if lbl == "" || production {
			// Production arrows draw their protocol as text; the label lives in customData.
			lbl = e.meta.Label
		}
		switch {
		case from != "" && to != "":
			g.Edges = append(g.Edges, &Edge{ID: e.ID, From: from, To: to, Label: lbl, Style: e.meta.Style, Protocol: e.meta.Protocol})
		case from == "" && to == "":
			g.Issues = append(g.Issues, Issue{"dangling-edge", []string{e.ID}, "an arrow isn't connected to anything"})
			if lbl != "" {
				g.Notes = append(g.Notes, &Note{ID: e.ID, Text: lbl, Loose: true, Near: g.nearest(arrowRect(e))})
			}
		default:
			near := from
			if near == "" {
				near = to
			}
			g.Issues = append(g.Issues, Issue{"loose-arrow", []string{e.ID}, "an arrow is connected at only one end"})
			g.Notes = append(g.Notes, &Note{ID: e.ID, Text: lbl, Loose: true, Near: near})
		}
	}

	// Free text: Notes, placed near the closest Node.
	for _, e := range els {
		if e.Type != "text" || e.meta.Role != "" || e.ContainerID != nil && byID[*e.ContainerID] != nil {
			continue
		}
		if t := strings.TrimSpace(e.Text); t != "" {
			g.Notes = append(g.Notes, &Note{ID: e.ID, Text: t, Near: g.nearest(rect(e))})
		}
	}
	return g
}

func arrowRect(e *Element) Rect {
	minX, minY, maxX, maxY := 0.0, 0.0, 0.0, 0.0
	for _, p := range e.Points {
		if len(p) < 2 {
			continue
		}
		minX, minY, maxX, maxY = math.Min(minX, p[0]), math.Min(minY, p[1]), math.Max(maxX, p[0]), math.Max(maxY, p[1])
	}
	return Rect{e.X + minX, e.Y + minY, maxX - minX, maxY - minY}
}

func orTop(g *Graph, id string) string {
	if id == "" {
		return "the top level"
	}
	if gr := g.groupByID[id]; gr != nil && gr.Label != "" {
		return fmt.Sprintf("%q", gr.Label)
	}
	return "a group"
}

func (g *Graph) nearest(r Rect) string {
	cx, cy := r.center()
	best, bd := "", math.Inf(1)
	for _, n := range g.Nodes {
		nx, ny := n.Rect.center()
		if d := math.Hypot(nx-cx, ny-cy); d < bd {
			best, bd = n.ID, d
		}
	}
	return best
}

// PreCheck returns the structural problems Conversion asks the user to fix.
func (g *Graph) PreCheck() []Issue {
	var out []Issue
	for _, i := range g.Issues {
		if i.Code != "parent-mismatch" {
			out = append(out, i)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Code < out[b].Code })
	return out
}

// Reindex rebuilds the ID lookups after Nodes or Groups were changed in place.
func (g *Graph) Reindex() {
	g.nodeByID = map[string]*Node{}
	g.groupByID = map[string]*Group{}
	for _, n := range g.Nodes {
		g.nodeByID[n.ID] = n
	}
	for _, gr := range g.Groups {
		g.groupByID[gr.ID] = gr
	}
}

// MarshalJSON writes empty lists as [] rather than null, so the browser can
// iterate every field without checks.
func (g *Graph) MarshalJSON() ([]byte, error) {
	type plain Graph
	c := plain(*g)
	if c.Nodes == nil {
		c.Nodes = []*Node{}
	}
	if c.Groups == nil {
		c.Groups = []*Group{}
	}
	if c.Edges == nil {
		c.Edges = []*Edge{}
	}
	if c.Notes == nil {
		c.Notes = []*Note{}
	}
	if c.Issues == nil {
		c.Issues = []Issue{}
	}
	return json.Marshal(c)
}
