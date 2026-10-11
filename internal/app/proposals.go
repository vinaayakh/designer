package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/vinaayakh/designer/internal/proposal"
	"github.com/vinaayakh/designer/internal/scene"
)

// Pending is the one Proposal waiting for review. It lives in the runtime
// (.designer/proposal.json), so it survives a Claude Code crash.
type Pending struct {
	ID          string            `json:"id"`
	Revision    int               `json:"revision"`
	Kind        string            `json:"kind"` // proposal | conversion
	View        string            `json:"view"`
	BaseVersion int               `json:"baseVersion"`
	Payload     *proposal.Payload `json:"payload"`
	Created     time.Time         `json:"created"`
	Awaiting    bool              `json:"awaitingRevision,omitempty"` // the user replied; a new revision is coming
	Reply       string            `json:"reply,omitempty"`

	// Step-by-step review: changes already accepted (carried into the next
	// revision when the AI keeps them unchanged), the changes the user
	// rejected along the way, and a clarifying question from the AI.
	Accepted   []string    `json:"accepted,omitempty"`
	Rejections []Rejection `json:"rejections,omitempty"`
	Question   string      `json:"question,omitempty"`
	Answers    []string    `json:"answers,omitempty"`
}

// Rejection is one change the user turned down while stepping through.
type Rejection struct {
	Change *proposal.Change `json:"change"`
	Reason string           `json:"reason,omitempty"`
}

const pendingFile = "proposal.json"

func (a *App) pendingLocked() *Pending {
	var p Pending
	if err := a.P.ReadState(pendingFile, &p); err != nil || p.ID == "" {
		return nil
	}
	return &p
}

// PendingProposal returns the Proposal waiting for review, if any.
func (a *App) PendingProposal() *Pending {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.pendingLocked()
}

func (a *App) nextProposalID() string {
	var seq struct{ N int }
	_ = a.P.ReadState("proposal-seq.json", &seq)
	seq.N++
	_ = a.P.WriteState("proposal-seq.json", seq)
	return fmt.Sprintf("p%d", seq.N)
}

func (a *App) proposalContext(view string) (*proposal.Context, int, error) {
	b, err := a.Brief()
	if err != nil {
		return nil, 0, err
	}
	g, ver, err := a.Graph(view)
	if err != nil {
		return nil, 0, err
	}
	g.Compactless(a.sess.Aliases)
	return &proposal.Context{
		Graph: g, Aliases: a.sess.Aliases, Catalog: a.Cat, Cloud: b.Get("cloud"),
		RecordExists: func(id string) bool { return a.KB.Get(id) != nil },
		TrustedHost:  TrustedHost,
	}, ver, nil
}

