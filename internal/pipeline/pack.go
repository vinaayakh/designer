package pipeline

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/vinaayakh/designer/internal/register"
)

// The document pack (wayfinder/doc-pipeline ticket 009) is the audit hand-off: every current
// document as written, the ADRs, the Register as Markdown plus its raw log, and an index.

// File is one file in the pack.
type File struct {
	Name string
	Data []byte
}

// PackDoc is a Step's latest version and its status now.
type PackDoc struct {
	Step    *Step
	Version *Version // nil when never written
	Status  Status
}

// Markdown reads the Markdown copy saved when a version was written.
func (st Store) Markdown(step string, n int) ([]byte, error) {
	return os.ReadFile(st.path(step, fmt.Sprintf("v%03d.md", n)))
}

// DocName is a written version's file name inside the pack.
func DocName(d PackDoc) string {
	return fmt.Sprintf("documents/%s-v%d.md", d.Step.ID, d.Version.N)
}

// Index lists what the pack holds and whether each document is still current.
func Index(project string, docs []PackDoc, entries []register.Entry, integrity string, at time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: document pack\n\nGenerated %s by Designer.\n\n", project, at.UTC().Format("2 Jan 2006 15:04 UTC"))
	b.WriteString("## Documents\n\n| Document | Version | Built from | Status |\n|---|---|---|---|\n")
	for _, d := range docs {
		if d.Version == nil {
			fmt.Fprintf(&b, "| %s | not written | | %s |\n", d.Step.Title, cell(d.Status.Text))
			continue
		}
		status := d.Status.Text
		if d.Status.Kind == "stale" {
			status = "**stale**: " + strings.Join(d.Status.Reasons, "; ")
		}
		fmt.Fprintf(&b, "| [%s](%s) | v%d, %s | %s | %s |\n", d.Step.Title, DocName(d), d.Version.N, d.Version.Created.UTC().Format("2 Jan 2006"), cell(builtFrom(d.Version)), cell(status))
	}
	counts := map[register.Type]map[register.Status]int{}
	for _, e := range entries {
		if counts[e.Type] == nil {
			counts[e.Type] = map[register.Status]int{}
		}
		counts[e.Type][e.Status]++
	}
	b.WriteString("\n## Register\n\nThe full trail is in [register.md](register.md); the raw, hash-chained event log is [register/log.jsonl](register/log.jsonl). ")
	if integrity == "" {
		b.WriteString("The hash chain is intact.\n\n")
	} else {
		fmt.Fprintf(&b, "**The hash chain is broken:** %s\n\n", integrity)
	}
	b.WriteString("| Type | Confirmed | Open | Rejected | Superseded |\n|---|---|---|---|---|\n")
	for _, t := range typeOrder {
		c := counts[t]
		if c == nil {
			continue
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d |\n", typeNames[t], c[register.Confirmed], c[register.Open], c[register.Rejected], c[register.Superseded])
	}
	b.WriteString("\nOpen entries are statements the Assistant made that nobody has confirmed yet.\n\n")
	b.WriteString("## Decisions\n\nEach decision (DEC) is an Architecture Decision Record in [adr/](adr/), in MADR 4.0 format: DEC-012 is ADR-0012.\n")
	return b.String()
}

func builtFrom(v *Version) string {
	parts := []string{"Design Brief"}
	if v.Inputs.Production > 0 {
		parts = append(parts, fmt.Sprintf("Production v%d", v.Inputs.Production))
	} else {
		parts = append(parts, "Rough Draft")
	}
	for _, up := range sortedKeys(v.Inputs.Upstream) {
		if n := v.Inputs.Upstream[up]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s v%d", Get(up).Title, n))
		}
	}
	return strings.Join(parts, ", ")
}

var typeOrder = []register.Type{register.REQ, register.CON, register.ASM, register.NG, register.DEC, register.RISK, register.Q}

var typeNames = map[register.Type]string{
	register.REQ: "Requirements", register.CON: "Constraints", register.ASM: "Assumptions", register.NG: "Non-goals and out of scope",
	register.DEC: "Decisions", register.RISK: "Risks", register.Q: "Open questions",
}

func cell(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "|", "\\|"), "\n", " ") }

func source(e register.Entry) string {
	s := e.Source.Kind
	if e.Source.Ref != "" {
		s += " · " + e.Source.Ref
	}
	return s
}

