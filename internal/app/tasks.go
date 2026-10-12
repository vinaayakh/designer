package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/vinaayakh/designer/internal/catalog"
	"github.com/vinaayakh/designer/internal/kb"
	"github.com/vinaayakh/designer/internal/scene"
)

// ---- Conversion: Design task (ticket 011)

// PreCheck runs the local, token-free check before Conversion.
func (a *App) PreCheck() ([]scene.Issue, error) {
	g, _, err := a.Graph(ViewRough)
	if err != nil {
		return nil, err
	}
	issues := g.PreCheck()
	if len(g.Nodes) == 0 {
		issues = append([]scene.Issue{{Code: "empty", Message: "the Rough Draft has no shapes yet"}}, issues...)
	}
	return issues, nil
}

// Convert starts the Design task.
func (a *App) Convert() error {
	if n, _ := a.P.Latest(); n > 0 {
		return errors.New("Conversion already happened for this Project")
	}
	b, err := a.Brief()
	if err != nil {
		return err
	}
	if !b.Complete() {
		return errors.New("answer the required Design Brief questions first")
	}
	if p := a.PendingProposal(); p != nil {
		return fmt.Errorf("Proposal %s is waiting for review; accept or reject it first", p.ID)
	}
	a.mu.Lock()
	model := a.settings.ConversionModel
	a.mu.Unlock()
	return a.run(&taskSpec{kind: "design", expect: "submit_proposal", model: model, build: a.designMessageLocked})
}

func (a *App) designMessageLocked() (string, error) {
	b, err := a.Brief()
	if err != nil {
		return "", err
	}
	ctx, err := a.contextLocked(ViewRough, nil)
	if err != nil {
		return "", err
	}
	cloud := b.Get("cloud")
	var sb strings.Builder
	sb.WriteString("TASK: Conversion Design.\n\n")
	sb.WriteString(ctx)
	sb.WriteString("\n\n")
	sb.WriteString(a.requirementsDigest())
	if export := a.Cat.Export(cloud); export != "" {
		sb.WriteString(export + "\n")
	} else {
		sb.WriteString("Target is Generic: use Component Kinds only, no Services.\n\n")
	}
	fmt.Fprintf(&sb, `Turn this Rough Draft into a production-ready Production Diagram for the %s target that meets the Design Brief.
- Keep the user's intent. Every Rough Draft Node and Group you keep is typed in place (update_node / update_group with set and parent), so it keeps its Alias. Add what production needs (add_node, add_group, add_edge); remove only what is clearly wrong or redundant, and say why.
- When done, every Node has a kind (and on AWS/Azure a catalog service, or service_name for something off-catalog), every Group a group kind, every Edge a style (sync/async/stream) and protocol where known. Put Nodes that hold data in the right subnets and give them a Data Classification when the Brief or labels imply it.
- Look up practices with search_kb first (e.g. for compliance regimes, availability targets and the data stores you use). Use the web only if the Knowledge Base has nothing useful.
- Where the drawing is ambiguous (e.g. what "DB" is), choose and list it under assumptions. List every unknown Brief field you had to assume.
- Submit once with submit_proposal: "kind": "conversion", "view": "production", a short title and summary, all changes, and "rationale": {"summary", "assumptions": [...], "nodes": {"<alias or +ref>": {"why", "practices": [record ids], "cites": [{"record": id} | {"url", "title"}]}}, "tradeoffs": [{"decision", "chose", "over", "because"}]}. Give a rationale entry for every Node.
- If submit_proposal reports problems, fix them all and submit again. Then reply with one or two sentences.`, cloud)
	return sb.String(), nil
}

// ---- Gap Check

type gapSpec struct {
	Version   int
	Controls  map[string]kb.Applied // non-process Controls the AI must judge
	Process   []kb.Applied          // filled in by the runtime
	Practices map[string]bool       // security practice candidates
}

// GapCheck starts a Gap Check on one Production version, waiting briefly if
// the Assistant is finishing another turn.
func (a *App) GapCheck(version int) error {
	deadline := time.Now().Add(2 * time.Minute)
	for {
		err := a.gapCheck(version)
		if !errors.Is(err, ErrBusy) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(time.Second)
	}
}

