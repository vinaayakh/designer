package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/vinaayakh/designer/internal/pipeline"
	"github.com/vinaayakh/designer/internal/register"
	"github.com/vinaayakh/designer/internal/scene"
)

// ---- Document Pipeline (wayfinder/doc-pipeline): the app side of internal/pipeline

func (a *App) pipeStore() pipeline.Store { return pipeline.Store{Dir: a.P.Dir} }

// stepSpec is the Step task in progress.
type stepSpec struct {
	Step  *pipeline.Step
	Graph *scene.Graph
	In    pipeline.Inputs
}

// StepView is one Step as the Documents tab shows it.
type StepView struct {
	*pipeline.Step
	Status   pipeline.Status            `json:"status"`
	Answers  map[string]pipeline.Answer `json:"answers"`
	Versions []int                      `json:"versions"`
	Latest   *pipeline.Version          `json:"latest,omitempty"`
}

// EntryView is a Register entry with the documents that cite it.
type EntryView struct {
	register.Entry
	CitedBy []string `json:"citedBy,omitempty"` // "Requirements spec v2"
}

// Arc42View is the arc42 document's state: it reads the same Register, and its text is written
// per Production version.
type Arc42View struct {
	Version int  `json:"version"`
	Written bool `json:"written"`
	Stale   bool `json:"stale"`
}

// PipelineView is everything the Documents tab needs.
type PipelineView struct {
	Steps      []StepView       `json:"steps"`
	Entries    []EntryView      `json:"entries"`
	Proposed   []*ProposedEntry `json:"proposed,omitempty"` // from chat, waiting for review
	Arc42      *Arc42View       `json:"arc42,omitempty"`    // the arc42 export of the latest Production version
	Integrity  string           `json:"integrity,omitempty"`
	Queue      []string         `json:"queue,omitempty"` // documents waiting to be written, in order
	Production int              `json:"production"`      // latest Production version (0: none yet)
}

func (a *App) briefHash() string {
	b, err := a.Brief()
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b.Marshal())
	return hex.EncodeToString(h[:8])
}

func (a *App) pipelineNow() (pipeline.Now, error) {
	st := a.pipeStore()
	ans, err := st.LoadAnswers()
	if err != nil {
		return pipeline.Now{}, err
	}
	latest, _ := a.P.Latest()
	now := pipeline.Now{BriefHash: a.briefHash(), Production: latest, Latest: map[string]*pipeline.Version{}, Entries: map[string]register.Entry{}, Answers: ans}
	for _, s := range pipeline.Steps() {
		v, err := st.Latest(s.ID)
		if err != nil {
			return now, err
		}
		if v != nil {
			now.Latest[s.ID] = v
		}
	}
	for _, e := range a.Reg.Entries() {
		now.Entries[e.ID] = e
	}
	return now, nil
}

// Pipeline returns every Step's status, answers and versions, and the Register.
func (a *App) Pipeline() (PipelineView, error) {
	now, err := a.pipelineNow()
	if err != nil {
		return PipelineView{}, err
	}
	a.mu.Lock()
	writing := ""
	if a.turn != nil && a.turn.Kind == "step" && a.step != nil {
		writing = a.step.Step.ID
	}
	proposed := a.proposedLocked().Entries
	queue := slices.Clone(a.docQueue)
	a.mu.Unlock()
	view := PipelineView{Integrity: a.Reg.Integrity(), Proposed: proposed, Queue: queue, Production: now.Production}
	citedBy := map[string][]string{}
	for _, s := range pipeline.Steps() {
		sv := StepView{Step: s, Status: pipeline.StatusOf(s, now), Answers: now.Answers[s.ID], Latest: now.Latest[s.ID]}
		if writing == s.ID {
			sv.Status = pipeline.Status{Kind: "writing", Text: "The Assistant is writing it…"}
		} else if i := slices.Index(queue, s.ID); i >= 0 {
			after := writing
			if i > 0 {
				after = queue[i-1]
			}
			sv.Status = pipeline.Status{Kind: "queued", Text: "Queued"}
			if p := pipeline.Get(after); p != nil {
				sv.Status.Text = "Queued after the " + p.Title
			}
		}
		sv.Versions, _ = a.pipeStore().Versions(s.ID)
		if v := sv.Latest; v != nil {
			for id := range v.Cites {
				citedBy[id] = append(citedBy[id], fmt.Sprintf("%s v%d", s.Title, v.N))
			}
		}
		view.Steps = append(view.Steps, sv)
	}
	for _, e := range a.Reg.Entries() {
		view.Entries = append(view.Entries, EntryView{Entry: e, CitedBy: citedBy[e.ID]})
	}
	if now.Production > 0 {
		written, stale, _ := a.DocumentStatus(now.Production)
		view.Arc42 = &Arc42View{Version: now.Production, Written: written, Stale: stale}
	}
	return view, nil
}