// submitProposalLocked handles the submit_proposal tool.
func (a *App) submitProposalLocked(raw json.RawMessage) (string, error) {
	p, err := proposal.Decode(raw)
	if err != nil {
		return "", err
	}
	meta, _ := a.P.Meta()
	latest, _ := a.P.Latest()
	cur := a.pendingLocked()

	kind := "proposal"
	if p.Kind == "conversion" {
		kind = "conversion"
		if latest > 0 {
			return "", errors.New("Conversion already happened; every change now goes through a normal Proposal on the Production Diagram")
		}
		if a.turn == nil || (a.turn.Kind != "design" && !(a.turn.Kind == "reply" && cur != nil && cur.Kind == "conversion")) {
			return "", errors.New("a conversion can only be submitted during the Conversion Design task")
		}
	} else if a.turn != nil && a.turn.Kind == "design" {
		return "", errors.New(`the Conversion Design task must be submitted with "kind": "conversion"`)
	}

	switch {
	case kind == "conversion":
		p.View = ViewProduction
	case p.View == "":
		p.View = a.DefaultView()
	}
	if p.View != ViewRough && p.View != ViewProduction {
		return "", fmt.Errorf("view must be rough or production")
	}
	if kind == "proposal" {
		if p.View == ViewRough && meta.FrozenAt != nil {
			return "", errors.New("the Rough Draft is frozen; propose changes to the Production Diagram instead")
		}
		if p.View == ViewProduction {
			if latest == 0 {
				return "", errors.New("there is no Production Diagram yet")
			}
			if p.BaseVersion != 0 && p.BaseVersion != latest {
				return "", fmt.Errorf("base_version %d is out of date: the current Production version is %d (look at the changes and resubmit)", p.BaseVersion, latest)
			}
		}
	}
	if cur != nil && !cur.Awaiting {
		return "", fmt.Errorf("Proposal %s is already waiting for the user's review; only one can wait at a time", cur.ID)
	}

	sourceView := p.View
	if kind == "conversion" {
		sourceView = ViewRough
	}
	ctx, ver, err := a.proposalContext(sourceView)
	if err != nil {
		return "", err
	}
	if err := proposal.Prepare(p, ctx); err != nil {
		return "", err
	}
	if p.View == ViewProduction && kind == "proposal" {
		p.BaseVersion = ver
	}

	pend := &Pending{Kind: kind, View: p.View, BaseVersion: p.BaseVersion, Payload: p, Created: time.Now().UTC()}
	if cur != nil && cur.Awaiting {
		pend.ID, pend.Revision = cur.ID, cur.Revision+1
		pend.Rejections, pend.Answers = cur.Rejections, cur.Answers
		// Accepted changes stay accepted when the AI kept them unchanged.
		for _, id := range cur.Accepted {
			old, now := findChange(cur.Payload.Changes, id), findChange(p.Changes, id)
			if old != nil && now != nil && proposal.Same(old, now) {
				pend.Accepted = append(pend.Accepted, id)
			}
		}
	} else {
		pend.ID, pend.Revision = a.nextProposalID(), 1
	}
	if err := a.P.WriteState(pendingFile, pend); err != nil {
		return "", err
	}
	a.markSubmittedLocked("submit_proposal")
	a.emit("proposal", pend)
	if kind == "conversion" {
		return fmt.Sprintf("Conversion %s (revision %d) is waiting for the user's review in the Conversion Preview.", pend.ID, pend.Revision), nil
	}
	return fmt.Sprintf("Proposal %s (revision %d, %d changes) is waiting for the user's review.", pend.ID, pend.Revision, len(p.Changes)), nil
}

// Decision is what the browser sends after review.
type Decision struct {
	ID       string          `json:"id"`
	Revision int             `json:"revision"`
	Action   string          `json:"action"` // accept | reject | reply | reject_change | clarify
	Change   string          `json:"change"` // reject_change: the change turned down
	Accepted []string        `json:"accepted"`
	Scene    json.RawMessage `json:"scene"` // the diagram with accepted changes drawn (accept only)
	Reply    string          `json:"reply"`
}

type DecisionResult struct {
	Version int `json:"version,omitempty"`
}

