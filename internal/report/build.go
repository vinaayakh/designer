package report

import (
	"fmt"
	"github.com/vinaayakh/designer/internal/register"
	"slices"
	"strings"

	"github.com/vinaayakh/designer/internal/scene"
)

// Build assembles the arc42 document.
func Build(in *Input) *Doc {
	b := &builder{in: in, g: in.Graph}
	b.index()
	d := &Doc{
		Title:    in.Project,
		Subtitle: fmt.Sprintf("Architecture documentation (arc42) · Production Diagram v%d", in.Version),
		Meta: [][2]string{
			{"Diagram version", fmt.Sprintf("v%d, saved %s", in.Version, in.Created.Format("2 Jan 2006 15:04 UTC"))},
			{"Generated", in.Generated.Format("2 Jan 2006 15:04 UTC")},
			{"Target", in.Cloud},
			{"Compliance", b.regimeList()},
			{"Gap Check", b.gapLine()},
		},
		Notice: []Run{T("Compliance checklists in this document show how the "), I("design"), T(" addresses each control. They are not an audit attestation: controls marked Outside diagram or Process need evidence from configuration, policies and procedures.")},
	}
	if in.Narrative == nil {
		d.Notice = append(d.Notice, T(" Component responsibilities and runtime scenarios were not written for this version, so those sections use stored data only."))
	}
	d.Sections = []*Section{
		b.goals(), b.constraints(), b.context(), b.strategy(), b.buildingBlocks(), b.runtime(),
		b.deployment(), b.concepts(), b.decisions(), b.quality(), b.risks(), b.glossary(),
		b.checklist(), b.sources(),
	}
	for _, s := range d.Sections {
		s.PageBreak = true
	}
	return d
}

type builder struct {
	in        *Input
	g         *scene.Graph
	order     []*scene.Node // Nodes in diagram order: by Group, depth first
	groups    []*scene.Group
	adr       []string // ADR ids by trade-off index
	mentioned map[string]bool
}

func (b *builder) index() {
	b.mentioned = map[string]bool{}
	var walk func(parent string)
	walk = func(parent string) {
		for _, n := range b.g.Nodes {
			if n.Parent == parent {
				b.order = append(b.order, n)
			}
		}
		for _, gr := range b.g.Groups {
			if gr.Parent == parent {
				b.groups = append(b.groups, gr)
				walk(gr.ID)
			}
		}
	}
	walk("")
	if b.in.Rationale != nil {
		for i := range b.in.Rationale.Tradeoffs {
			b.adr = append(b.adr, fmt.Sprintf("ADR-%03d", i+1))
		}
	}
}

// ---- naming

func (b *builder) name(id string) string {
	if n := b.g.Node(id); n != nil {
		return n.Label
	}
	if gr := b.g.Group(id); gr != nil {
		return gr.Label
	}
	return id
}

func (b *builder) names(ids []string) string {
	var out []string
	for _, id := range ids {
		out = append(out, b.name(id))
	}
	return strings.Join(out, ", ")
}

func (b *builder) service(n *scene.Node) (string, string) {
	if s, ok := b.in.Services[n.Service]; ok {
		return s.Name, s.Docs
	}
	return n.ServiceName, ""
}

func (b *builder) kindText(n *scene.Node) string {
	if g := b.in.Kinds[n.Kind]; g != "" {
		return g
	}
	return n.Kind
}

// path is where a Node or Group sits, e.g. "Primary region › App VPC › App".
func (b *builder) path(parent string) string {
	var parts []string
	for p := parent; p != ""; {
		gr := b.g.Group(p)
		if gr == nil {
			break
		}
		parts = append([]string{gr.Label}, parts...)
		p = gr.Parent
	}
	if len(parts) == 0 {
		return "outside every boundary"
	}
	return strings.Join(parts, " › ")
}

func (b *builder) regimeName(id string) string {
	if n := b.in.RegimeNames[id]; n != "" {
		return n
	}
	return strings.ToUpper(id)
}

func (b *builder) regimeList() string {
	if len(b.in.Regimes) == 0 {
		return "None selected"
	}
	var out []string
	for _, r := range b.in.Regimes {
		out = append(out, b.regimeName(r))
	}
	return strings.Join(out, ", ")
}

func (b *builder) gapLine() string {
	gp := b.in.Gap
	if gp == nil {
		return "Not run yet"
	}
	met, unmet, other := 0, 0, 0
	for _, it := range gp.Items {
		switch it.Status {
		case "met":
			met++
		case "unmet":
			unmet++
		default:
			other++
		}
	}
	s := fmt.Sprintf("%s: %d met, %d unmet, %d outside the diagram", gp.Created.Format("2 Jan 2006"), met, unmet, other)
	if gp.Version != b.in.Version {
		s += fmt.Sprintf(" (checked v%d, an earlier version)", gp.Version)
	}
	return s
}

