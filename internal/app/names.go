package app

import (
	"regexp"
	"strings"

	"github.com/vinaayakh/designer/internal/proposal"
	"github.com/vinaayakh/designer/internal/report"
	"github.com/vinaayakh/designer/internal/scene"
)

// The AI refers to elements by Alias (n3, g2) or, for new ones, +ref. Aliases
// belong to an Assistant Session, so text that is saved (Gap Reports, the
// Design Rationale, document text) gets names instead while the mapping is
// known; old text only loses its "(n3)" asides when exported.

var (
	aliasRe       = regexp.MustCompile(`\s?\((\+[A-Za-z0-9_-]+|[ng]\d+)\)|(\+[A-Za-z0-9_-]+|\b[ng]\d+\b)`)
	aliasAsideRe  = regexp.MustCompile(`\s?\((?:\+[A-Za-z0-9_-]+|[ng]\d+)\)`)
	collapseSpace = regexp.MustCompile(`  +`)
)

// nameAliasesLocked replaces aliases in s with element labels. refs maps a
// Proposal's +refs to element IDs. An aside right after the element's own
// name, as in "Audit trail (n15)", is dropped.
func (a *App) nameAliasesLocked(s string, g *scene.Graph, refs map[string]string) string {
	if s == "" || g == nil {
		return s
	}
	label := func(al string) (string, bool) {
		id, ok := refs[al]
		if !ok {
			id, ok = a.sess.Aliases.ID(al)
		}
		if !ok {
			return "", false
		}
		if n := g.Node(id); n != nil {
			return n.Label, true
		}
		if gr := g.Group(id); gr != nil {
			return gr.Label, true
		}
		return "", false
	}
	var out strings.Builder
	last := 0
	for _, m := range aliasRe.FindAllStringSubmatchIndex(s, -1) {
		out.WriteString(s[last:m[0]])
		last = m[1]
		if m[2] >= 0 { // "(n3)"
			name, ok := label(s[m[2]:m[3]])
			switch {
			case !ok:
				out.WriteString(s[m[0]:m[1]])
			case strings.HasSuffix(strings.ToLower(strings.TrimSpace(s[max(0, m[0]-len(name)-2):m[0]])), strings.ToLower(name)):
				// the name is already there
			default:
				out.WriteString(" (" + name + ")")
			}
			continue
		}
		if name, ok := label(s[m[4]:m[5]]); ok {
			out.WriteString(name)
		} else {
			out.WriteString(s[m[0]:m[1]])
		}
	}
	out.WriteString(s[last:])
	return out.String()
}

// nameRationaleLocked names the aliases in every text of a Rationale.
func (a *App) nameRationaleLocked(r *proposal.Rationale, g *scene.Graph, refs map[string]string) {
	if r == nil {
		return
	}
	n := func(s string) string { return a.nameAliasesLocked(s, g, refs) }
	r.Summary = n(r.Summary)
	for i := range r.Assumptions {
		r.Assumptions[i] = n(r.Assumptions[i])
	}
	for _, nr := range r.Nodes {
		if nr != nil {
			nr.Why = n(nr.Why)
		}
	}
	for i := range r.Tradeoffs {
		t := &r.Tradeoffs[i]
		t.Decision, t.Chose, t.Over, t.Because = n(t.Decision), n(t.Chose), n(t.Over), n(t.Because)
	}
}

// refsOf maps a Proposal's +refs to the element IDs its changes created.
func refsOf(changes []*proposal.Change) map[string]string {
	m := map[string]string{}
	for _, ch := range changes {
		if strings.HasPrefix(ch.Ref, "+") && ch.ElementID != "" {
			m[ch.Ref] = ch.ElementID
		}
	}
	return m
}

// dropAsides removes "(n3)" asides from text saved before names were used.
func dropAsides(s string) string {
	return collapseSpace.ReplaceAllString(aliasAsideRe.ReplaceAllString(s, ""), " ")
}

// tidyInput applies dropAsides to every AI-written text an export uses.
func tidyInput(in *report.Input) {
	if r := in.Rationale; r != nil {
		r.Summary = dropAsides(r.Summary)
		for i := range r.Assumptions {
			r.Assumptions[i] = dropAsides(r.Assumptions[i])
		}
		for _, nr := range r.Nodes {
			if nr != nil {
				nr.Why = dropAsides(nr.Why)
			}
		}
		for i := range r.Tradeoffs {
			t := &r.Tradeoffs[i]
			t.Decision, t.Chose, t.Over, t.Because = dropAsides(t.Decision), dropAsides(t.Chose), dropAsides(t.Over), dropAsides(t.Because)
		}
	}
	if gp := in.Gap; gp != nil {
		for i := range gp.Items {
			gp.Items[i].Finding = dropAsides(gp.Items[i].Finding)
		}
		for i := range gp.Recommendations {
			gp.Recommendations[i].Finding = dropAsides(gp.Recommendations[i].Finding)
		}
	}
	if n := in.Narrative; n != nil {
		n.Overview = dropAsides(n.Overview)
		for id, rs := range n.Components {
			for i := range rs {
				rs[i] = dropAsides(rs[i])
			}
			n.Components[id] = rs
		}
		for id, s := range n.Groups {
			n.Groups[id] = dropAsides(s)
		}
		for i := range n.Scenarios {
			n.Scenarios[i].Summary = dropAsides(n.Scenarios[i].Summary)
			for j := range n.Scenarios[i].Steps {
				n.Scenarios[i].Steps[j].What = dropAsides(n.Scenarios[i].Steps[j].What)
			}
		}
		for i := range n.Concepts {
			n.Concepts[i].Text = dropAsides(n.Concepts[i].Text)
		}
		for i := range n.Quality {
			q := &n.Quality[i]
			q.Scenario, q.Response = dropAsides(q.Scenario), dropAsides(q.Response)
		}
	}
}