// Answer saves the user's answer to a wizard question. Each answer is a confirmed Register entry
// (one per line or value); changing the answer replaces the entries it made before. unknown is
// remembered, and the Assistant states what it assumes when it writes the document.
func (a *App) Answer(stepID, qid string, values []string, unknown bool) error {
	// One at a time: each answer reads, changes and writes answers.json and the Register.
	a.answerMu.Lock()
	defer a.answerMu.Unlock()
	s := pipeline.Get(stepID)
	if s == nil {
		return fmt.Errorf("no Step %q", stepID)
	}
	q := s.Question(qid)
	if q == nil {
		return fmt.Errorf("%s has no question %q", stepID, qid)
	}
	if unknown && q.Blocking {
		return fmt.Errorf("%q is needed to write the %s", q.Label, s.Title)
	}
	var clean []string
	if !unknown {
		var err error
		if clean, err = q.Clean(values); err != nil {
			return err
		}
	}
	st := a.pipeStore()
	ans, err := st.LoadAnswers()
	if err != nil {
		return err
	}
	prev := ans[stepID][qid]
	ref := stepID + "·" + qid
	var made []string
	if q.Creates != "" {
		// What stays: entries whose text is unchanged.
		keep := map[string]string{}
		for _, id := range prev.Entries {
			if e, ok := a.Reg.Get(id); ok && e.Live() {
				keep[e.Statement] = id
			}
		}
		// A list or multi-choice answer is one entry per value, worded as the value itself (the
		// entry's source names the question); a single answer reads "Label: value".
		statement := func(v string) string {
			if q.Kind == "list" || q.Kind == "multi" {
				return q.Prefix + v
			}
			return q.Label + ": " + v
		}
		fields := func(v string) map[string]string {
			if q.ValueField == "" {
				return q.Fields
			}
			f := map[string]string{q.ValueField: v}
			for k, x := range q.Fields {
				f[k] = x
			}
			return f
		}
		var fresh []string
		for _, v := range clean {
			if id, ok := keep[statement(v)]; ok {
				made = append(made, id)
				delete(keep, statement(v))
			} else {
				fresh = append(fresh, v)
			}
		}
		src := register.Source{Kind: "wizard", Ref: ref}
		olds := mapValues(keep)
		for i, v := range fresh {
			n := register.New{Type: q.Creates, Statement: statement(v), Fields: fields(v), Source: src, Status: register.Confirmed}
			var e register.Entry
			if i < len(olds) { // a changed value replaces an old one
				e, err = a.Reg.Supersede(register.ActorUser, stepID, "", olds[i], n, "the answer changed")
			} else {
				e, err = a.Reg.Create(register.ActorUser, stepID, "", n, "")
			}
			if err != nil {
				return err
			}
			made = append(made, e.ID)
		}
		for _, id := range olds[min(len(olds), len(fresh)):] {
			if err := a.Reg.SetStatus(register.ActorUser, id, register.Rejected, "removed from the answer"); err != nil {
				return err
			}
		}
	}
	if ans[stepID] == nil {
		ans[stepID] = map[string]pipeline.Answer{}
	}
	ans[stepID][qid] = pipeline.Answer{Values: clean, Unknown: unknown, At: time.Now().UTC(), Entries: made}
	if err := st.SaveAnswers(ans); err != nil {
		return err
	}
	slog.Info("pipeline: answered", "step", stepID, "question", qid, "unknown", unknown, "entries", made)
	a.emit("pipeline", nil)
	return nil
}

func mapValues(m map[string]string) []string {
	var out []string
	for _, k := range sortedKeys(m) {
		out = append(out, m[k])
	}
	return out
}