func (b *builder) record(id string) (title string, runs []Run) {
	b.mentioned[id] = true
	r := b.in.Records[id]
	if r == nil {
		return id, []Run{M(id)}
	}
	if r.Ref != "" {
		return r.Title, []Run{B(b.regimeName(r.Regime) + " " + r.Ref), T(" " + r.Title)}
	}
	return r.Title, []Run{T(r.Title)}
}

func (b *builder) responsibilities(n *scene.Node) []string {
	if b.in.Narrative != nil {
		if r := b.in.Narrative.Components[n.ID]; len(r) > 0 {
			return r
		}
	}
	return []string{capital(b.kindText(n)) + "."}
}

func (b *builder) why(id string) string {
	if b.in.Rationale != nil {
		if r := b.in.Rationale.Nodes[id]; r != nil {
			return r.Why
		}
	}
	return ""
}

func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func edgeText(e *scene.Edge) string {
	return strings.Join(slices.DeleteFunc([]string{e.Protocol, e.Label}, func(s string) bool { return s == "" }), " · ")
}

func briefRow(in *Input, id string) *BriefRow {
	for i := range in.Brief {
		if in.Brief[i].ID == id {
			return &in.Brief[i]
		}
	}
	return nil
}

// ---- 1 Introduction and Goals

func (b *builder) goals() *Section {
	s := &Section{Num: "1", Title: "Introduction and Goals"}
	o := s.sub("1.1", "Requirements overview")
	switch {
	case b.in.Narrative != nil && b.in.Narrative.Overview != "":
		o.add(Ptext(b.in.Narrative.Overview))
	case b.in.Rationale != nil && b.in.Rationale.Summary != "":
		o.add(Ptext(b.in.Rationale.Summary))
	}
	o.add(P(T("The requirements come from the Design Brief:")))
	t := Table{Head: []string{"Question", "Answer"}, Widths: []int{35, 65}}
	for _, r := range b.in.Brief {
		v := []Run{T(r.Value)}
		if r.Unknown {
			v = []Run{Muted("Unknown (assumed; see 2.2)")}
		}
		t.Rows = append(t.Rows, [][]Run{ctext(r.Label), v})
	}
	o.add(t)

	q := s.sub("1.2", "Quality goals")
	qt := Table{Head: []string{"Quality goal", "Target"}, Widths: []int{35, 65}}
	for _, id := range []string{"availability", "latency", "throughput", "users", "rpo", "rto", "data_volume"} {
		if r := briefRow(b.in, id); r != nil && !r.Unknown {
			qt.Rows = append(qt.Rows, [][]Run{ctext(r.Label), ctext(r.Value)})
		}
	}
	if len(qt.Rows) > 0 {
		q.add(qt, P(T("Section 10 shows how the architecture meets them.")))
	} else {
		q.add(Ptext("The Design Brief sets no measurable quality targets."))
	}
	if t := b.requirementsTable(); t != nil {
		s.sub("1.3", "Requirements").add(Ptext("From the Register (the Requirements spec cites the same entries)."), *t)
	}
	return s
}

// ---- 2 Constraints

func (b *builder) constraints() *Section {
	s := &Section{Num: "2", Title: "Architecture Constraints"}
	c := s.sub("2.1", "Constraints")
	t := Table{Head: []string{"Constraint", "Value"}, Widths: []int{35, 65}}
	t.Rows = append(t.Rows, [][]Run{ctext("Target platform"), ctext(b.in.Cloud)})
	t.Rows = append(t.Rows, [][]Run{ctext("Compliance"), ctext(b.regimeList())})
	for _, id := range []string{"regions", "residency", "cap", "budget", "ops_maturity"} {
		if r := briefRow(b.in, id); r != nil && !r.Unknown {
			t.Rows = append(t.Rows, [][]Run{ctext(r.Label), ctext(r.Value)})
		}
	}
	for _, e := range b.live(register.CON, notBrief) {
		t.Rows = append(t.Rows, [][]Run{ctext(e.ID), cell(T(e.Statement), statusRun(e))})
	}
	c.add(t)

	a := s.sub("2.2", "Assumptions")
	var items [][]Run
	if b.hasRegister() {
		items = b.entryList(b.live(register.ASM, nil)).Items
	} else if b.in.Rationale != nil {
		for _, x := range b.in.Rationale.Assumptions {
			items = append(items, []Run{T(x)})
		}
	}
	var unknown []string
	for _, r := range b.in.Brief {
		if r.Unknown {
			unknown = append(unknown, r.Label)
		}
	}
	if len(items) > 0 {
		a.add(Ptext("The design was made under these assumptions. If one turns out to be wrong, revisit the decisions in section 9."), List{Items: items})
	} else {
		a.add(Ptext("No assumptions were recorded."))
	}
	if len(unknown) > 0 {
		a.add(Callout{Tone: "warn", Runs: []Run{B("Unknown in the Design Brief: "), T(strings.Join(unknown, ", ") + ". Confirm these with the stakeholders.")}})
	}
	if ng := b.live(register.NG, nil); len(ng) > 0 {
		s.sub("2.3", "Non-goals and out of scope").add(b.entryList(ng))
	}
	return s
}

