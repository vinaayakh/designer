package scene

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// Aliases maps element IDs to the short handles (n1, g2) the AI uses. One
// set lives for a whole Assistant Session, so prompt caching and change-only
// updates keep working.
type Aliases struct {
	ByID  map[string]string `json:"byId"`
	NextN int               `json:"nextN"`
	NextG int               `json:"nextG"`
	byAl  map[string]string
}

func NewAliases() *Aliases { return &Aliases{ByID: map[string]string{}} }

func (a *Aliases) Of(id string, group bool) string {
	if al, ok := a.ByID[id]; ok {
		return al
	}
	var al string
	if group {
		a.NextG++
		al = fmt.Sprintf("g%d", a.NextG)
	} else {
		a.NextN++
		al = fmt.Sprintf("n%d", a.NextN)
	}
	a.ByID[id] = al
	a.byAl = nil
	return al
}

// ID resolves an alias back to an element ID.
func (a *Aliases) ID(alias string) (string, bool) {
	if a.byAl == nil {
		a.byAl = map[string]string{}
		for id, al := range a.ByID {
			a.byAl[al] = id
		}
	}
	id, ok := a.byAl[alias]
	return id, ok
}

// Peek returns an alias without assigning one.
func (a *Aliases) Peek(id string) string { return a.ByID[id] }

func q(s string) string { return fmt.Sprintf("%q", s) }

func (g *Graph) nodeLine(n *Node, a *Aliases) string {
	var b strings.Builder
	b.WriteString(a.Of(n.ID, false) + " " + q(n.Label))
	if n.Kind != "" {
		b.WriteString(" " + n.Kind)
	}
	if n.Service != "" {
		b.WriteString(" " + n.Service)
	} else if n.ServiceName != "" {
		b.WriteString(" ~" + strings.ReplaceAll(n.ServiceName, " ", "_"))
	}
	if len(n.Classes) > 0 {
		b.WriteString(" [" + strings.Join(n.Classes, ",") + "]")
	}
	if len(n.Attrs) > 0 {
		var kv []string
		for _, k := range slices.Sorted(maps.Keys(n.Attrs)) {
			if v := n.Attrs[k]; v == "true" {
				kv = append(kv, k)
			} else {
				kv = append(kv, k+"="+v)
			}
		}
		b.WriteString(" {" + strings.Join(kv, " ") + "}")
	}
	return b.String()
}

func (g *Graph) groupLine(gr *Group, a *Aliases) string {
	s := a.Of(gr.ID, true)
	if gr.Kind != "" {
		s += " " + gr.Kind
		if gr.Tier != "" {
			s += ":" + gr.Tier
		}
	}
	return s + " " + q(gr.Label)
}

func (g *Graph) edgeLine(e *Edge, a *Aliases) string {
	s := g.aliasOf(e.From, a) + " -> " + g.aliasOf(e.To, a)
	if e.Style != "" {
		s += " " + e.Style
	}
	if e.Protocol != "" {
		s += " " + e.Protocol
	}
	if e.Label != "" {
		s += " " + q(e.Label)
	}
	return s
}

func (g *Graph) noteLine(n *Note, a *Aliases) string {
	s := "note"
	if n.Loose {
		s += " (loose arrow)"
	}
	s += " " + q(n.Text)
	if n.Near != "" {
		s += " near " + g.aliasOf(n.Near, a)
	}
	return s
}

func (g *Graph) aliasOf(id string, a *Aliases) string {
	return a.Of(id, g.groupByID[id] != nil)
}

// children returns the Nodes and Groups whose parent is id, in reading
// order (top to bottom, then left to right).
func (g *Graph) children(id string) (groups []*Group, nodes []*Node) {
	for _, gr := range g.Groups {
		if gr.Parent == id {
			groups = append(groups, gr)
		}
	}
	for _, n := range g.Nodes {
		if n.Parent == id {
			nodes = append(nodes, n)
		}
	}
	byPos := func(a, b Rect) bool {
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		return a.X < b.X
	}
	sort.SliceStable(groups, func(i, j int) bool { return byPos(groups[i].Rect, groups[j].Rect) })
	sort.SliceStable(nodes, func(i, j int) bool { return byPos(nodes[i].Rect, nodes[j].Rect) })
	return
}