// RegisterMarkdown renders every entry, grouped by type, with its full history.
func RegisterMarkdown(project string, entries []register.Entry, integrity string, label func(id string) string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: Register\n\nEvery requirement, constraint, assumption, non-goal, decision, risk and open question, with where it came from and every change of status. Entries are never edited: a changed statement supersedes the old entry.\n\n", project)
	if integrity != "" {
		fmt.Fprintf(&b, "> **The hash chain is broken:** %s\n\n", integrity)
	}
	for _, t := range typeOrder {
		var es []register.Entry
		for _, e := range entries {
			if e.Type == t {
				es = append(es, e)
			}
		}
		if len(es) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", typeNames[t])
		for _, e := range es {
			fmt.Fprintf(&b, "### %s (%s)\n\n%s\n\n", e.ID, e.Status, e.Statement)
			for _, k := range sortedKeys(e.Fields) {
				fmt.Fprintf(&b, "- **%s:** %s\n", k, e.Fields[k])
			}
			fmt.Fprintf(&b, "- **Source:** %s\n", source(e))
			if e.Source.Text != "" {
				fmt.Fprintf(&b, "- **From the chat message:** \"%s\"\n", strings.ReplaceAll(e.Source.Text, "\n", " "))
			}
			if e.Step != "" {
				fmt.Fprintf(&b, "- **Introduced by:** %s\n", stepTitle(e.Step))
			}
			if len(e.Links) > 0 {
				fmt.Fprintf(&b, "- **Linked to:** %s\n", labels(e.Links, label))
			}
			if e.Supersedes != "" {
				fmt.Fprintf(&b, "- **Supersedes:** %s\n", e.Supersedes)
			}
			if e.SupersededBy != "" {
				fmt.Fprintf(&b, "- **Superseded by:** %s\n", e.SupersededBy)
			}
			b.WriteString("\n| When | Who | Status | Why |\n|---|---|---|---|\n")
			for _, h := range e.History {
				fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", h.At.UTC().Format("2006-01-02 15:04"), h.Actor, h.Status, cell(h.Reason))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func stepTitle(id string) string {
	if s := Get(id); s != nil {
		return s.Title
	}
	return id
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// ADRName is the ADR file for a decision: DEC-012 → adr/ADR-0012-<slug>.md.
func ADRName(e register.Entry) string {
	n := strings.TrimPrefix(e.ID, "DEC-")
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(e.Statement), "-"), "-")
	if len(slug) > 50 {
		slug = strings.TrimRight(slug[:50], "-")
	}
	return fmt.Sprintf("adr/ADR-%04s-%s.md", n, slug)
}

func adrID(id string) string { return "ADR-" + fmt.Sprintf("%04s", strings.TrimPrefix(id, "DEC-")) }

// ADR renders a decision as an Architecture Decision Record in MADR 4.0's layout.
func ADR(e register.Entry, byID map[string]register.Entry, label func(id string) string) string {
	status := map[register.Status]string{register.Open: "proposed", register.Confirmed: "accepted", register.Rejected: "rejected", register.Superseded: "superseded"}[e.Status]
	if e.SupersededBy != "" {
		if by, ok := byID[e.SupersededBy]; ok && by.Type == register.DEC {
			status = "superseded by " + adrID(by.ID)
		} else {
			status = "superseded by " + e.SupersededBy
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "---\nstatus: %s\ndate: %s\n---\n\n", status, e.Created.UTC().Format("2006-01-02"))
	fmt.Fprintf(&b, "# %s: %s\n\n", adrID(e.ID), e.Statement)
	b.WriteString("## Context and Problem Statement\n\n")
	if c := e.Fields["because"]; c != "" {
		b.WriteString(c + "\n\n")
	} else {
		b.WriteString(e.Statement + "\n\n")
	}
	if e.Fields["chose"] != "" || e.Fields["over"] != "" {
		b.WriteString("## Considered Options\n\n")
		for _, o := range []string{e.Fields["chose"], e.Fields["over"]} {
			if o != "" {
				fmt.Fprintf(&b, "* %s\n", o)
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("## Decision Outcome\n\n")
	if c := e.Fields["chose"]; c != "" {
		fmt.Fprintf(&b, "Chosen option: \"%s\"", c)
		if r := e.Fields["because"]; r != "" {
			fmt.Fprintf(&b, ", because %s", strings.TrimSuffix(r, "."))
		}
		b.WriteString(".\n\n")
	} else {
		b.WriteString(e.Statement + "\n\n")
	}
	b.WriteString("## More Information\n\n")
	fmt.Fprintf(&b, "Register entry %s, source: %s.", e.ID, source(e))
	if e.Supersedes != "" {
		fmt.Fprintf(&b, " Supersedes %s.", e.Supersedes)
	}
	if len(e.Links) > 0 {
		fmt.Fprintf(&b, " Applies to: %s.", labels(e.Links, label))
	}
	b.WriteString("\n")
	return b.String()
}

func labels(ids []string, label func(string) string) string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id
		if label != nil {
			if l := label(id); l != "" {
				out[i] = l
			}
		}
	}
	return strings.Join(out, ", ")
}
