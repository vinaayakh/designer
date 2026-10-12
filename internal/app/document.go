package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/vinaayakh/designer/internal/brief"
	"github.com/vinaayakh/designer/internal/catalog"
	"github.com/vinaayakh/designer/internal/kb"
	"github.com/vinaayakh/designer/internal/proposal"
	"github.com/vinaayakh/designer/internal/report"
	"github.com/vinaayakh/designer/internal/scene"
)

// ---- arc42 document: the Assistant writes what stored data can't say

type docSpec struct {
	Version int
	Graph   *scene.Graph
}

// DocumentStatus says whether version n has document text, and whether the
// diagram changed since it was written.
func (a *App) DocumentStatus(n int) (written, stale bool, err error) {
	g, err := a.versionGraph(n)
	if err != nil {
		return false, false, err
	}
	nar, err := a.narrative(n)
	if err != nil || nar == nil {
		return false, false, err
	}
	return true, nar.Stale(g), nil
}

func (a *App) versionGraph(n int) (*scene.Graph, error) {
	_, data, err := a.P.Version(n)
	if err != nil {
		return nil, fmt.Errorf("no Production version %d", n)
	}
	return scene.Parse(data, true)
}

func (a *App) narrative(n int) (*report.Narrative, error) {
	raw, err := a.P.DocText(n)
	if err != nil || raw == nil {
		return nil, err
	}
	var nar report.Narrative
	if err := json.Unmarshal(raw, &nar); err != nil {
		return nil, err
	}
	return &nar, nil
}

// WriteDocument starts the task that writes version n's document text.
func (a *App) WriteDocument(n int) error {
	if _, err := a.versionGraph(n); err != nil {
		return err
	}
	a.mu.Lock()
	model := a.settings.ConversionModel
	a.mu.Unlock()
	return a.run(&taskSpec{kind: "document", expect: "submit_document", model: model, build: func() (string, error) { return a.documentMessageLocked(n) }})
}

func (a *App) documentMessageLocked(n int) (string, error) {
	b, err := a.Brief()
	if err != nil {
		return "", err
	}
	g, err := a.versionGraph(n)
	if err != nil {
		return "", err
	}
	a.doc = &docSpec{Version: n, Graph: g}
	al := a.sess.Aliases
	v, _, _ := a.P.Version(n)
	var rat proposal.Rationale
	if len(v.Rationale) > 0 {
		_ = json.Unmarshal(v.Rationale, &rat)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "TASK: Write the arc42 document text for Production Diagram v%d.\n\n%s\n\nProduction Diagram v%d:\n%s\n", n, b.Compact(), n, strings.TrimRight(g.Compact(al), "\n"))
	if d := a.requirementsDigest(); d != "" {
		sb.WriteString("\n" + d)
	}
	if rat.Summary != "" || len(rat.Nodes) > 0 {
		sb.WriteString("\nDesign Rationale (already in the document; don't repeat it, build on it):\n")
		if rat.Summary != "" {
			sb.WriteString("summary: " + rat.Summary + "\n")
		}
		for _, nd := range g.Nodes {
			if r := rat.Nodes[nd.ID]; r != nil && r.Why != "" {
				fmt.Fprintf(&sb, "- %s why: %s\n", al.Of(nd.ID, false), r.Why)
			}
		}
	}
	fmt.Fprintf(&sb, `
The document already has, from stored data: the Brief, constraints, assumptions, trade-offs, each component's kind, service, location, connections and why it was chosen, and the compliance checklists. Write only what's missing, plainly and specifically for THIS design (no generic filler). Then call submit_document once with:
{"version": %d,
 "overview": "<2–4 sentences: what the system does, for whom, and its overall shape>",
 "components": {"<node alias>": ["<responsibility>", ...]},   // EVERY Node; 1–4 responsibilities each, each one short sentence saying what it does in this system
 "groups": {"<group alias>": "<one sentence: the purpose of this boundary>"},   // every Group
 "scenarios": [{"title": "...", "summary": "<one sentence>", "steps": [{"from": "<alias>", "to": "<alias>", "what": "<what happens>"}]}],   // 2–5 key runtime flows (e.g. the main user request, a write, an async job, a failure or recovery); each step must follow an Edge on the diagram (either direction)
 "concepts": [{"topic": "...", "text": "<2–4 sentences>", "nodes": ["<alias>"]}],   // 3–7 crosscutting concepts this design actually uses (e.g. identity and access, encryption and keys, observability, backup and recovery, scaling, network security)
 "quality": [{"attribute": "...", "requirement": "<the target>", "scenario": "<stimulus and context>", "response": "<how this design responds>", "nodes": ["<alias>"]}]}   // one per quality attribute with a target in the Brief OR in the Rationale's assumptions (availability, latency, throughput, RPO, RTO, scale...); say "(assumed)" after an assumed target
Write names, not aliases, inside the text fields (aliases only in "from", "to", "nodes" and keys). Never change the diagram in this task. If submit_document reports problems, fix them all and submit again. Then reply with one sentence.`, n)
	return sb.String(), nil
}

