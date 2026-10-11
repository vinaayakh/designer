package app

import (
	"fmt"
	"strings"

	"github.com/vinaayakh/designer/internal/scene"
)

// contextLocked builds the context block for a message (ticket 012): the
// Brief and the whole diagram the first time, then only what changed, plus
// the Nodes the user clicked into context and any queued Proposal outcomes.
func (a *App) contextLocked(view string, selection []string) (string, error) {
	if view != ViewProduction {
		view = ViewRough
	}
	b, err := a.Brief()
	if err != nil {
		return "", err
	}
	g, ver, err := a.Graph(view)
	if err != nil {
		return "", err
	}
	al := a.sess.Aliases
	var parts []string

	if len(a.sess.Feedback) > 0 {
		parts = append(parts, "Proposal updates:\n"+strings.Join(a.sess.Feedback, "\n"))
		a.sess.Feedback = nil
	}

	bc := b.Compact()
	switch {
	case a.sess.SentBrief == "":
		m, _ := a.P.Meta()
		head := fmt.Sprintf("Project %q.", m.Name)
		if m.FrozenAt != nil {
			head += " The Rough Draft is frozen (Conversion done); changes go to the Production Diagram."
		}
		parts = append(parts, head+"\n"+bc)
	case a.sess.SentBrief != bc:
		parts = append(parts, "The Design Brief changed:\n"+bc)
	}
	a.sess.SentBrief = bc

	label := "Rough Draft"
	if view == ViewProduction {
		label = fmt.Sprintf("Production Diagram v%d", ver)
	}
	snap := g.Snapshot(al)
	prev, seen := a.sess.Sent[view]
	switch {
	case !seen:
		parts = append(parts, fmt.Sprintf("Active view: %s\n%s", label, strings.TrimRight(g.Compact(al), "\n")))
	default:
		d := scene.Diff(prev, snap, al)
		if d == "" {
			d = "(no changes since you last saw it)"
		}
		parts = append(parts, fmt.Sprintf("Active view: %s. Changes:\n%s", label, d))
	}
	a.sess.Sent[view] = snap
	if view == ViewProduction {
		a.sess.SentVer = ver
	}

	if len(selection) > 0 {
		var lines []string
		for _, id := range selection {
			if l := g.Line(id, al); l != "" {
				lines = append(lines, l)
				lines = append(lines, indent(g.EdgesOf(id, al))...)
			}
		}
		if len(lines) > 0 {
			parts = append(parts, "The user selected:\n"+strings.Join(lines, "\n"))
		}
	}
	a.saveSession()
	return strings.Join(parts, "\n\n"), nil
}

func indent(lines []string) []string {
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return lines
}

// fullDiagramLocked renders a view in full and records it as seen.
func (a *App) fullDiagramLocked(view string) (string, int, error) {
	g, ver, err := a.Graph(view)
	if err != nil {
		return "", 0, err
	}
	a.sess.Sent[view] = g.Snapshot(a.sess.Aliases)
	return strings.TrimRight(g.Compact(a.sess.Aliases), "\n"), ver, nil
}