// ---- 3 Context and Scope

func (b *builder) context() *Section {
	s := &Section{Num: "3", Title: "Context and Scope"}
	bc := s.sub("3.1", "Business context")
	var ext []*scene.Node
	for _, n := range b.order {
		if strings.HasPrefix(n.Kind, "external.") {
			ext = append(ext, n)
		}
	}
	if len(ext) == 0 {
		bc.add(Ptext("The diagram has no external actors or systems."))
		return s
	}
	bc.add(Ptext("The people and outside systems the system talks to, and how:"))
	t := Table{Head: []string{"Partner", "Type", "Interactions"}, Widths: []int{22, 22, 56}}
	for _, n := range ext {
		var lines []string
		for _, e := range b.g.Edges {
			switch n.ID {
			case e.From:
				lines = append(lines, fmt.Sprintf("→ %s (%s)", b.name(e.To), edgeText(e)))
			case e.To:
				lines = append(lines, fmt.Sprintf("← %s (%s)", b.name(e.From), edgeText(e)))
			}
		}
		t.Rows = append(t.Rows, [][]Run{cell(B(n.Label)), ctext(capital(b.kindText(n))), ctext(strings.Join(lines, "\n"))})
	}
	bc.add(t)
	return s
}

// ---- 4 Solution Strategy

func (b *builder) strategy() *Section {
	s := &Section{Num: "4", Title: "Solution Strategy"}
	r := b.in.Rationale
	if r == nil {
		s.add(Ptext("No Design Rationale was recorded for this version."))
		return s
	}
	if r.Summary != "" {
		s.add(Ptext(r.Summary))
	}
	if decs := b.decisionEntries(); b.hasRegister() && len(decs) > 0 {
		s.add(P(T("The key decisions (each is an ADR in section 9):")))
		t := Table{Head: []string{"Decision", "Chosen", "Over", "ADR"}, Widths: []int{34, 26, 26, 14}}
		for _, e := range decs {
			t.Rows = append(t.Rows, [][]Run{cell(T(e.Statement), statusRun(e)), cell(B(e.Fields["chose"])), ctext(e.Fields["over"]), ctext(ADRID(e.ID))})
		}
		s.add(t)
	} else if len(r.Tradeoffs) > 0 && !b.hasRegister() {
		s.add(P(T("The key trade-offs (each is recorded as a decision in section 9):")))
		t := Table{Head: []string{"Decision", "Chosen", "Over", "ADR"}, Widths: []int{34, 26, 26, 14}}
		for i, x := range r.Tradeoffs {
			t.Rows = append(t.Rows, [][]Run{ctext(x.Decision), cell(B(x.Chose)), ctext(x.Over), ctext(b.adr[i])})
		}
		s.add(t)
	}
	return s
}

// ---- 5 Building Block View

func (b *builder) buildingBlocks() *Section {
	s := &Section{Num: "5", Title: "Building Block View"}
	top := s.sub("5.1", "Whitebox: the overall system")
	top.add(Ptext("The system's boundaries and what each contains. Section 7 shows them on the diagram."))
	t := Table{Head: []string{"Boundary", "Type", "Contains", "Purpose"}, Widths: []int{20, 18, 32, 30}}
	for _, gr := range b.groups {
		var kids []string
		for _, n := range b.g.Nodes {
			if n.Parent == gr.ID {
				kids = append(kids, n.Label)
			}
		}
		for _, c := range b.g.Groups {
			if c.Parent == gr.ID {
				kids = append(kids, c.Label+" (boundary)")
			}
		}
		kind := b.in.GroupKinds[gr.Kind]
		if gr.Tier != "" {
			kind += " (" + gr.Tier + " tier)"
		}
		purpose := ""
		if b.in.Narrative != nil {
			purpose = b.in.Narrative.Groups[gr.ID]
		}
		t.Rows = append(t.Rows, [][]Run{cell(B(gr.Label)), ctext(capital(kind)), ctext(strings.Join(kids, ", ")), ctext(purpose)})
	}
	if len(t.Rows) > 0 {
		top.add(t)
	}
	ov := Table{Head: []string{"Component", "Where", "Responsibility"}, Widths: []int{24, 26, 50}}
	for _, n := range b.order {
		ov.Rows = append(ov.Rows, [][]Run{cell(B(n.Label)), ctext(b.path(n.Parent)), ctext(b.responsibilities(n)[0])})
	}
	top.add(P(T("Components:")), ov)

	det := s.sub("5.2", "Components")
	det.add(Ptext("Each component: what it is, what it is responsible for, why it was chosen, how it connects, and the compliance controls that apply to it."))
	for i, n := range b.order {
		det.Sub = append(det.Sub, b.component(fmt.Sprintf("5.2.%d", i+1), n))
	}
	return s
}

