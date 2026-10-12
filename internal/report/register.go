package report

import (
	"fmt"
	"strings"

	"github.com/vinaayakh/designer/internal/register"
)

// When the Project has a Register (wayfinder/doc-pipeline ticket 009), arc42 reads requirements,
// constraints, assumptions, non-goals, decisions, risks and open questions from it, so each fact
// lives in one place. Without one (older inputs), sections fall back to the Design Rationale.

func (b *builder) hasRegister() bool { return len(b.in.Entries) > 0 }

// live returns the live entries of one type, optionally filtered.
func (b *builder) live(t register.Type, keep func(register.Entry) bool) []register.Entry {
	var out []register.Entry
	for _, e := range b.in.Entries {
		if e.Type == t && e.Live() && (keep == nil || keep(e)) {
			out = append(out, e)
		}
	}
	return out
}

// statusRun marks entries nobody has confirmed yet.
func statusRun(e register.Entry) Run {
	if e.Status == register.Open {
		return Muted(" (unconfirmed)")
	}
	return T("")
}

// ADRID numbers a decision as its ADR: DEC-012 → ADR-0012 (as in the document pack).
func ADRID(id string) string {
	return "ADR-" + fmt.Sprintf("%04s", strings.TrimPrefix(id, "DEC-"))
}

func notBrief(e register.Entry) bool { return e.Source.Kind != "brief" }

func (b *builder) requirementsTable() *Table {
	reqs := b.live(register.REQ, func(e register.Entry) bool { return notBrief(e) })
	if len(reqs) == 0 {
		return nil
	}
	t := &Table{Head: []string{"ID", "Requirement", "Kind", "Priority"}, Widths: []int{12, 58, 15, 15}}
	for _, e := range reqs {
		t.Rows = append(t.Rows, [][]Run{ctext(e.ID), cell(T(e.Statement), statusRun(e)), ctext(e.Fields["kind"]), ctext(e.Fields["priority"])})
	}
	return t
}

func (b *builder) entryList(es []register.Entry) List {
	var items [][]Run
	for _, e := range es {
		items = append(items, []Run{B(e.ID + " "), T(e.Statement), statusRun(e)})
	}
	return List{Items: items}
}

func (b *builder) decisionEntries() []register.Entry { return b.live(register.DEC, nil) }

func decisionStatus(e register.Entry) string {
	if e.Status == register.Open {
		return "Proposed (not yet confirmed)"
	}
	return "Accepted"
}

func orStatement(v string, e register.Entry) string {
	if v != "" {
		return v
	}
	return e.Statement
}
