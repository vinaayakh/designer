package pipeline

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/vinaayakh/designer/internal/register"
)

// Written is what the Assistant submits for one section it writes.
type Written struct {
	Body  string     `json:"body,omitempty"`
	Items []string   `json:"items,omitempty"`
	Rows  [][]string `json:"rows,omitempty"`
	Cites []string   `json:"cites,omitempty"`
}

var idRef = regexp.MustCompile(`\b(REQ|ASM|CON|NG|DEC|RISK|Q)-\d{3,}\b`)

// Assemble builds a document's sections: the ones the Assistant wrote (checked against the Step's
// schema) and the ones the runtime fills in (Register entries, derived tables). It returns every
// problem found, so the Assistant can fix them all in one resubmission.
func Assemble(s *Step, written map[string]Written, in Input, entries []register.Entry) ([]SectionOut, []string) {
	var out []SectionOut
	var problems []string
	known := map[string]register.Entry{}
	for _, e := range entries {
		known[e.ID] = e
	}
	checkCites := func(where string, ids []string, text string) []string {
		all := append(slices.Clone(ids), idRef.FindAllString(text, -1)...)
		var ok []string
		for _, id := range all {
			e, found := known[id]
			switch {
			case !found:
				problems = append(problems, fmt.Sprintf("%s cites %s, which isn't in the Register", where, id))
			case !e.Live():
				problems = append(problems, fmt.Sprintf("%s cites %s, which was %s", where, id, e.Status))
			case !slices.Contains(ok, id):
				ok = append(ok, id)
			}
		}
		return ok
	}
	for id := range written {
		if !slices.ContainsFunc(s.Sections, func(d SectionDef) bool { return d.ID == id && d.Written() }) {
			problems = append(problems, fmt.Sprintf("there is no section %q to write (written sections: %s)", id, strings.Join(writtenIDs(s), ", ")))
		}
	}
	for _, d := range s.Sections {
		sec := SectionOut{ID: d.ID, Title: d.Title, Kind: d.Kind, Columns: d.Columns}
		w, has := written[d.ID]
		where := "section " + d.ID
		switch d.Kind {
		case "prose":
			body := strings.TrimSpace(w.Body)
			if !has || body == "" {
				problems = append(problems, where+": write it (body)")
			} else if len(body) > 4000 {
				problems = append(problems, where+": too long (at most 4000 characters)")
			}
			sec.Body = body
			sec.Cites = checkCites(where, w.Cites, body)
		case "list":
			if !has || len(w.Items) == 0 {
				problems = append(problems, where+": write at least one item (items)")
			}
			if len(w.Items) > 40 {
				problems = append(problems, where+": at most 40 items")
			}
			for _, it := range w.Items {
				if t := strings.TrimSpace(it); t != "" {
					sec.Items = append(sec.Items, t)
				}
			}
			sec.Cites = checkCites(where, w.Cites, strings.Join(sec.Items, "\n"))
		case "table":
			if !has || len(w.Rows) == 0 {
				problems = append(problems, fmt.Sprintf("%s: write at least one row (rows of %d cells: %s)", where, len(d.Columns), strings.Join(d.Columns, ", ")))
			}
			if len(w.Rows) > 80 {
				problems = append(problems, where+": at most 80 rows")
			}
			var text []string
			for i, r := range w.Rows {
				if len(r) != len(d.Columns) {
					problems = append(problems, fmt.Sprintf("%s row %d has %d cells, needs %d (%s)", where, i+1, len(r), len(d.Columns), strings.Join(d.Columns, ", ")))
					continue
				}
				sec.Rows = append(sec.Rows, r)
				text = append(text, strings.Join(r, " "))
			}
			sec.Cites = checkCites(where, w.Cites, strings.Join(text, "\n"))
		case "entries":
			for _, e := range EntriesFor(d, entries) {
				sec.Entries = append(sec.Entries, e.ID)
			}
			sec.Cites = sec.Entries
		case "derived":
			dv := Derive(d.Deriver, in)
			sec.Columns, sec.Rows, sec.Body = dv.Columns, dv.Rows, dv.Note
		}
		out = append(out, sec)
	}
	return out, problems
}

func writtenIDs(s *Step) []string {
	var out []string
	for _, d := range s.Sections {
		if d.Written() {
			out = append(out, d.ID)
		}
	}
	return out
}