func (b *builder) component(num string, n *scene.Node) *Section {
	s := &Section{Num: num, Title: n.Label}
	facts := Table{Head: []string{"", ""}, Widths: []int{25, 75}}
	facts.Rows = append(facts.Rows, [][]Run{ctext("Kind"), ctext(capital(b.kindText(n)))})
	if name, docs := b.service(n); name != "" {
		v := []Run{T(name)}
		if docs != "" {
			v = []Run{L(name, docs)}
		}
		facts.Rows = append(facts.Rows, [][]Run{ctext("Service"), v})
	}
	facts.Rows = append(facts.Rows, [][]Run{ctext("Located in"), ctext(b.path(n.Parent))})
	if len(n.Classes) > 0 {
		facts.Rows = append(facts.Rows, [][]Run{ctext("Data"), ctext(strings.ToUpper(strings.Join(n.Classes, ", ")))})
	}
	for _, k := range sortedKeys(n.Attrs) {
		facts.Rows = append(facts.Rows, [][]Run{ctext(capital(strings.ReplaceAll(k, "_", " "))), ctext(n.Attrs[k])})
	}
	s.add(facts)

	s.add(P(B("Responsibilities")))
	var items [][]Run
	for _, r := range b.responsibilities(n) {
		items = append(items, []Run{T(r)})
	}
	s.add(List{Items: items})

	if why := b.why(n.ID); why != "" {
		s.add(P(B("Why it was chosen")), Ptext(why))
		if r := b.in.Rationale.Nodes[n.ID]; len(r.Practices) > 0 || len(r.Cites) > 0 {
			var runs []Run
			for _, p := range r.Practices {
				if len(runs) > 0 {
					runs = append(runs, T("; "))
				}
				title, _ := b.record(p)
				runs = append(runs, T(title))
			}
			for _, c := range r.Cites {
				if c.URL != "" {
					if len(runs) > 0 {
						runs = append(runs, T("; "))
					}
					runs = append(runs, L(orStr(c.Title, c.URL), c.URL))
				} else if c.Record != "" && !slices.Contains(r.Practices, c.Record) {
					if len(runs) > 0 {
						runs = append(runs, T("; "))
					}
					title, _ := b.record(c.Record)
					runs = append(runs, T(title))
				}
			}
			s.add(Para{Small: true, Runs: append([]Run{Muted("Based on: ")}, runs...)})
		}
	}

	var conns [][][]Run
	for _, e := range b.g.Edges {
		switch n.ID {
		case e.From:
			conns = append(conns, [][]Run{ctext("Calls / sends to"), cell(B(b.name(e.To))), ctext(orStr(e.Protocol, "–")), ctext(orStr(e.Label, styleWord(e.Style)))})
		case e.To:
			conns = append(conns, [][]Run{ctext("Called by / receives from"), cell(B(b.name(e.From))), ctext(orStr(e.Protocol, "–")), ctext(orStr(e.Label, styleWord(e.Style)))})
		}
	}
	if len(conns) > 0 {
		s.add(P(B("Interfaces")), Table{Head: []string{"Direction", "Peer", "Protocol", "Purpose"}, Widths: []int{22, 26, 18, 34}, Rows: conns, Compact: true})
	}

	if rows := b.componentChecks(n); len(rows) > 0 {
		s.add(P(B("Compliance checklist")), Table{Head: []string{"Status", "Control or practice", "Finding"}, Widths: []int{16, 38, 46}, Rows: rows, Compact: true})
	}
	return s
}

func orStr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func styleWord(style string) string {
	switch style {
	case "async":
		return "asynchronous"
	case "stream":
		return "stream"
	case "sync":
		return "request / response"
	}
	return ""
}

// status words, shared by every checklist.
func statusRuns(status string) []Run {
	switch status {
	case "met":
		return []Run{B("✔ Met")}
	case "unmet":
		return []Run{{Text: "✘ Unmet", Bold: true}}
	case "outside":
		return []Run{T("○ Outside diagram")}
	case "process":
		return []Run{T("○ Process evidence")}
	case "applied":
		return []Run{T("✔ Applied")}
	case "recommended":
		return []Run{T("➜ Recommended")}
	}
	return []Run{Muted("– Not checked")}
}

func (b *builder) componentChecks(n *scene.Node) [][][]Run {
	var rows [][][]Run
	seen := map[string]bool{}
	if gp := b.in.Gap; gp != nil {
		for _, it := range gp.Items {
			if !slices.Contains(it.Nodes, n.ID) {
				continue
			}
			seen[it.Record] = true
			_, runs := b.record(it.Record)
			rows = append(rows, [][]Run{statusRuns(it.Status), runs, ctext(it.Finding)})
		}
		for _, rc := range gp.Recommendations {
			if !slices.Contains(rc.Nodes, n.ID) {
				continue
			}
			seen[rc.Practice] = true
			title, _ := b.record(rc.Practice)
			rows = append(rows, [][]Run{statusRuns("recommended"), ctext(title), ctext(rc.Finding)})
		}
	}
	var applied []string
	if b.in.Rationale != nil {
		if r := b.in.Rationale.Nodes[n.ID]; r != nil {
			applied = r.Practices
		}
	}
	for _, id := range b.in.Applies[n.ID] {
		if seen[id] {
			continue
		}
		r := b.in.Records[id]
		if r == nil {
			continue
		}
		seen[id] = true
		_, runs := b.record(id)
		switch {
		case r.Type == "control":
			rows = append(rows, [][]Run{statusRuns(""), runs, ctext(r.Summary)})
		case slices.Contains(applied, id):
			rows = append(rows, [][]Run{statusRuns("applied"), runs, ctext(r.Summary)})
		}
	}
	for _, id := range applied {
		if seen[id] || b.in.Records[id] == nil {
			continue
		}
		seen[id] = true
		_, runs := b.record(id)
		rows = append(rows, [][]Run{statusRuns("applied"), runs, ctext(b.in.Records[id].Summary)})
	}
	return rows
}