// SetEntryStatus confirms or rejects a Register entry for the user. Rejecting with a replacement
// supersedes it with the replacement, confirmed.
func (a *App) SetEntryStatus(id string, status register.Status, reason, replacement string) error {
	e, ok := a.Reg.Get(id)
	if !ok {
		return fmt.Errorf("no entry %s", id)
	}
	var err error
	if status == register.Rejected && strings.TrimSpace(replacement) != "" {
		_, err = a.Reg.Supersede(register.ActorUser, e.Step, "", id, register.New{Type: e.Type, Statement: strings.TrimSpace(replacement), Fields: e.Fields,
			Source: register.Source{Kind: "user", Ref: "replaces " + id}, Links: e.Links, Status: register.Confirmed}, orDefault(reason, "rejected and replaced by the user"))
	} else {
		err = a.Reg.SetStatus(register.ActorUser, id, status, reason)
	}
	if err == nil {
		a.emit("pipeline", nil)
	}
	return err
}

func orDefault(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}

// RunStep starts the Assistant task that writes one Step's document.
func (a *App) RunStep(stepID string) error {
	s := pipeline.Get(stepID)
	if s == nil {
		return fmt.Errorf("no Step %q", stepID)
	}
	now, err := a.pipelineNow()
	if err != nil {
		return err
	}
	switch st := pipeline.StatusOf(s, now); {
	case st.Kind == "waiting":
		return errors.New(st.Text)
	case len(st.Blocking) > 0:
		return fmt.Errorf("answer %s first", strings.Join(st.Blocking, " and "))
	}
	if s.Diagram == "production" && now.Production == 0 {
		return errors.New("there is no Production diagram yet")
	}
	a.mu.Lock()
	model := a.settings.ConversionModel
	a.mu.Unlock()
	return a.run(&taskSpec{kind: "step", expect: "submit_step", model: model, build: func() (string, error) { return a.stepMessageLocked(s) }})
}

func (a *App) stepGraph(s *pipeline.Step) (*scene.Graph, int, error) {
	if s.Diagram == "rough" {
		g, _, err := a.Graph(ViewRough)
		return g, 0, err
	}
	n, err := a.P.Latest()
	if err != nil {
		return nil, 0, err
	}
	g, err := a.versionGraph(n)
	return g, n, err
}