// Compact renders the whole diagram in the line format the AI reads: no
// coordinates or styling, indentation shows Group nesting.
func (g *Graph) Compact(a *Aliases) string {
	var b strings.Builder
	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		groups, nodes := g.children(id)
		pad := strings.Repeat("  ", depth)
		for _, gr := range groups {
			b.WriteString(pad + g.groupLine(gr, a) + "\n")
			walk(gr.ID, depth+1)
		}
		for _, n := range nodes {
			b.WriteString(pad + g.nodeLine(n, a) + "\n")
		}
	}
	walk("", 0)
	for _, e := range g.sortedEdges(a) {
		b.WriteString(g.edgeLine(e, a) + "\n")
	}
	for _, n := range g.Notes {
		b.WriteString(g.noteLine(n, a) + "\n")
	}
	for _, i := range g.Issues {
		b.WriteString(g.issueLine(i, a) + "\n")
	}
	if b.Len() == 0 {
		return "(empty diagram)\n"
	}
	return b.String()
}

func (g *Graph) sortedEdges(a *Aliases) []*Edge {
	// Make sure every endpoint has an alias first, so ordering is stable.
	g.Compactless(a)
	es := slices.Clone(g.Edges)
	sort.SliceStable(es, func(i, j int) bool { return g.edgeLine(es[i], a) < g.edgeLine(es[j], a) })
	return es
}

// Compactless assigns aliases in reading order without rendering.
func (g *Graph) Compactless(a *Aliases) {
	var walk func(id string)
	walk = func(id string) {
		groups, nodes := g.children(id)
		for _, gr := range groups {
			a.Of(gr.ID, true)
			walk(gr.ID)
		}
		for _, n := range nodes {
			a.Of(n.ID, false)
		}
	}
	walk("")
}

// issueLine renders a structural Issue: `! detached n7 n8: …`.
func (g *Graph) issueLine(i Issue, a *Aliases) string {
	var als []string
	for _, id := range i.IDs {
		if g.nodeByID[id] != nil || g.groupByID[id] != nil {
			als = append(als, g.aliasOf(id, a))
		}
	}
	line := "! " + i.Code
	if len(als) > 0 {
		line += " " + strings.Join(als, " ")
	}
	return line + ": " + i.Message
}

// Snapshot is one line per entity, keyed by identity, used to build change lists.
type Snapshot map[string]string

func (g *Graph) Snapshot(a *Aliases) Snapshot {
	g.Compactless(a)
	s := Snapshot{}
	in := func(parent string) string {
		if parent == "" {
			return ""
		}
		return " in " + g.aliasOf(parent, a)
	}
	for _, gr := range g.Groups {
		s["g:"+gr.ID] = g.groupLine(gr, a) + in(gr.Parent)
	}
	for _, n := range g.Nodes {
		s["n:"+n.ID] = g.nodeLine(n, a) + in(n.Parent)
	}
	for _, e := range g.Edges {
		s["e:"+e.From+">"+e.To] = g.edgeLine(e, a)
	}
	for _, n := range g.Notes {
		s["t:"+n.ID] = g.noteLine(n, a)
	}
	// Issues too, so one that appears or clears mid-session reaches the AI.
	for _, i := range g.Issues {
		s["i:"+i.Code+":"+strings.Join(i.IDs, ",")] = g.issueLine(i, a)
	}
	return s
}

// Diff lists what changed between two snapshots, in the format sent to the
// AI after the first turn. Empty when nothing changed.
func Diff(prev, cur Snapshot, a *Aliases) string {
	var add, del, chg []string
	for k, v := range cur {
		old, ok := prev[k]
		switch {
		case !ok:
			add = append(add, "+ "+v)
		case old != v:
			chg = append(chg, "~ "+v)
		}
	}
	for k, v := range prev {
		if _, ok := cur[k]; !ok {
			if strings.HasPrefix(k, "e:") || strings.HasPrefix(k, "t:") || strings.HasPrefix(k, "i:") {
				del = append(del, "- "+v)
			} else {
				del = append(del, "- "+strings.Fields(v)[0])
			}
		}
	}
	sort.Strings(add)
	sort.Strings(del)
	sort.Strings(chg)
	return strings.Join(slices.Concat(del, chg, add), "\n")
}

// Line renders one Node or Group as the AI sees it ("" if id is neither).
func (g *Graph) Line(id string, a *Aliases) string {
	if n := g.nodeByID[id]; n != nil {
		s := g.nodeLine(n, a)
		if n.Parent != "" {
			s += " in " + g.aliasOf(n.Parent, a)
		}
		return s
	}
	if gr := g.groupByID[id]; gr != nil {
		return g.groupLine(gr, a)
	}
	return ""
}

// EdgesOf renders the Edges touching a Node or Group.
func (g *Graph) EdgesOf(id string, a *Aliases) []string {
	var out []string
	for _, e := range g.Edges {
		if e.From == id || e.To == id {
			out = append(out, g.edgeLine(e, a))
		}
	}
	return out
}