// ---- 6 Runtime View

func (b *builder) runtime() *Section {
	s := &Section{Num: "6", Title: "Runtime View"}
	i := 0
	if b.in.Narrative != nil {
		for _, sc := range b.in.Narrative.Scenarios {
			i++
			sub := s.sub(fmt.Sprintf("6.%d", i), sc.Title)
			if sc.Summary != "" {
				sub.add(Ptext(sc.Summary))
			}
			var steps [][]Run
			for _, st := range sc.Steps {
				if b.g.Node(st.From) == nil || b.g.Node(st.To) == nil {
					continue // written for an earlier state of this version
				}
				runs := []Run{B(b.name(st.From)), T(" → "), B(b.name(st.To)), T(": " + st.What)}
				if e := b.edgeBetween(st.From, st.To); e != nil && e.Protocol != "" {
					runs = append(runs, Muted(" ("+e.Protocol+")"))
				}
				steps = append(steps, runs)
			}
			sub.add(List{Ordered: true, Items: steps})
		}
	}
	if i == 0 {
		s.add(Ptext("Runtime scenarios were not written for this version. Every connection is listed below."))
	}
	all := s.sub(fmt.Sprintf("6.%d", i+1), "All connections")
	t := Table{Head: []string{"From", "To", "Style", "Protocol", "Purpose"}, Widths: []int{20, 20, 14, 16, 30}, Compact: true}
	for _, e := range b.g.Edges {
		t.Rows = append(t.Rows, [][]Run{ctext(b.name(e.From)), ctext(b.name(e.To)), ctext(styleWord(e.Style)), ctext(orStr(e.Protocol, "–")), ctext(e.Label)})
	}
	all.add(t)
	return s
}

func (b *builder) edgeBetween(x, y string) *scene.Edge {
	for _, e := range b.g.Edges {
		if (e.From == x && e.To == y) || (e.From == y && e.To == x) {
			return e
		}
	}
	return nil
}

// ---- 7 Deployment View

func (b *builder) deployment() *Section {
	s := &Section{Num: "7", Title: "Deployment View", Landscape: true}
	d := s.sub("7.1", "Production Diagram")
	d.add(Diagram{Caption: fmt.Sprintf("Figure 1: Production Diagram v%d", b.in.Version)})

	inf := s.sub("7.2", "Infrastructure")
	t := Table{Head: []string{"Location", "Components"}, Widths: []int{38, 62}}
	locs := append([]string{""}, func() []string {
		var ids []string
		for _, gr := range b.groups {
			ids = append(ids, gr.ID)
		}
		return ids
	}()...)
	for _, loc := range locs {
		var names []string
		for _, n := range b.g.Nodes {
			if n.Parent == loc {
				names = append(names, n.Label)
			}
		}
		if len(names) > 0 {
			t.Rows = append(t.Rows, [][]Run{ctext(b.path(loc)), ctext(strings.Join(names, ", "))})
		}
	}
	inf.add(t)

	svc := s.sub("7.3", "Services used")
	hipaa := slices.Contains(b.in.Regimes, "hipaa")
	head := []string{"Component", "Kind", "Service"}
	widths := []int{30, 30, 40}
	if hipaa {
		head, widths = append(head, "HIPAA eligible"), []int{26, 26, 32, 16}
	}
	st := Table{Head: head, Widths: widths, Compact: true}
	for _, n := range b.order {
		name, docs := b.service(n)
		v := ctext(orStr(name, "–"))
		if docs != "" {
			v = []Run{L(name, docs)}
		}
		row := [][]Run{ctext(n.Label), ctext(n.Kind), v}
		if hipaa {
			e := "–"
			if sv, ok := b.in.Services[n.Service]; ok {
				e = map[bool]string{true: "Yes", false: "No"}[sv.HIPAA]
			}
			row = append(row, ctext(e))
		}
		st.Rows = append(st.Rows, row)
	}
	svc.add(st)
	if hipaa && b.in.HIPAAUnverified {
		svc.add(Callout{Tone: "warn", Runs: []Run{B("Check HIPAA eligibility with the provider. "), T("Designer's catalog flags have not been verified against the provider's current HIPAA-eligible services list.")}})
	}
	return s
}

// ---- 8 Crosscutting Concepts