func (a *App) submitDocumentLocked(raw json.RawMessage) (string, error) {
	ds := a.doc
	if a.turn == nil || a.turn.Kind != "document" || ds == nil {
		return "", errors.New("submit_document is only used during the document task")
	}
	var in struct {
		Version    int                 `json:"version"`
		Overview   string              `json:"overview"`
		Components map[string][]string `json:"components"`
		Groups     map[string]string   `json:"groups"`
		Scenarios  []report.Scenario   `json:"scenarios"`
		Concepts   []report.Concept    `json:"concepts"`
		Quality    []report.Quality    `json:"quality"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("document: %w", err)
	}
	g := ds.Graph
	var errs []string
	add := func(f string, args ...any) { errs = append(errs, fmt.Sprintf(f, args...)) }
	node := func(where, al string) string {
		id, ok := a.sess.Aliases.ID(al)
		if !ok || g.Node(id) == nil {
			add("%s: %q is not a Node on v%d", where, al, ds.Version)
			return ""
		}
		return id
	}
	nodes := func(where string, als []string) []string {
		var out []string
		for _, al := range als {
			if id := node(where, al); id != "" {
				out = append(out, id)
			}
		}
		return out
	}
	text := func(where, s string, maxLen int) string {
		s = strings.TrimSpace(s)
		if s == "" {
			add("%s is empty", where)
		} else if len(s) > maxLen {
			add("%s is too long (at most %d characters)", where, maxLen)
		}
		return s
	}

	nar := &report.Narrative{Version: ds.Version, Created: time.Now().UTC().Truncate(time.Second), Model: a.settings.ConversionModel,
		Components: map[string][]string{}, Groups: map[string]string{}}
	if in.Version != ds.Version {
		add("version must be %d", ds.Version)
	}
	nar.Overview = text("overview", in.Overview, 900)
	for _, al := range sortedKeys(in.Components) {
		id := node("components", al)
		if id == "" {
			continue
		}
		rs := in.Components[al]
		if len(rs) == 0 || len(rs) > 4 {
			add("components.%s: give 1–4 responsibilities", al)
		}
		for i, r := range rs {
			nar.Components[id] = append(nar.Components[id], text(fmt.Sprintf("components.%s[%d]", al, i), r, 240))
		}
	}
	for _, nd := range g.Nodes {
		if _, ok := nar.Components[nd.ID]; !ok {
			add("components: missing %s (%s)", a.sess.Aliases.Of(nd.ID, false), nd.Label)
		}
	}
	for _, al := range sortedKeys(in.Groups) {
		id, ok := a.sess.Aliases.ID(al)
		if !ok || g.Group(id) == nil {
			add("groups: %q is not a Group on v%d", al, ds.Version)
			continue
		}
		nar.Groups[id] = text("groups."+al, in.Groups[al], 300)
	}
	if len(in.Scenarios) < 1 || len(in.Scenarios) > 6 {
		add("scenarios: give 2–5")
	}
	for i, sc := range in.Scenarios {
		where := fmt.Sprintf("scenarios[%d]", i)
		out := report.Scenario{Title: text(where+".title", sc.Title, 100), Summary: strings.TrimSpace(sc.Summary)}
		if len(sc.Steps) == 0 {
			add("%s has no steps", where)
		}
		for j, st := range sc.Steps {
			w := fmt.Sprintf("%s.steps[%d]", where, j)
			from, to := node(w+".from", st.From), node(w+".to", st.To)
			if from != "" && to != "" && !connected(g, from, to) {
				add("%s: there is no Edge between %s and %s", w, st.From, st.To)
			}
			out.Steps = append(out.Steps, report.Step{From: from, To: to, What: text(w+".what", st.What, 300)})
		}
		nar.Scenarios = append(nar.Scenarios, out)
	}
	for i, c := range in.Concepts {
		where := fmt.Sprintf("concepts[%d]", i)
		nar.Concepts = append(nar.Concepts, report.Concept{Topic: text(where+".topic", c.Topic, 80), Text: text(where+".text", c.Text, 1200), Nodes: nodes(where, c.Nodes)})
	}
	for i, q := range in.Quality {
		where := fmt.Sprintf("quality[%d]", i)
		nar.Quality = append(nar.Quality, report.Quality{Attribute: text(where+".attribute", q.Attribute, 60), Requirement: strings.TrimSpace(q.Requirement),
			Scenario: text(where+".scenario", q.Scenario, 400), Response: text(where+".response", q.Response, 600), Nodes: nodes(where, q.Nodes)})
	}
	if len(errs) > 0 {
		return "", fmt.Errorf("the document text was not saved:\n- %s", strings.Join(errs, "\n- "))
	}
	name := func(s string) string { return a.nameAliasesLocked(s, g, nil) }
	nar.Overview = name(nar.Overview)
	for _, rs := range nar.Components {
		for i := range rs {
			rs[i] = name(rs[i])
		}
	}
	for id, s := range nar.Groups {
		nar.Groups[id] = name(s)
	}
	for i := range nar.Scenarios {
		nar.Scenarios[i].Summary = name(nar.Scenarios[i].Summary)
		for j := range nar.Scenarios[i].Steps {
			nar.Scenarios[i].Steps[j].What = name(nar.Scenarios[i].Steps[j].What)
		}
	}
	for i := range nar.Concepts {
		nar.Concepts[i].Text = name(nar.Concepts[i].Text)
	}
	for i := range nar.Quality {
		nar.Quality[i].Scenario, nar.Quality[i].Response = name(nar.Quality[i].Scenario), name(nar.Quality[i].Response)
	}
	data, _ := json.MarshalIndent(nar, "", "  ")
	if err := a.P.SaveDocText(ds.Version, data); err != nil {
		return "", err
	}
	a.markSubmittedLocked("submit_document")
	a.doc = nil
	a.emit("document", map[string]any{"version": nar.Version})
	return fmt.Sprintf("Document text saved for v%d.", nar.Version), nil
}

func connected(g *scene.Graph, x, y string) bool {
	for _, e := range g.Edges {
		if (e.From == x && e.To == y) || (e.From == y && e.To == x) {
			return true
		}
	}
	return false
}

// ---- export

// DocumentInput gathers everything the arc42 document of version n is built from.
func (a *App) DocumentInput(n int) (*report.Input, error) {
	v, data, err := a.P.Version(n)
	if err != nil {
		return nil, fmt.Errorf("no Production version %d", n)
	}
	g, err := scene.Parse(data, true)
	if err != nil {
		return nil, err
	}
	b, err := a.Brief()
	if err != nil {
		return nil, err
	}
	m, _ := a.P.Meta()
	in := &report.Input{
		Project: m.Name, Version: n, Created: v.Created, Generated: time.Now().UTC(), Graph: g,
		RegimeNames: map[string]string{}, Kinds: map[string]string{}, GroupKinds: map[string]string{},
		Services: map[string]report.Service{}, Applies: map[string][]string{}, Records: map[string]*kb.Record{}, KBVersion: a.KB.Version,
	}

	schema := brief.TheSchema()
	label := func(f *brief.Field, val string) string {
		for _, x := range f.Values {
			if x.ID == val {
				return x.Label
			}
		}
		return val
	}
	for _, f := range schema.Fields {
		f := f
		if f.ID == "compliance" {
			var names []string
			for _, r := range b.Compliance {
				in.RegimeNames[r] = label(&f, r)
				names = append(names, label(&f, r))
				if r != "none" {
					in.Regimes = append(in.Regimes, r)
				}
			}
			in.Brief = append(in.Brief, report.BriefRow{ID: f.ID, Label: f.Label, Value: strings.Join(names, ", ")})
			continue
		}
		val := b.Get(f.ID)
		in.Brief = append(in.Brief, report.BriefRow{ID: f.ID, Label: f.Label, Value: label(&f, val), Unknown: val == brief.Unknown || val == ""})
	}
	if strings.TrimSpace(b.Notes) != "" {
		in.Brief = append(in.Brief, report.BriefRow{ID: "notes", Label: "Notes", Value: b.Notes})
	}

	cloud := b.Get("cloud")
	in.Cloud = label(schema.Field("cloud"), cloud)
	for _, k := range a.Cat.Kinds {
		in.Kinds[k.ID] = k.Gloss
	}
	for _, k := range a.Cat.GroupKinds {
		in.GroupKinds[k.ID] = k.Gloss
	}
	if c := a.Cat.Clouds[cloud]; c != nil {
		in.Cloud = c.Display
		in.HIPAAUnverified = c.HIPAAAsOf == nil
		for _, s := range c.Services {
			in.Services[s.ID] = report.Service{Name: s.Name, Docs: s.Docs, HIPAA: s.HIPAA}
		}
	}

	if len(v.Rationale) > 0 {
		var r proposal.Rationale
		if json.Unmarshal(v.Rationale, &r) == nil {
			in.Rationale = &r
		}
	}

	// The newest Gap Report of this version, else of the newest earlier version.
	refs, _ := a.P.Reports()
	var best *struct {
		file string
		ver  int
	}
	for _, r := range refs {
		if r.Version <= n && (best == nil || r.Version >= best.ver) {
			best = &struct {
				file string
				ver  int
			}{r.File, r.Version}
		}
	}
	if best != nil {
		if raw, err := a.P.Report(best.file); err == nil {
			var gp report.Gap
			if json.Unmarshal(raw, &gp) == nil {
				in.Gap = &gp
			}
		}
	}

	// Records that apply to each Node.
	for _, nd := range g.Nodes {
		if nd.Kind == "" {
			continue
		}
		controls, practices := a.KB.Applies([]kb.Node{{Alias: nd.ID, Kind: nd.Kind, DataClasses: nd.Classes}}, cloud, b.Matches, catalog.MatchKind)
		for _, x := range append(controls, practices...) {
			if len(x.Nodes) > 0 && !slices.Contains(in.Applies[nd.ID], x.Record.ID) {
				in.Applies[nd.ID] = append(in.Applies[nd.ID], x.Record.ID)
			}
		}
	}
	for _, r := range a.KB.Records {
		in.Records[r.ID] = r
	}

	if nar, err := a.narrative(n); err == nil && nar != nil {
		in.Narrative = nar
	}
	if a.Reg != nil {
		in.Entries = a.Reg.Entries()
	}
	tidyInput(in)
	return in, nil
}