// Decide applies the user's review of the pending Proposal.
func (a *App) Decide(d Decision) (DecisionResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var res DecisionResult
	p := a.pendingLocked()
	if p == nil || p.ID != d.ID || p.Revision != d.Revision {
		return res, errors.New("that Proposal is no longer waiting for review")
	}
	tag := fmt.Sprintf(`"proposal":%q,"revision":%d`, p.ID, p.Revision)
	switch d.Action {
	case "reject":
		_ = a.P.RemoveState(pendingFile)
		a.queueFeedbackLocked(fmt.Sprintf(`{%s,"status":"rejected"}`, tag))
		a.emit("proposal.closed", map[string]any{"id": p.ID, "status": "rejected"})
		return res, nil

	case "reply":
		reply := strings.TrimSpace(d.Reply)
		if reply == "" {
			return res, errors.New("write what you'd like changed")
		}
		p.Awaiting, p.Reply = true, reply
		if p.Kind == "proposal" && d.Accepted != nil {
			ids := map[string]bool{}
			for _, id := range d.Accepted {
				ids[id] = true
			}
			acc := proposal.Closure(p.Payload.Changes, ids)
			p.Accepted = nil
			for _, c := range p.Payload.Changes {
				if acc[c.ID] {
					p.Accepted = append(p.Accepted, c.ID)
				}
			}
		}
		if err := a.P.WriteState(pendingFile, p); err != nil {
			return res, err
		}
		a.emit("proposal", p)
		view := p.View
		if p.Kind == "conversion" {
			view = ViewRough
		}
		spec := &taskSpec{kind: "reply", expect: "submit_proposal", build: func() (string, error) {
			ctx, err := a.contextLocked(view, nil)
			if err != nil {
				return "", err
			}
			what := "Proposal"
			if p.Kind == "conversion" {
				what = `Conversion Design (resubmit the whole design with "kind": "conversion" and its rationale)`
			}
			return fmt.Sprintf("%s\n\n{%s,\"status\":\"changes_requested\"}\nThe user replied to %s %s:\n%s\n\nSubmit a new revision with submit_proposal.", ctx, tag, what, p.ID, reply), nil
		}}
		if p.Kind == "conversion" {
			spec.model = a.settings.ConversionModel
		}
		a.mu.Unlock()
		err := a.run(spec)
		a.mu.Lock()
		return res, err

	case "reject_change", "clarify":
		if p.Kind != "proposal" {
			return res, errors.New("a Conversion is reviewed as a whole")
		}
		if d.Action == "reject_change" {
			ch := findChange(p.Payload.Changes, d.Change)
			if ch == nil {
				return res, fmt.Errorf("Proposal %s has no change %q", p.ID, d.Change)
			}
			ids := map[string]bool{}
			for _, id := range d.Accepted {
				if id != d.Change {
					ids[id] = true
				}
			}
			acc := proposal.Closure(p.Payload.Changes, ids)
			p.Accepted = nil
			for _, c := range p.Payload.Changes {
				if acc[c.ID] {
					p.Accepted = append(p.Accepted, c.ID)
				}
			}
			p.Rejections = append(p.Rejections, Rejection{Change: ch, Reason: strings.TrimSpace(d.Reply)})
		} else {
			answer := strings.TrimSpace(d.Reply)
			if p.Question == "" || answer == "" {
				return res, errors.New("there is no question to answer")
			}
			p.Answers = append(p.Answers, fmt.Sprintf("Q: %s\nA: %s", p.Question, answer))
		}
		p.Awaiting, p.Question = true, ""
		if err := a.P.WriteState(pendingFile, p); err != nil {
			return res, err
		}
		a.emit("proposal", p)
		snapshot := *p
		spec := &taskSpec{kind: "revise", build: func() (string, error) { return a.reviseMessageLocked(&snapshot) }}
		a.mu.Unlock()
		err := a.run(spec)
		a.mu.Lock()
		if err != nil {
			p.Awaiting = false
			_ = a.P.WriteState(pendingFile, p)
			a.emit("proposal", p)
		}
		return res, err

	case "accept":
		if len(d.Scene) == 0 || !json.Valid(d.Scene) {
			return res, errors.New("accept needs the updated scene")
		}
		if p.Kind == "conversion" {
			return a.acceptConversionLocked(p, d)
		}
		ids := map[string]bool{}
		for _, id := range d.Accepted {
			ids[id] = true
		}
		acc := proposal.Closure(p.Payload.Changes, ids)
		var applied, declined []string
		for _, ch := range p.Payload.Changes {
			if acc[ch.ID] {
				applied = append(applied, ch.ID)
			} else {
				declined = append(declined, ch.ID)
			}
		}
		if len(applied) == 0 {
			return res, errors.New("nothing was accepted (use Reject instead)")
		}
		if p.View == ViewProduction {
			latest, _ := a.P.Latest()
			if latest != p.BaseVersion {
				return res, fmt.Errorf("the diagram moved on to v%d since this Proposal was made", latest)
			}
			if _, err := scene.Parse(d.Scene, true); err != nil {
				return res, err
			}
			rat := a.updatedRationaleLocked(latest, p.Payload.Changes, acc)
			v, err := a.P.SaveVersion(d.Scene, "proposal", p.Payload.Title, p.ID, rat)
			if err != nil {
				return res, err
			}
			res.Version = v.N
		} else {
			if err := a.P.SaveRoughDraft(d.Scene); err != nil {
				return res, err
			}
		}
		_ = a.P.RemoveState(pendingFile)
		status := "accepted"
		if len(declined) > 0 {
			status = "partially_accepted"
		}
		fb, _ := json.Marshal(map[string]any{"applied": applied, "declined": declined})
		extra := strings.TrimSuffix(strings.TrimPrefix(string(fb), "{"), "}")
		if res.Version > 0 {
			extra += fmt.Sprintf(`,"new_version":%d`, res.Version)
		}
		a.queueFeedbackLocked(fmt.Sprintf(`{%s,"status":%q,%s}`, tag, status, extra))
		a.emit("proposal.closed", map[string]any{"id": p.ID, "status": status, "version": res.Version, "addresses": p.Payload.Addresses})
		return res, nil
	}
	return res, fmt.Errorf("unknown action %q", d.Action)
}