func isShared(kind string) bool {
	return strings.HasPrefix(kind, "observability.") || slices.Contains([]string{"security.secrets", "security.key-management", "security.identity", "security.threat-detection", "data.backup"}, kind)
}

func (b *builder) concepts() *Section {
	s := &Section{Num: "8", Title: "Crosscutting Concepts"}
	i := 0
	if b.in.Narrative != nil {
		for _, c := range b.in.Narrative.Concepts {
			i++
			sub := s.sub(fmt.Sprintf("8.%d", i), c.Topic)
			sub.add(Ptext(c.Text))
			if len(c.Nodes) > 0 {
				sub.add(Para{Small: true, Runs: []Run{Muted("Components: "), T(b.names(c.Nodes))}})
			}
		}
	}
	var data [][][]Run
	for _, n := range b.order {
		if len(n.Classes) > 0 {
			data = append(data, [][]Run{ctext(n.Label), ctext(strings.ToUpper(strings.Join(n.Classes, ", "))), ctext(b.path(n.Parent))})
		}
	}
	i++
	dc := s.sub(fmt.Sprintf("8.%d", i), "Data classification")
	if len(data) > 0 {
		dc.add(Ptext("Components that store or receive classified data:"), Table{Head: []string{"Component", "Classification", "Location"}, Widths: []int{30, 25, 45}, Rows: data})
	} else {
		dc.add(Ptext("No component is marked as holding classified data."))
	}
	var shared [][]Run
	for _, n := range b.order {
		if isShared(n.Kind) {
			var users []string
			for _, e := range b.g.Edges {
				if e.To == n.ID {
					users = append(users, b.name(e.From))
				} else if e.From == n.ID {
					users = append(users, b.name(e.To))
				}
			}
			shared = append(shared, []Run{B(n.Label), T(": " + capital(b.kindText(n)) + ". Used by " + orStr(strings.Join(users, ", "), "nothing yet") + ".")})
		}
	}
	if len(shared) > 0 {
		i++
		s.sub(fmt.Sprintf("8.%d", i), "Shared platform services").add(List{Items: shared})
	}
	return s
}

// ---- 9 Architecture Decisions

func (b *builder) decisions() *Section {
	s := &Section{Num: "9", Title: "Architecture Decisions"}
	r := b.in.Rationale
	if b.hasRegister() {
		for i, e := range b.decisionEntries() {
			d := s.sub(fmt.Sprintf("9.%d", i+1), fmt.Sprintf("%s: %s", ADRID(e.ID), e.Statement))
			rows := [][][]Run{{ctext("Status"), ctext(decisionStatus(e))}, {ctext("Decision"), cell(B(orStatement(e.Fields["chose"], e)))}}
			if o := e.Fields["over"]; o != "" {
				rows = append(rows, [][]Run{ctext("Alternatives"), ctext(o)})
			}
			if w := e.Fields["because"]; w != "" {
				rows = append(rows, [][]Run{ctext("Rationale"), ctext(w)})
			}
			rows = append(rows, [][]Run{ctext("Register"), ctext(e.ID)})
			d.add(Table{Head: []string{"", ""}, Widths: []int{22, 78}, Rows: rows})
		}
	} else if r != nil {
		for i, x := range r.Tradeoffs {
			d := s.sub(fmt.Sprintf("9.%d", i+1), fmt.Sprintf("%s: %s", b.adr[i], x.Decision))
			d.add(Table{Head: []string{"", ""}, Widths: []int{22, 78}, Rows: [][][]Run{
				{ctext("Status"), ctext(fmt.Sprintf("Accepted (Production v%d)", b.in.Version))},
				{ctext("Decision"), cell(B(x.Chose))},
				{ctext("Alternatives"), ctext(x.Over)},
				{ctext("Rationale"), ctext(x.Because)},
			}})
		}
	}
	n := len(s.Sub) + 1
	c := s.sub(fmt.Sprintf("9.%d", n), "Component choices")
	t := Table{Head: []string{"Component", "Why it was chosen"}, Widths: []int{26, 74}}
	for _, nd := range b.order {
		if w := b.why(nd.ID); w != "" {
			t.Rows = append(t.Rows, [][]Run{cell(B(nd.Label)), ctext(w)})
		}
	}
	if len(t.Rows) > 0 {
		c.add(t)
	} else {
		c.add(Ptext("No reasons were recorded for the components."))
	}
	return s
}

// ---- 10 Quality Requirements

func (b *builder) quality() *Section {
	s := &Section{Num: "10", Title: "Quality Requirements"}
	if b.in.Narrative != nil && len(b.in.Narrative.Quality) > 0 {
		s.add(Ptext("Quality scenarios: how the architecture responds to the situations its quality goals (1.2) are about."))
		t := Table{Head: []string{"Quality", "Requirement", "Scenario", "Architecture response"}, Widths: []int{14, 20, 30, 36}}
		for _, q := range b.in.Narrative.Quality {
			resp := []Run{T(q.Response)}
			if len(q.Nodes) > 0 {
				resp = append(resp, Muted(" ("+b.names(q.Nodes)+")"))
			}
			t.Rows = append(t.Rows, [][]Run{cell(B(q.Attribute)), ctext(q.Requirement), ctext(q.Scenario), resp})
		}
		s.add(t)
		return s
	}
	s.add(Ptext("Quality scenarios were not written for this version; the targets are in 1.2."))
	return s
}