func (a *App) gapCheck(version int) error {
	a.mu.Lock()
	model := a.settings.ConversionModel
	a.mu.Unlock()
	spec := &taskSpec{kind: "gap", expect: "submit_gap_report", model: model}
	spec.build = func() (string, error) { return a.gapMessageLocked(version) }
	return a.run(spec)
}

func (a *App) gapSpecFor(version int, g *scene.Graph) (*gapSpec, error) {
	b, err := a.Brief()
	if err != nil {
		return nil, err
	}
	var nodes []kb.Node
	for _, n := range g.Nodes {
		nodes = append(nodes, kb.Node{Alias: n.ID, Kind: n.Kind, DataClasses: n.Classes})
	}
	controls, practices := a.KB.Applies(nodes, b.Get("cloud"), b.Matches, catalog.MatchKind)
	gs := &gapSpec{Version: version, Controls: map[string]kb.Applied{}, Practices: map[string]bool{}}
	for _, c := range controls {
		if c.Record.Coverage == "process" {
			gs.Process = append(gs.Process, c)
		} else {
			gs.Controls[c.Record.ID] = c
		}
	}
	for _, p := range practices {
		if p.Record.Domain == "security" {
			gs.Practices[p.Record.ID] = true
		}
	}
	return gs, nil
}