func (a *App) stepMessageLocked(s *pipeline.Step) (string, error) {
	b, err := a.Brief()
	if err != nil {
		return "", err
	}
	g, prod, err := a.stepGraph(s)
	if err != nil {
		return "", err
	}
	now, err := a.pipelineNow()
	if err != nil {
		return "", err
	}
	in := pipeline.Inputs{BriefHash: now.BriefHash, Production: prod, Upstream: map[string]int{}}
	for _, up := range append(slices.Clone(s.Requires), s.Uses...) {
		if v := now.Latest[up]; v != nil {
			in.Upstream[up] = v.N
		}
	}
	for _, an := range now.Answers[s.ID] {
		if an.At.After(in.AnswersAt) {
			in.AnswersAt = an.At
		}
	}
	a.step = &stepSpec{Step: s, Graph: g, In: in}
	al := a.sess.Aliases

	var sb strings.Builder
	fmt.Fprintf(&sb, "TASK: Write the %s (Document Pipeline step %q).\n\n%s\n\n", s.Title, s.ID, b.Compact())
	if prod > 0 {
		fmt.Fprintf(&sb, "Production diagram v%d:\n%s\n\n", prod, strings.TrimRight(g.Compact(al), "\n"))
	} else {
		fmt.Fprintf(&sb, "Rough Draft:\n%s\n\n", strings.TrimRight(g.Compact(al), "\n"))
	}
	sb.WriteString("Register (live entries; cite them by ID; open = not yet confirmed by the user):\n")
	for _, e := range a.Reg.Entries() {
		if !e.Live() {
			continue
		}
		fmt.Fprintf(&sb, "- %s [%s] %s", e.ID, e.Status, e.Statement)
		if len(e.Fields) > 0 {
			var fs []string
			for _, k := range sortedKeys(e.Fields) {
				fs = append(fs, k+"="+e.Fields[k])
			}
			fmt.Fprintf(&sb, " (%s)", strings.Join(fs, ", "))
		}
		sb.WriteString("\n")
	}
	if len(s.Questions) > 0 {
		sb.WriteString("\nThe user's answers for this document:\n")
		for _, q := range s.Questions {
			an, ok := now.Answers[s.ID][q.ID]
			switch {
			case !ok:
				fmt.Fprintf(&sb, "- %s: not answered. If the document needs it, create an ASM entry stating what you assume.\n", q.Label)
			case an.Unknown:
				fmt.Fprintf(&sb, "- %s: unknown. Create an ASM entry stating what you assume, and use it.\n", q.Label)
			default:
				fmt.Fprintf(&sb, "- %s: %s (entries %s)\n", q.Label, strings.Join(an.Values, "; "), strings.Join(an.Entries, ", "))
			}
		}
	}
	for _, up := range append(slices.Clone(s.Requires), s.Uses...) {
		if v := now.Latest[up]; v != nil {
			fmt.Fprintf(&sb, "\nUpstream document (summary of v%d):\n%s", v.N, v.Summary)
		}
	}
	din := pipeline.Input{Graph: g, Brief: b}
	for _, d := range s.Derive {
		fmt.Fprintf(&sb, "\nComputed (%s):\n%s", d, pipeline.Derive(d, din).Prompt)
	}
	for _, sec := range s.Sections {
		if sec.Kind == "derived" {
			fmt.Fprintf(&sb, "\nComputed for section %q (already in the document):\n%s", sec.ID, pipeline.Derive(sec.Deriver, din).Prompt)
		}
	}
	fmt.Fprintf(&sb, "\n%s\nSections you write:\n", strings.TrimSpace(s.Instructions))
	var filled []string
	for _, sec := range s.Sections {
		switch sec.Kind {
		case "prose":
			fmt.Fprintf(&sb, "- %s (%s): body. %s\n", sec.ID, sec.Title, sec.Guidance)
		case "list":
			fmt.Fprintf(&sb, "- %s (%s): items (list of strings). %s\n", sec.ID, sec.Title, sec.Guidance)
		case "table":
			fmt.Fprintf(&sb, "- %s (%s): rows, each %d cells: %s. %s\n", sec.ID, sec.Title, len(sec.Columns), strings.Join(sec.Columns, " | "), sec.Guidance)
		case "entries":
			var ts []string
			for _, t := range sec.Types {
				ts = append(ts, string(t))
			}
			filled = append(filled, fmt.Sprintf("%s (all live %s entries)", sec.Title, strings.Join(ts, "/")))
		}
	}
	if len(filled) > 0 {
		fmt.Fprintf(&sb, "The runtime fills these from the Register, so add what belongs there as new entries: %s.\n", strings.Join(filled, "; "))
	}
	var types []string
	for _, t := range s.Creates {
		types = append(types, string(t))
	}
	fmt.Fprintf(&sb, `
Then call submit_step once:
{"step": %q,
 "sections": {"<section id>": {"body": "..."} | {"items": ["..."]} | {"rows": [["..."]]}, ...},   // every section you write
 "entries": [{"ref": "+short-name", "type": one of %s, "statement": "...", "fields": {...}, "links": ["n3"]}],   // new Register entries (they start open: the user confirms them)
 "cites": ["REQ-001", "+short-name", {"record": "practice.x"}]}   // entries the document relies on beyond those named in the text, and Knowledge Base Records it draws on
Cite entries by ID in the text (e.g. "(REQ-004)"); a new entry is cited by its "+ref" until it's numbered. Field values: REQ kind goal/functional/non-functional, priority must/should/could, acceptance, rationale; ASM assumed, if_wrong; NG kind non-goal/out-of-scope, reason; DEC chose, over, because; RISK likelihood/impact low/medium/high, mitigation, owner. Links are diagram aliases.
Inside text, name components by their labels as written on the diagram, never by alias (no n3, no ranges like n20–n25).
Write plainly and specifically for this design. Never change the diagram. If submit_step reports problems, fix them all and submit again. Then reply with one sentence.`, s.ID, strings.Join(types, "/"))
	return sb.String(), nil
}

var plusRef = regexp.MustCompile(`\+[A-Za-z0-9][A-Za-z0-9_-]*`)