// ---- 11 Risks and Technical Debt

func (b *builder) risks() *Section {
	s := &Section{Num: "11", Title: "Risks and Technical Debt"}
	gp := b.in.Gap
	if gp == nil {
		s.add(Callout{Tone: "warn", Runs: []Run{B("No Gap Check has been run. "), T("Compliance gaps are unknown; run a Gap Check in Designer before relying on this document.")}})
	} else {
		var unmet [][][]Run
		for _, it := range gp.Items {
			if it.Status != "unmet" {
				continue
			}
			_, runs := b.record(it.Record)
			var fixes []string
			for _, f := range it.Fix {
				t, _ := b.record(f)
				fixes = append(fixes, t)
			}
			unmet = append(unmet, [][]Run{runs, ctext(b.names(it.Nodes)), ctext(it.Finding), ctext(strings.Join(fixes, "; "))})
		}
		r := s.sub("11.1", "Unmet controls")
		if len(unmet) > 0 {
			r.add(Table{Head: []string{"Control", "Components", "Finding", "Suggested fix"}, Widths: []int{24, 18, 34, 24}, Rows: unmet})
		} else {
			r.add(Ptext("The Gap Check found no unmet controls the diagram can show."))
		}
		var recs [][][]Run
		for _, rc := range gp.Recommendations {
			title, _ := b.record(rc.Practice)
			recs = append(recs, [][]Run{ctext(title), ctext(b.names(rc.Nodes)), ctext(rc.Finding)})
		}
		if len(recs) > 0 {
			s.sub("11.2", "Recommended security practices").add(Table{Head: []string{"Practice", "Components", "Why it matters"}, Widths: []int{28, 22, 50}, Rows: recs})
		}
		outside := 0
		for _, it := range gp.Items {
			if it.Status == "outside" {
				outside++
			}
		}
		if outside > 0 {
			s.sub(fmt.Sprintf("11.%d", len(s.Sub)+1), "Evidence needed outside the diagram").add(Ptext(fmt.Sprintf("%d controls can't be shown on a diagram (configuration, policies, procedures). Appendix A lists them with space for evidence and an owner.", outside)))
		}
	}
	if risks := b.live(register.RISK, nil); len(risks) > 0 {
		t := Table{Head: []string{"ID", "Risk", "Likelihood / impact", "Mitigation"}, Widths: []int{10, 42, 18, 30}}
		for _, e := range risks {
			t.Rows = append(t.Rows, [][]Run{ctext(e.ID), cell(T(e.Statement), statusRun(e)), ctext(strings.Trim(e.Fields["likelihood"]+" / "+e.Fields["impact"], " /")), ctext(e.Fields["mitigation"])})
		}
		s.sub(fmt.Sprintf("11.%d", len(s.Sub)+1), "Design risks").add(t)
	}
	if qs := b.live(register.Q, nil); len(qs) > 0 {
		s.sub(fmt.Sprintf("11.%d", len(s.Sub)+1), "Open questions").add(b.entryList(qs))
	}
	if b.hasRegister() {
		open := len(b.live(register.ASM, func(e register.Entry) bool { return e.Status == register.Open }))
		if open > 0 {
			s.sub(fmt.Sprintf("11.%d", len(s.Sub)+1), "Assumptions to validate").add(Ptext(fmt.Sprintf("%d assumptions in 2.2 are not confirmed yet. Each one that proves wrong is a risk to the decisions built on it.", open)))
		}
	} else if b.in.Rationale != nil && len(b.in.Rationale.Assumptions) > 0 {
		s.sub(fmt.Sprintf("11.%d", len(s.Sub)+1), "Assumptions to validate").add(Ptext(fmt.Sprintf("The design rests on %d assumptions (2.2). Each one that proves wrong is a risk to the decisions built on it.", len(b.in.Rationale.Assumptions))))
	}
	return s
}

// ---- 12 Glossary