func (a *App) gapMessageLocked(version int) (string, error) {
	b, err := a.Brief()
	if err != nil {
		return "", err
	}
	latest, _ := a.P.Latest()
	var diagram string
	var g *scene.Graph
	if version == latest {
		diagram, err = a.contextLocked(ViewProduction, nil)
		if err != nil {
			return "", err
		}
		g, _, err = a.Graph(ViewProduction)
	} else {
		_, data, err2 := a.P.Version(version)
		if err2 != nil {
			return "", err2
		}
		g, err = scene.Parse(data, true)
		if err == nil {
			diagram = fmt.Sprintf("Production Diagram v%d (an older version; the current one is v%d):\n%s", version, latest, strings.TrimRight(g.Compact(a.sess.Aliases), "\n"))
		}
	}
	if err != nil {
		return "", err
	}
	gs, err := a.gapSpecFor(version, g)
	if err != nil {
		return "", err
	}
	a.gap = gs
	al := a.sess.Aliases
	aliases := func(ids []string) string {
		var out []string
		for _, id := range ids {
			out = append(out, al.Of(id, false))
		}
		return strings.Join(out, " ")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "TASK: Gap Check of Production Diagram v%d.\n\n%s\n\n", version, diagram)
	regimes := slices.DeleteFunc(slices.Clone(b.Compliance), func(s string) bool { return s == "none" })
	if len(gs.Controls) > 0 {
		fmt.Fprintf(&sb, "Controls that apply (%s); judge each one:\n", strings.Join(regimes, ", "))
		for _, id := range sortedKeys(gs.Controls) {
			c := gs.Controls[id]
			fmt.Fprintf(&sb, "- %s | %s | %s", c.Record.ID, c.Record.Title, c.Record.Summary)
			if len(c.Nodes) > 0 {
				fmt.Fprintf(&sb, " | Nodes: %s", aliases(c.Nodes))
			}
			if c.Record.Coverage == "partial" {
				sb.WriteString(" | partial: the diagram can show the component, not its configuration")
			}
			if len(c.Record.SatisfiedBy) > 0 {
				fmt.Fprintf(&sb, " | satisfied by: %s", strings.Join(c.Record.SatisfiedBy, ", "))
			}
			sb.WriteString("\n")
		}
	} else if len(regimes) == 0 {
		sb.WriteString("The Brief has no compliance regime, so there are no Controls to judge; the report is security recommendations only.\n")
	} else {
		sb.WriteString("No Controls in this Knowledge Base apply to this diagram.\n")
	}
	if len(gs.Process) > 0 {
		var ids []string
		for _, p := range gs.Process {
			ids = append(ids, p.Record.ID)
		}
		fmt.Fprintf(&sb, "Process Controls (policy or procedure) are filed by the runtime; leave them out: %s\n", strings.Join(ids, ", "))
	}
	if len(gs.Practices) > 0 {
		sb.WriteString("Candidate security practices that matched the diagram:\n")
		for _, id := range sortedKeys(gs.Practices) {
			r := a.KB.Get(id)
			fmt.Fprintf(&sb, "- %s | %s | %s\n", r.ID, r.Title, r.Summary)
		}
	}
	fmt.Fprintf(&sb, `
Use get_record only where you need a Record's detail. Then call submit_gap_report once with:
{"version": %d,
 "items": [{"record": "<control id>", "status": "met" | "unmet" | "outside", "nodes": ["n1"], "finding": "<at most 2 sentences>", "fix": ["<practice ids>"]}],   // exactly one per Control listed above
 "recommendations": [{"practice": "<practice id>", "nodes": ["n1"], "finding": "<why it matters here, at most 2 sentences>"}]}   // security practices not yet applied that matter for this design
"outside" means the diagram can't show it. Never change the diagram in this task. Then reply with one sentence.`, version)
	return sb.String(), nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// GapItem / GapReport are what is saved (never edited afterwards).
type GapItem struct {
	Record    string   `json:"record"`
	Title     string   `json:"title"`
	Regime    string   `json:"regime,omitempty"`
	Coverage  string   `json:"coverage"`
	Status    string   `json:"status"`
	Nodes     []string `json:"nodes,omitempty"` // element IDs
	Finding   string   `json:"finding"`
	Fix       []string `json:"fix,omitempty"`
	ByRuntime bool     `json:"byRuntime,omitempty"`
}

type GapRec struct {
	Practice string   `json:"practice"`
	Title    string   `json:"title"`
	Nodes    []string `json:"nodes,omitempty"`
	Finding  string   `json:"finding"`
}

type GapReport struct {
	Version         int       `json:"version"`
	KBVersion       string    `json:"kbVersion"`
	Regimes         []string  `json:"regimes"`
	Created         time.Time `json:"created"`
	Items           []GapItem `json:"items"`
	Recommendations []GapRec  `json:"recommendations"`
}

func (a *App) submitGapReportLocked(raw json.RawMessage) (string, error) {
	gs := a.gap
	if a.turn == nil || a.turn.Kind != "gap" || gs == nil {
		return "", errors.New("submit_gap_report is only used during a Gap Check task")
	}
	var in struct {
		Version int `json:"version"`
		Items   []struct {
			Record  string   `json:"record"`
			Status  string   `json:"status"`
			Nodes   []string `json:"nodes"`
			Finding string   `json:"finding"`
			Fix     []string `json:"fix"`
		} `json:"items"`
		Recommendations []struct {
			Practice string   `json:"practice"`
			Nodes    []string `json:"nodes"`
			Finding  string   `json:"finding"`
		} `json:"recommendations"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("report: %w", err)
	}
	var errs []string
	if in.Version != gs.Version {
		errs = append(errs, fmt.Sprintf("version must be %d", gs.Version))
	}
	toIDs := func(where string, aliases []string) []string {
		var ids []string
		for _, al := range aliases {
			id, ok := a.sess.Aliases.ID(al)
			if !ok {
				errs = append(errs, fmt.Sprintf("%s: %q is not a Node alias", where, al))
				continue
			}
			ids = append(ids, id)
		}
		return ids
	}
	b, _ := a.Brief()
	rep := GapReport{Version: gs.Version, KBVersion: a.KB.Version, Created: time.Now().UTC().Truncate(time.Second)}
	for _, r := range b.Compliance {
		if r != "none" {
			rep.Regimes = append(rep.Regimes, r)
		}
	}
	seen := map[string]bool{}
	for _, it := range in.Items {
		c, ok := gs.Controls[it.Record]
		if !ok {
			errs = append(errs, fmt.Sprintf("item %q is not one of the Controls listed for this check", it.Record))
			continue
		}
		if seen[it.Record] {
			errs = append(errs, fmt.Sprintf("item %q appears twice", it.Record))
		}
		seen[it.Record] = true
		if !slices.Contains([]string{"met", "unmet", "outside"}, it.Status) {
			errs = append(errs, fmt.Sprintf("%s: status must be met, unmet or outside", it.Record))
		}
		for _, f := range it.Fix {
			if r := a.KB.Get(f); r == nil || r.Type != "practice" {
				errs = append(errs, fmt.Sprintf("%s: fix %q is not a practice Record", it.Record, f))
			}
		}
		rep.Items = append(rep.Items, GapItem{Record: it.Record, Title: c.Record.Title, Regime: c.Record.Regime, Coverage: c.Record.Coverage,
			Status: it.Status, Nodes: toIDs(it.Record, it.Nodes), Finding: it.Finding, Fix: it.Fix})
	}
	for _, id := range sortedKeys(gs.Controls) {
		if !seen[id] {
			errs = append(errs, fmt.Sprintf("missing an item for %s", id))
		}
	}
	for _, rc := range in.Recommendations {
		r := a.KB.Get(rc.Practice)
		if r == nil || r.Type != "practice" {
			errs = append(errs, fmt.Sprintf("recommendation %q is not a practice Record", rc.Practice))
			continue
		}
		rep.Recommendations = append(rep.Recommendations, GapRec{Practice: rc.Practice, Title: r.Title, Nodes: toIDs(rc.Practice, rc.Nodes), Finding: rc.Finding})
	}
	if len(errs) > 0 {
		return "", fmt.Errorf("the Gap Report was not saved:\n- %s", strings.Join(errs, "\n- "))
	}
	for _, p := range gs.Process {
		rep.Items = append(rep.Items, GapItem{Record: p.Record.ID, Title: p.Record.Title, Regime: p.Record.Regime, Coverage: "process",
			Status: "outside", Finding: "This is a policy or procedure, which a diagram can't show.", ByRuntime: true})
	}
	slices.SortStableFunc(rep.Items, func(x, y GapItem) int { return strings.Compare(x.Record, y.Record) })
	if g, err := a.versionGraph(gs.Version); err == nil {
		for i := range rep.Items {
			rep.Items[i].Finding = a.nameAliasesLocked(rep.Items[i].Finding, g, nil)
		}
		for i := range rep.Recommendations {
			rep.Recommendations[i].Finding = a.nameAliasesLocked(rep.Recommendations[i].Finding, g, nil)
		}
	}
	data, _ := json.Marshal(rep)
	ref, err := a.P.SaveReport(gs.Version, data)
	if err != nil {
		return "", err
	}
	a.markSubmittedLocked("submit_gap_report")
	a.gap = nil
	a.emit("gapreport", ref)
	return fmt.Sprintf("Gap Report saved for v%d (%d items, %d recommendations).", gs.Version, len(rep.Items), len(rep.Recommendations)), nil
}

// ---- Fix / Fix all

// Fix drafts one Proposal that fixes the chosen unmet items of a report.
func (a *App) Fix(reportFile string, records []string) error {
	raw, err := a.P.Report(reportFile)
	if err != nil {
		return err
	}
	var rep GapReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return err
	}
	var items []GapItem
	for _, it := range rep.Items {
		if it.Status == "unmet" && slices.Contains(records, it.Record) {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		return errors.New("no unmet items selected")
	}
	if p := a.PendingProposal(); p != nil {
		return fmt.Errorf("Proposal %s is waiting for review; decide on it first", p.ID)
	}
	return a.run(&taskSpec{kind: "fix", expect: "submit_proposal", build: func() (string, error) {
		ctx, err := a.contextLocked(ViewProduction, nil)
		if err != nil {
			return "", err
		}
		var sb strings.Builder
		sb.WriteString("TASK: Fix Gap Report items.\n\n" + ctx + "\n\nDraft ONE Proposal on the Production Diagram that fixes these unmet items:\n")
		var addr []string
		for _, it := range items {
			var als []string
			for _, id := range it.Nodes {
				als = append(als, a.sess.Aliases.Of(id, false))
			}
			fmt.Fprintf(&sb, "- %s (%s): %s Nodes: %s. Suggested practices: %s\n", it.Record, it.Title, it.Finding, strings.Join(als, " "), strings.Join(it.Fix, ", "))
			addr = append(addr, "gap:"+it.Record)
		}
		latest, _ := a.P.Latest()
		fmt.Fprintf(&sb, "Use get_record for a practice's details if needed. Submit with submit_proposal, \"view\": \"production\", \"base_version\": %d, \"addresses\": %s, and practices on the changes that apply them. Then reply with one sentence.", latest, mustJSON(addr))
		return sb.String(), nil
	}})
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
