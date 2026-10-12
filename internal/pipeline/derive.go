package pipeline

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/vinaayakh/designer/internal/brief"
	"github.com/vinaayakh/designer/internal/scene"
)

// Derived is what a deriver computes: a table for the document and a compact text for the prompt.
type Derived struct {
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
	Note    string     `json:"note,omitempty"`
	Prompt  string     `json:"-"`
}

// Input is what derivers read.
type Input struct {
	Graph   *scene.Graph
	Brief   *brief.Brief
	Written map[string]Written // the sections the Assistant wrote (set by Assemble)
}

type deriver func(in Input) Derived

var derivers = map[string]deriver{
	"external":      deriveExternal,
	"dfd":           deriveDFD,
	"error_budget":  deriveErrorBudget,
	"failure_modes": deriveFailureModes,
}

// Derive runs one deriver (no tokens).
func Derive(name string, in Input) Derived {
	if in.Graph == nil {
		return Derived{Note: "No diagram yet."}
	}
	return derivers[name](in)
}

func label(g *scene.Graph, id string) string {
	if n := g.Node(id); n != nil {
		return n.Label
	}
	if gr := g.Group(id); gr != nil {
		return gr.Label
	}
	return id
}

func edgeText(e *scene.Edge) string {
	parts := slices.DeleteFunc([]string{e.Protocol, e.Label}, func(s string) bool { return s == "" })
	return strings.Join(parts, " · ")
}

// deriveExternal lists external actors and systems and their flows.
func deriveExternal(in Input) Derived {
	g := in.Graph
	d := Derived{Columns: []string{"Partner", "Direction", "Peer", "Interface"}}
	for _, n := range g.Nodes {
		if !strings.HasPrefix(n.Kind, "external.") {
			continue
		}
		for _, e := range g.Edges {
			switch n.ID {
			case e.From:
				d.Rows = append(d.Rows, []string{n.Label, "sends to", label(g, e.To), edgeText(e)})
			case e.To:
				d.Rows = append(d.Rows, []string{n.Label, "receives from", label(g, e.From), edgeText(e)})
			}
		}
	}
	if len(d.Rows) == 0 {
		d.Note = "No external actors or systems on the diagram."
	}
	d.Prompt = table(d)
	return d
}

// DFD element types and the STRIDE categories that apply to each (STRIDE per element).
const (
	dfdExternal = "External entity"
	dfdProcess  = "Process"
	dfdStore    = "Data store"
	dfdFlow     = "Data flow"
)

var strideFor = map[string]string{
	dfdExternal: "S R",
	dfdProcess:  "S T R I D E",
	dfdStore:    "T I D",
	dfdFlow:     "T I D",
}

func dfdType(n *scene.Node) string {
	switch {
	case strings.HasPrefix(n.Kind, "external."):
		return dfdExternal
	case strings.HasPrefix(n.Kind, "data."), strings.HasPrefix(n.Kind, "observability."), n.Kind == "messaging.queue", n.Kind == "messaging.stream":
		return dfdStore
	default:
		return dfdProcess
	}
}

// boundaries lists the Groups containing a Node, innermost first.
func boundaries(g *scene.Graph, parent string) []string {
	var out []string
	for p := parent; p != ""; {
		gr := g.Group(p)
		if gr == nil {
			break
		}
		out = append(out, gr.ID)
		p = gr.Parent
	}
	return out
}

// deriveDFD classifies every element for the data flow diagram, marks STRIDE categories, and finds
// the flows that cross a trust boundary or carry classified data.
func deriveDFD(in Input) Derived {
	g := in.Graph
	d := Derived{Columns: []string{"Element", "Type", "Trust boundary", "Data", "STRIDE applies"}}
	var prompt strings.Builder
	prompt.WriteString("Elements (type · boundary · data · STRIDE):\n")
	classes := func(n *scene.Node) string { return strings.ToUpper(strings.Join(n.Classes, ", ")) }
	for _, n := range g.Nodes {
		t := dfdType(n)
		st := strideFor[t]
		if t == dfdStore && (strings.Contains(n.Kind, "log") || strings.Contains(n.Kind, "audit")) {
			st += " R" // repudiation applies to log and audit stores
		}
		bs := boundaries(g, n.Parent)
		where := "outside every boundary"
		if len(bs) > 0 {
			where = label(g, bs[0])
		}
		d.Rows = append(d.Rows, []string{n.Label, t, where, orDash(classes(n)), st})
		fmt.Fprintf(&prompt, "- %s · %s · %s · %s · %s\n", n.Label, t, where, orDash(classes(n)), st)
	}
	prompt.WriteString("Flows that cross a trust boundary or carry classified data (STRIDE T I D):\n")
	crossings := 0
	for _, e := range g.Edges {
		a, b := g.Node(e.From), g.Node(e.To)
		if a == nil || b == nil {
			continue
		}
		ba, bb := boundaries(g, a.Parent), boundaries(g, b.Parent)
		cross := !slices.Equal(ba, bb)
		data := classes(a)
		if data == "" {
			data = classes(b)
		}
		if !cross && data == "" {
			continue
		}
		crossings++
		why := []string{}
		if cross {
			why = append(why, "crosses a boundary")
		}
		if data != "" {
			why = append(why, "carries "+data)
		}
		d.Rows = append(d.Rows, []string{a.Label + " → " + b.Label, dfdFlow, strings.Join(why, ", "), orDash(data), strideFor[dfdFlow]})
		fmt.Fprintf(&prompt, "- %s → %s (%s) · %s\n", a.Label, b.Label, edgeText(e), strings.Join(why, ", "))
	}
	d.Note = fmt.Sprintf("%d elements, %d flows (%d crossing a boundary or carrying classified data), %d trust boundaries. Computed from the Production diagram.", len(g.Nodes), len(g.Edges), crossings, len(g.Groups))
	d.Prompt = prompt.String()
	return d
}