func (b *builder) glossary() *Section {
	s := &Section{Num: "12", Title: "Glossary"}
	t := Table{Head: []string{"Term", "Meaning"}, Widths: []int{30, 70}}
	seen := map[string]bool{}
	var kinds []string
	for _, n := range b.order {
		if n.Kind != "" && !seen[n.Kind] {
			seen[n.Kind] = true
			kinds = append(kinds, n.Kind)
		}
	}
	slices.Sort(kinds)
	for _, k := range kinds {
		t.Rows = append(t.Rows, [][]Run{cell(M(k)), ctext(capital(b.in.Kinds[k]))})
	}
	terms := map[string]string{
		"PHI":                "Protected health information (HIPAA).",
		"PII":                "Personally identifiable information.",
		"RPO":                "Recovery point objective: how much data may be lost in a failure.",
		"RTO":                "Recovery time objective: how long recovery may take.",
		"Control":            "A requirement of a compliance regime (e.g. a SOC 2 criterion or a HIPAA safeguard).",
		"Practice":           "A design or security practice that helps satisfy controls.",
		"Outside diagram":    "A control the diagram can't show; it needs evidence such as configuration or procedures.",
		"ADR":                "Architecture Decision Record (section 9).",
		"Production Diagram": "The typed, production-ready architecture diagram this document describes.",
	}
	for _, k := range sortedKeys(terms) {
		t.Rows = append(t.Rows, [][]Run{cell(B(k)), ctext(terms[k])})
	}
	s.add(t)
	return s
}

// ---- Appendix A: system compliance checklist

func (b *builder) checklist() *Section {
	s := &Section{Num: "A", Title: "Appendix: System Compliance Checklist"}
	s.add(Callout{Tone: "info", Runs: []Run{T("One row per control that applies to this design. ✔ Met and ✘ Unmet are judged from the diagram; Outside diagram and Process rows need evidence. Fill in the Evidence column during review.")}})
	type row struct {
		regime, status, record, finding string
		nodes                           []string
	}
	var rows []row
	if gp := b.in.Gap; gp != nil {
		for _, it := range gp.Items {
			st := it.Status
			if it.Coverage == "process" {
				st = "process"
			}
			rows = append(rows, row{it.Regime, st, it.Record, it.Finding, it.Nodes})
		}
	} else {
		byRecord := map[string][]string{}
		for id, recs := range b.in.Applies {
			for _, r := range recs {
				if rec := b.in.Records[r]; rec != nil && rec.Type == "control" {
					byRecord[r] = append(byRecord[r], id)
				}
			}
		}
		for _, id := range sortedKeys(byRecord) {
			rec := b.in.Records[id]
			slices.Sort(byRecord[id])
			rows = append(rows, row{rec.Regime, "", id, rec.Summary, byRecord[id]})
		}
		s.add(Callout{Tone: "warn", Runs: []Run{T("No Gap Check has been run on this version, so nothing is judged yet.")}})
	}
	if len(rows) == 0 {
		s.add(Ptext("No compliance controls apply: the Design Brief selects no regime, or none of its controls match this design."))
		return s
	}
	regimes := []string{}
	for _, r := range rows {
		if !slices.Contains(regimes, r.regime) {
			regimes = append(regimes, r.regime)
		}
	}
	slices.Sort(regimes)
	for i, rg := range regimes {
		title := b.regimeName(rg)
		if rg == "" {
			title = "Other controls"
		}
		sub := s.sub(fmt.Sprintf("A.%d", i+1), title)
		t := Table{Head: []string{"Status", "Control", "Components", "Finding", "Evidence / owner"}, Widths: []int{13, 25, 17, 27, 18}, Compact: true}
		met, total := 0, 0
		for _, r := range rows {
			if r.regime != rg {
				continue
			}
			total++
			if r.status == "met" {
				met++
			}
			_, runs := b.record(r.record)
			t.Rows = append(t.Rows, [][]Run{statusRuns(r.status), runs, ctext(orStr(b.names(r.nodes), "Whole system")), ctext(r.finding), ctext("")})
		}
		sub.add(P(Muted(fmt.Sprintf("%d of %d met on the diagram.", met, total))), t)
	}
	return s
}

// ---- Appendix B: sources

func (b *builder) sources() *Section {
	s := &Section{Num: "B", Title: "Appendix: Sources"}
	s.add(P(T("Structure: the arc42 architecture documentation template ("), L("arc42.org", "https://arc42.org"), T(") by Dr. Peter Hruschka, Dr. Gernot Starke and contributors, licensed "), L("CC BY-SA 4.0", "https://creativecommons.org/licenses/by-sa/4.0/"), T(". Its section headings are used; the text is generated by Designer from this project.")))
	s.add(Ptext(fmt.Sprintf("Knowledge Base records referred to in this document (Knowledge Base version %s).", orStr(b.in.KBVersion, "unknown"))))
	t := Table{Head: []string{"Record", "Title", "Sources"}, Widths: []int{30, 30, 40}, Compact: true}
	for _, id := range sortedKeys(b.mentioned) {
		r := b.in.Records[id]
		if r == nil {
			continue
		}
		var src []Run
		for i, x := range r.Sources {
			if i > 0 {
				src = append(src, T("\n"))
			}
			src = append(src, L(x.Title, x.URL))
			if x.AsOf != "" {
				src = append(src, Muted(" (as of "+x.AsOf+")"))
			}
			if x.Attribution != "" {
				src = append(src, Muted(". "+x.Attribution))
			}
		}
		t.Rows = append(t.Rows, [][]Run{cell(M(id)), ctext(r.Title), src})
	}
	if len(t.Rows) == 0 {
		s.add(Ptext("None."))
	} else {
		s.add(t)
	}
	return s
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