// submitStepLocked checks and saves the Assistant's document.
func (a *App) submitStepLocked(raw json.RawMessage) (string, error) {
	sp := a.step
	if a.turn == nil || a.turn.Kind != "step" || sp == nil {
		return "", errors.New("submit_step is only used during a Document Pipeline task")
	}
	var in struct {
		Step     string                      `json:"step"`
		Sections map[string]pipeline.Written `json:"sections"`
		Entries  []struct {
			Ref       string            `json:"ref"`
			Type      register.Type     `json:"type"`
			Statement string            `json:"statement"`
			Fields    map[string]string `json:"fields"`
			Links     []string          `json:"links"`
		} `json:"entries"`
		Cites []json.RawMessage `json:"cites"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("submission: %w", err)
	}
	s := sp.Step
	var problems []string
	// cites holds Register entry IDs (or +refs) and Knowledge Base Records as {"record": id}.
	var cited, records []string
	for _, c := range in.Cites {
		var id string
		var rec struct{ Record string }
		switch {
		case json.Unmarshal(c, &id) == nil:
			cited = append(cited, id)
		case json.Unmarshal(c, &rec) == nil && rec.Record != "":
			if a.KB.Get(rec.Record) == nil {
				problems = append(problems, fmt.Sprintf("cites Knowledge Base Record %q, which doesn't exist (use search_kb)", rec.Record))
			} else if !slices.Contains(records, rec.Record) {
				records = append(records, rec.Record)
			}
		default:
			problems = append(problems, fmt.Sprintf(`cites %s: give an entry ID ("REQ-004" or "+ref") or a Knowledge Base Record ({"record": "practice.x"})`, c))
		}
	}
	if in.Step != s.ID {
		problems = append(problems, fmt.Sprintf("step must be %q", s.ID))
	}
	// New entries: check them all before creating any.
	refs := map[string]int{}
	news := make([]register.New, len(in.Entries))
	for i, e := range in.Entries {
		where := fmt.Sprintf("entries[%d]", i)
		if !strings.HasPrefix(e.Ref, "+") || refs[e.Ref] > 0 {
			problems = append(problems, where+`: needs a unique "ref" starting with + (e.g. "+peak-load")`)
		}
		refs[e.Ref] = i + 1
		if !slices.Contains(s.Creates, e.Type) {
			problems = append(problems, fmt.Sprintf("%s: this document may only add %v entries", where, s.Creates))
		}
		var links []string
		for _, al := range e.Links {
			id, ok := a.sess.Aliases.ID(al)
			if !ok || (sp.Graph.Node(id) == nil && sp.Graph.Group(id) == nil) {
				problems = append(problems, fmt.Sprintf("%s: link %q is not on the diagram", where, al))
				continue
			}
			links = append(links, id)
		}
		news[i] = register.New{Type: e.Type, Statement: a.nameAliasesLocked(strings.TrimSpace(e.Statement), sp.Graph, nil), Fields: e.Fields,
			Source: register.Source{Kind: "assistant", Ref: s.ID}, Links: links}
		if err := register.Validate(news[i]); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", where, err))
		}
	}
	// Check the sections, standing the new entries in for themselves by their +refs.
	entries := a.Reg.Entries()
	stand := slices.Clone(entries)
	for ref := range refs {
		stand = append(stand, register.Entry{ID: ref, Status: register.Open})
	}
	b, _ := a.Brief()
	din := pipeline.Input{Graph: sp.Graph, Brief: b}
	written := withRefsAsIDs(in.Sections)
	_, more := pipeline.Assemble(s, written, din, stand)
	problems = append(problems, more...)
	for _, c := range cited {
		if !slices.ContainsFunc(stand, func(e register.Entry) bool { return e.ID == c }) {
			problems = append(problems, fmt.Sprintf("cites %q, which isn't in the Register or this submission", c))
		}
	}
	if len(problems) > 0 {
		return "", fmt.Errorf("the document was not saved:\n- %s", strings.Join(problems, "\n- "))
	}

	// Create the entries, then put their real IDs in place of the refs.
	ns, _ := a.pipeStore().Versions(s.ID)
	doc := fmt.Sprintf("%s/v%03d", s.ID, len(ns)+1)
	if len(ns) > 0 {
		doc = fmt.Sprintf("%s/v%03d", s.ID, ns[len(ns)-1]+1)
	}
	ids := map[string]string{}
	var made []string
	for i, n := range news {
		e, err := a.Reg.Create(register.ActorAssistant, s.ID, doc, n, "")
		if err != nil {
			return "", err
		}
		ids[in.Entries[i].Ref] = e.ID
		made = append(made, e.ID)
	}
	swap := func(t string) string {
		return plusRef.ReplaceAllStringFunc(t, func(r string) string {
			if id, ok := ids[r]; ok {
				return id
			}
			return r
		})
	}
	final := map[string]pipeline.Written{}
	for id, w := range in.Sections {
		w.Body = a.nameAliasesLocked(swap(w.Body), sp.Graph, nil)
		for i := range w.Items {
			w.Items[i] = a.nameAliasesLocked(swap(w.Items[i]), sp.Graph, nil)
		}
		for _, r := range w.Rows {
			for j := range r {
				r[j] = a.nameAliasesLocked(swap(r[j]), sp.Graph, nil)
			}
		}
		for i := range w.Cites {
			w.Cites[i] = swap(w.Cites[i])
		}
		final[id] = w
	}
	entries = a.Reg.Entries()
	sections, problems := pipeline.Assemble(s, final, din, entries)
	if len(problems) > 0 { // shouldn't happen after the check above
		return "", fmt.Errorf("the document was not saved:\n- %s", strings.Join(problems, "\n- "))
	}
	cites := pipeline.Cited(sections, entries)
	status := map[string]register.Status{}
	for _, e := range entries {
		status[e.ID] = e.Status
	}
	for _, c := range cited {
		cites[swap(c)] = status[swap(c)]
	}
	for _, id := range made {
		cites[id] = status[id]
	}
	v := &pipeline.Version{Step: s.ID, Created: time.Now().UTC().Truncate(time.Second), StepVersion: s.Version, Model: modelOrDefault(a.settings.ConversionModel),
		Inputs: sp.In, Cites: cites, Records: records, NewEntries: made, Summary: pipeline.Summary(s, sections), Sections: sections}
	m, _ := a.P.Meta()
	// Number it first so the Markdown shows the right version.
	vs, _ := a.pipeStore().Versions(s.ID)
	v.N = 1
	if len(vs) > 0 {
		v.N = vs[len(vs)-1] + 1
	}
	if err := a.pipeStore().Save(v, pipeline.Markdown(s, v, m.Name, entries)); err != nil {
		return "", err
	}
	a.markSubmittedLocked("submit_step")
	a.step = nil
	slog.Info("pipeline: document written", "step", s.ID, "version", v.N, "cites", len(cites), "new_entries", len(made))
	a.emit("pipeline", map[string]any{"step": s.ID, "version": v.N})
	return fmt.Sprintf("%s v%d saved: %d sections, %d entries cited, %d new entries (%s), open until the user confirms them.", s.Title, v.N, len(sections), len(cites), len(made), strings.Join(made, ", ")), nil
}

// withRefsAsIDs leaves +refs as they are: during the check they stand in for the new entries.
func withRefsAsIDs(in map[string]pipeline.Written) map[string]pipeline.Written {
	out := map[string]pipeline.Written{}
	for k, w := range in {
		w.Cites = append(slices.Clone(w.Cites), plusRef.FindAllString(w.Body+" "+strings.Join(w.Items, " ")+" "+rowsText(w.Rows), -1)...)
		out[k] = w
	}
	return out
}

func rowsText(rows [][]string) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(strings.Join(r, " ") + " ")
	}
	return b.String()
}

// DocumentVersion reads one written version.
func (a *App) DocumentVersion(stepID string, n int) (*pipeline.Version, string, error) {
	s := pipeline.Get(stepID)
	if s == nil {
		return nil, "", fmt.Errorf("no Step %q", stepID)
	}
	v, err := a.pipeStore().Version(stepID, n)
	if err != nil {
		return nil, "", err
	}
	m, _ := a.P.Meta()
	return v, pipeline.Markdown(s, v, m.Name, a.Reg.Entries()), nil
}

// FixEntry drafts a Proposal that resolves a risk or open question (Draft a fix).
func (a *App) FixEntry(id string) error {
	e, ok := a.Reg.Get(id)
	if !ok {
		return fmt.Errorf("no entry %s", id)
	}
	if e.Type != register.RISK && e.Type != register.Q {
		return errors.New("only risks and open questions can be fixed with a Proposal")
	}
	if p := a.PendingProposal(); p != nil {
		return fmt.Errorf("Proposal %s is waiting for review; decide on it first", p.ID)
	}
	return a.run(&taskSpec{kind: "fix", expect: "submit_proposal", build: func() (string, error) {
		ctx, err := a.contextLocked(ViewProduction, e.Links)
		if err != nil {
			return "", err
		}
		latest, _ := a.P.Latest()
		var als []string
		for _, l := range e.Links {
			als = append(als, a.sess.Aliases.Of(l, false))
		}
		return fmt.Sprintf("TASK: Draft a fix for a Register entry.\n\n%s\n\n%s %s: %s\nInvolves: %s\n\nDraft ONE Proposal on the Production Diagram that resolves it. Submit with submit_proposal, \"view\": \"production\", \"base_version\": %d, \"addresses\": [\"register:%s\"]. Then reply with one sentence.",
			ctx, e.Type, e.ID, e.Statement, orDefault(strings.Join(als, " "), "the whole design"), latest, e.ID), nil
	}})
}