// deriveErrorBudget turns the Brief's availability target into an error budget.
func deriveErrorBudget(in Input) Derived {
	d := Derived{Columns: []string{"Availability target", "Window", "Error budget"}}
	v := ""
	if in.Brief != nil {
		v = in.Brief.Get("availability")
	}
	label := "%g%%"
	pcts := []float64{}
	if pct, err := strconv.ParseFloat(v, 64); err == nil {
		pcts = append(pcts, pct)
	} else {
		// No target in the Brief: use the availability targets this document assumes in its SLO table.
		for _, r := range in.Written["slos"].Rows {
			if len(r) < 3 {
				continue
			}
			for _, m := range percent.FindAllStringSubmatch(r[2], -1) {
				if p, err := strconv.ParseFloat(m[1], 64); err == nil && p >= 90 && p < 100 && !slices.Contains(pcts, p) {
					pcts = append(pcts, p)
				}
			}
		}
		label = "%g%% (assumed)"
		d.Columns[0] = "SLO target"
		d.Note = "The Design Brief has no availability target; these budgets use the targets assumed in the SLO table."
	}
	if len(pcts) == 0 {
		d.Note = "The Design Brief has no availability target, so there is no error budget yet (it's an assumption)."
		d.Prompt = "The Design Brief has no availability target: state the target you assume in the SLO table (e.g. 99.9%), citing the assumption, and the error budget is computed from it."
		return d
	}
	slices.Sort(pcts)
	for _, pct := range pcts {
		for _, days := range []int{28, 365} {
			mins := (1 - pct/100) * float64(days) * 24 * 60
			d.Rows = append(d.Rows, []string{fmt.Sprintf(label, pct), fmt.Sprintf("%d days", days), duration(mins)})
		}
	}
	if d.Note == "" {
		d.Prompt = table(d)
	} else {
		d.Prompt = "The Design Brief has no availability target: state the target you assume in the SLO table (e.g. 99.9%), citing the assumption, and the error budget is computed from it."
	}
	return d
}

var percent = regexp.MustCompile(`(\d{2}(?:\.\d+)?)\s?%`)

func duration(mins float64) string {
	switch {
	case mins >= 24*60:
		return fmt.Sprintf("%.1f days", mins/(24*60))
	case mins >= 60:
		h := math.Floor(mins / 60)
		return fmt.Sprintf("%g h %d min", h, int(math.Round(mins-h*60)))
	default:
		m := math.Floor(mins)
		return fmt.Sprintf("%g min %d s", m, int(math.Round((mins-m)*60)))
	}
}

// deriveFailureModes suggests failure modes per component from its Kind and attributes.
func deriveFailureModes(in Input) Derived {
	g := in.Graph
	d := Derived{Columns: []string{"Component", "Suggested failure mode"}}
	attr := func(n *scene.Node, keys ...string) string {
		for _, k := range keys {
			if v := n.Attrs[k]; v != "" {
				return v
			}
		}
		return ""
	}
	for _, n := range g.Nodes {
		if strings.HasPrefix(n.Kind, "external.") {
			d.Rows = append(d.Rows, []string{n.Label, "unavailable or slow third party"})
			continue
		}
		var modes []string
		if strings.HasPrefix(n.Kind, "data.") {
			modes = append(modes, "data loss or corruption")
			if attr(n, "replicas", "multi_az", "zones", "ha") == "" {
				modes = append(modes, "single instance or single zone: an outage stops it")
			}
			if !hasBackup(g) {
				modes = append(modes, "no backup component on the diagram")
			}
		}
		if strings.HasPrefix(n.Kind, "compute.") {
			if attr(n, "replicas", "instances", "autoscale", "min") == "" {
				modes = append(modes, "single instance: a crash stops it")
			}
			modes = append(modes, "overload at peak")
		}
		if strings.HasPrefix(n.Kind, "messaging.") {
			modes = append(modes, "backlog grows faster than consumers")
		}
		if strings.HasPrefix(n.Kind, "network.") {
			modes = append(modes, "misconfiguration blocks traffic")
		}
		for _, m := range modes {
			d.Rows = append(d.Rows, []string{n.Label, m})
		}
	}
	d.Prompt = table(d)
	return d
}

func hasBackup(g *scene.Graph) bool {
	for _, n := range g.Nodes {
		if n.Kind == "data.backup" {
			return true
		}
	}
	return false
}

func orDash(s string) string {
	if s == "" {
		return "–"
	}
	return s
}

func table(d Derived) string {
	var b strings.Builder
	b.WriteString(strings.Join(d.Columns, " | ") + "\n")
	for _, r := range d.Rows {
		b.WriteString(strings.Join(r, " | ") + "\n")
	}
	if d.Note != "" {
		b.WriteString(d.Note + "\n")
	}
	return b.String()
}