// updatedRationaleLocked keeps the Design Rationale current: added Nodes get
// their why and practices, removed Nodes lose their entry.
func (a *App) updatedRationaleLocked(version int, changes []*proposal.Change, acc map[string]bool) json.RawMessage {
	v, _, err := a.P.Version(version)
	if err != nil {
		return nil
	}
	var r proposal.Rationale
	if len(v.Rationale) > 0 {
		_ = json.Unmarshal(v.Rationale, &r)
	}
	if r.Nodes == nil {
		r.Nodes = map[string]*proposal.NodeRationale{}
	}
	for _, ch := range changes {
		if !acc[ch.ID] {
			continue
		}
		switch ch.Op {
		case "add_node":
			r.Nodes[ch.ElementID] = &proposal.NodeRationale{Why: ch.Why, Practices: ch.Practices}
		case "update_node":
			if nr := r.Nodes[ch.ElementID]; nr != nil {
				for _, pr := range ch.Practices {
					if !slices.Contains(nr.Practices, pr) {
						nr.Practices = append(nr.Practices, pr)
					}
				}
			} else if ch.Why != "" || len(ch.Practices) > 0 {
				r.Nodes[ch.ElementID] = &proposal.NodeRationale{Why: ch.Why, Practices: ch.Practices}
			}
		case "remove_node":
			delete(r.Nodes, ch.ElementID)
		}
	}
	b, _ := json.Marshal(r)
	return b
}

func (a *App) acceptConversionLocked(p *Pending, d Decision) (DecisionResult, error) {
	var res DecisionResult
	if latest, _ := a.P.Latest(); latest > 0 {
		return res, errors.New("Conversion already happened")
	}
	g, err := scene.Parse(d.Scene, true)
	if err != nil {
		return res, err
	}
	if len(g.Nodes) == 0 {
		return res, errors.New("the converted diagram is empty")
	}
	rat, _ := json.Marshal(p.Payload.Rationale)
	v, err := a.P.SaveVersion(d.Scene, "conversion", p.Payload.Title, p.ID, rat)
	if err != nil {
		return res, err
	}
	if err := a.P.Freeze(); err != nil {
		return res, err
	}
	_ = a.P.RemoveState(pendingFile)
	res.Version = v.N
	a.queueFeedbackLocked(fmt.Sprintf(`{"proposal":%q,"revision":%d,"status":"accepted","new_version":%d} (Conversion accepted: Production v%d exists and the Rough Draft is frozen)`, p.ID, p.Revision, v.N, v.N))
	a.emit("proposal.closed", map[string]any{"id": p.ID, "status": "accepted", "version": v.N, "conversion": true})
	// The Gap Check runs automatically on the accepted version (ticket 011).
	go func() {
		if err := a.GapCheck(v.N); err != nil {
			a.emit("assistant.notice", map[string]any{"text": "Couldn't start the Gap Check: " + err.Error()})
		}
	}()
	return res, nil
}

