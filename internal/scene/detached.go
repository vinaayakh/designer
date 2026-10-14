package scene

import (
	"fmt"
	"strings"
)

// Detached is part of a diagram that isn't connected to the main system:
// one Node with no Edges at all, or a few Nodes linked only to each other
// (e.g. CloudTrail → GuardDuty, with nothing else attached).
type Detached struct {
	IDs      []string `json:"ids"`      // Node IDs, in diagram order
	Isolated bool     `json:"isolated"` // a single Node with no Edges
}

// Detached returns the parts of the diagram not connected to the main
// system, following Edges in either direction. The main system is the
// largest connected part that a user or client enters (an external.user or
// external.client Node), or the largest part when nothing does. A diagram
// that is one connected whole, or has no connected Nodes at all, has
// nothing detached.
//
// Being detached isn't always a mistake: account-wide services (audit
// logging, threat detection, key management) are often drawn on their own.
// The caller decides what to do; the Assistant is told to ask when unsure.
func (g *Graph) Detached() []Detached {
	if len(g.Nodes) < 2 {
		return nil
	}
	adj := map[string][]string{}
	link := func(a, b string) {
		adj[a] = append(adj[a], b)
		adj[b] = append(adj[b], a)
	}
	for _, e := range g.Edges {
		// An Edge from a Group (CloudTrail observes the AWS account) ties
		// its Node to everything inside that Group.
		from, to := g.members(e.From), g.members(e.To)
		for _, a := range from {
			for _, b := range to {
				if a != b {
					link(a, b)
				}
			}
		}
	}
	// Connected parts, each listed in diagram order.
	part := map[string]int{}
	var parts [][]string
	for _, n := range g.Nodes {
		if _, ok := part[n.ID]; ok {
			continue
		}
		i := len(parts)
		part[n.ID] = i
		stack := []string{n.ID}
		for len(stack) > 0 {
			v := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, w := range adj[v] {
				if _, ok := part[w]; !ok {
					part[w] = i
					stack = append(stack, w)
				}
			}
		}
		parts = append(parts, nil)
	}
	for _, n := range g.Nodes {
		parts[part[n.ID]] = append(parts[part[n.ID]], n.ID)
	}
	if len(parts) == 1 {
		return nil
	}

	entered := func(ids []string) bool {
		for _, id := range ids {
			if k := g.nodeByID[id].Kind; k == "external.user" || k == "external.client" {
				return true
			}
		}
		return false
	}
	main := -1
	for i, p := range parts {
		better := main < 0 ||
			entered(p) && !entered(parts[main]) ||
			entered(p) == entered(parts[main]) && len(p) > len(parts[main])
		if better {
			main = i
		}
	}
	// Nothing is detached from a "system" of one Node: a sketch with boxes
	// but no arrows yet is unfinished, not disconnected.
	if len(parts[main]) < 2 {
		return nil
	}
	var out []Detached
	for i, p := range parts {
		if i != main {
			out = append(out, Detached{IDs: p, Isolated: len(p) == 1 && len(adj[p[0]]) == 0})
		}
	}
	return out
}

// members is the Node itself, or every Node inside a Group (nested Groups included).
func (g *Graph) members(id string) []string {
	if g.nodeByID[id] != nil {
		return []string{id}
	}
	if g.groupByID[id] == nil {
		return nil
	}
	inside := func(parent string) bool {
		for p := parent; p != ""; {
			if p == id {
				return true
			}
			gr := g.groupByID[p]
			if gr == nil {
				return false
			}
			p = gr.Parent
		}
		return false
	}
	var out []string
	for _, n := range g.Nodes {
		if inside(n.Parent) {
			out = append(out, n.ID)
		}
	}
	return out
}

// detachedIssues reports each detached part as a structural Issue, so it
// reaches the Assistant with the diagram (`! detached n7 n8: …`).
func (g *Graph) detachedIssues() {
	for _, d := range g.Detached() {
		labels := make([]string, len(d.IDs))
		for i, id := range d.IDs {
			labels[i] = fmt.Sprintf("%q", g.nodeByID[id].Label)
		}
		var msg string
		switch {
		case d.Isolated:
			msg = labels[0] + " isn't connected to anything"
		case len(labels) == 1:
			msg = labels[0] + " is connected only to itself"
		default:
			msg = strings.Join(labels, ", ") + " are connected only to each other, not to the rest of the system"
		}
		g.Issues = append(g.Issues, Issue{"detached", d.IDs, msg})
	}
}