// Cited collects every entry a document relies on, with its status now.
func Cited(sections []SectionOut, entries []register.Entry) map[string]register.Status {
	status := map[string]register.Status{}
	for _, e := range entries {
		status[e.ID] = e.Status
	}
	out := map[string]register.Status{}
	for _, s := range sections {
		for _, id := range s.Cites {
			out[id] = status[id]
		}
	}
	return out
}

// Markdown renders a version as the readable copy saved beside it.
func Markdown(s *Step, v *Version, project string, entries []register.Entry) string {
	byID := map[string]register.Entry{}
	for _, e := range entries {
		byID[e.ID] = e
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: %s\n\n", project, s.Title)
	fmt.Fprintf(&b, "| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| Version | v%d, %s |\n", v.N, v.Created.Format("2 Jan 2006 15:04 UTC"))
	built := []string{"Design Brief"}
	if v.Inputs.Production > 0 {
		built = append(built, fmt.Sprintf("Production diagram v%d", v.Inputs.Production))
	} else {
		built = append(built, "Rough Draft")
	}
	for _, up := range sortedKeys(v.Inputs.Upstream) {
		if n := v.Inputs.Upstream[up]; n > 0 {
			built = append(built, fmt.Sprintf("%s v%d", Get(up).Title, n))
		}
	}
	fmt.Fprintf(&b, "| Built from | %s |\n", strings.Join(built, ", "))
	fmt.Fprintf(&b, "| Structure | %s |\n", s.Template)
	open := 0
	for id := range v.Cites {
		if byID[id].Status == register.Open {
			open++
		}
	}
	fmt.Fprintf(&b, "| Register | cites %d entries, %d still open (unconfirmed) |\n", len(v.Cites), open)
	if v.Model != "" {
		fmt.Fprintf(&b, "| Written by | the Assistant (%s), from the sources above |\n", v.Model)
	}
	b.WriteString("\n")
	if s.Credit != "" {
		fmt.Fprintf(&b, "_%s_\n\n", s.Credit)
	}
	cell := func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "|", "\\|"), "\n", " ") }
	for _, sec := range v.Sections {
		fmt.Fprintf(&b, "## %s\n\n", sec.Title)
		if sec.Body != "" {
			b.WriteString(sec.Body + "\n\n")
		}
		for _, it := range sec.Items {
			fmt.Fprintf(&b, "- %s\n", it)
		}
		if len(sec.Items) > 0 {
			b.WriteString("\n")
		}
		if len(sec.Rows) > 0 {
			b.WriteString("| " + strings.Join(sec.Columns, " | ") + " |\n|" + strings.Repeat("---|", len(sec.Columns)) + "\n")
			for _, r := range sec.Rows {
				cs := make([]string, len(r))
				for i, c := range r {
					cs[i] = cell(c)
				}
				b.WriteString("| " + strings.Join(cs, " | ") + " |\n")
			}
			b.WriteString("\n")
		}
		if sec.Kind == "entries" {
			if len(sec.Entries) == 0 {
				b.WriteString("None.\n\n")
				continue
			}
			b.WriteString("| ID | Statement | Status | Source |\n|---|---|---|---|\n")
			for _, id := range sec.Entries {
				e := byID[id]
				src := e.Source.Kind
				if e.Source.Ref != "" {
					src += " · " + e.Source.Ref
				}
				fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", id, cell(e.Statement), e.Status, cell(src))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// Summary is the short form of a version that downstream Steps get instead of the full text.
func Summary(s *Step, sections []SectionOut) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:\n", s.Title)
	for _, sec := range sections {
		line := sec.Body
		if line == "" && len(sec.Items) > 0 {
			line = strings.Join(sec.Items, "; ")
		}
		if line == "" && len(sec.Entries) > 0 {
			line = "entries " + strings.Join(sec.Entries, ", ")
		}
		if line == "" && len(sec.Rows) > 0 {
			line = fmt.Sprintf("%d rows", len(sec.Rows))
		}
		if len(line) > 300 {
			line = line[:300] + "…"
		}
		fmt.Fprintf(&b, "- %s: %s\n", sec.Title, strings.ReplaceAll(line, "\n", " "))
	}
	return b.String()
}