// SimulatePending returns the meaning of the active diagram before and after
// the pending Proposal (all changes, or only the accepted ones after
// dependencies). The browser draws the result.
func (a *App) SimulatePending(accepted []string) (base, result *scene.Graph, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.pendingLocked()
	if p == nil {
		return nil, nil, errors.New("no Proposal is waiting for review")
	}
	view := p.View
	if p.Kind == "conversion" {
		view = ViewRough
	}
	base, _, err = a.Graph(view)
	if err != nil {
		return nil, nil, err
	}
	var acc map[string]bool
	if accepted != nil {
		ids := map[string]bool{}
		for _, id := range accepted {
			ids[id] = true
		}
		acc = proposal.Closure(p.Payload.Changes, ids)
	}
	result, err = proposal.Simulate(base, p.Payload.Changes, acc)
	return base, result, err
}

func findChange(cs []*proposal.Change, id string) *proposal.Change {
	for _, c := range cs {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// aiChange is a change as the AI wrote it (without what the runtime filled in).
func aiChange(c *proposal.Change) *proposal.Change {
	x := *c
	x.ElementID, x.ParentID, x.FromID, x.ToID, x.Needs = "", nil, "", "", nil
	return &x
}

// reviseMessageLocked asks the AI to rework the rest of a Proposal after the
// user rejected a change, or answered its clarifying question.
func (a *App) reviseMessageLocked(p *Pending) (string, error) {
	view := p.View
	ctx, err := a.contextLocked(view, nil)
	if err != nil {
		return "", err
	}
	accepted := map[string]bool{}
	for _, id := range p.Accepted {
		accepted[id] = true
	}
	rejected := map[string]bool{}
	for _, r := range p.Rejections {
		rejected[r.Change.ID] = true
	}
	var keep, rest []*proposal.Change
	for _, c := range p.Payload.Changes {
		switch {
		case accepted[c.ID]:
			keep = append(keep, aiChange(c))
		case !rejected[c.ID]:
			rest = append(rest, aiChange(c))
		}
	}
	var rej []map[string]any
	for _, r := range p.Rejections {
		rej = append(rej, map[string]any{"change": aiChange(r.Change), "reason": r.Reason})
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "TASK: Revise Proposal %s (%q). The user is stepping through it one change at a time.\n\n%s\n\n", p.ID, p.Payload.Title, ctx)
	fmt.Fprintf(&sb, "Accepted so far — resubmit these exactly as they are, with the same ids:\n%s\n\n", mustJSON(keep))
	fmt.Fprintf(&sb, "Rejected by the user:\n%s\n\n", mustJSON(rej))
	if len(p.Answers) > 0 {
		fmt.Fprintf(&sb, "The user's answers to your questions:\n%s\n\n", strings.Join(p.Answers, "\n"))
	}
	fmt.Fprintf(&sb, "Not reviewed yet:\n%s\n\n", mustJSON(rest))
	sb.WriteString(`Rework the changes not reviewed yet in light of the rejection: drop or replace any that relied on what was rejected, and keep the rest. Do not re-propose a rejected change in the same form.
- If you need to know more about why the user rejected it before you can revise well, do NOT submit: reply with one short, specific question and stop. The user will answer.
- Otherwise call submit_proposal once: the same title (adjusted if needed), the accepted changes unchanged, then the revised remaining changes with new ids for new ones. Then reply with one sentence on what you changed.
- If nothing else makes sense without the rejected change, submit just the accepted changes; if none were accepted either, reply saying the Proposal should be rejected.`)
	return sb.String(), nil
}
