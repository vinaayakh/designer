package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vinaayakh/designer/internal/pipeline"
	"github.com/vinaayakh/designer/internal/project"
	"github.com/vinaayakh/designer/internal/register"
	"github.com/vinaayakh/designer/internal/scene"
)

func pipelineApp(t *testing.T) *App {
	t.Helper()
	p, err := project.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := &App{P: p, sess: newSessionState()}
	if err := a.openRegister(); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAnswersBecomeEntries(t *testing.T) {
	a := pipelineApp(t)
	if err := a.Answer("requirements", "problem", nil, true); err == nil {
		t.Fatal("a blocking question can't be unknown")
	}
	if err := a.Answer("requirements", "roles", []string{"Customer\nAdmin"}, false); err != nil {
		t.Fatal(err)
	}
	ans, _ := a.pipeStore().LoadAnswers()
	ids := ans["requirements"]["roles"].Entries
	if len(ids) != 2 {
		t.Fatalf("two roles → two entries, got %v", ids)
	}
	e, _ := a.Reg.Get(ids[0])
	if e.Type != register.CON || e.Status != register.Confirmed || e.Source.Ref != "requirements·roles" || e.Statement != "Customer" {
		t.Errorf("entry %+v", e)
	}
	// Changing the answer: Customer stays, Admin is replaced by Support agent.
	if err := a.Answer("requirements", "roles", []string{"Customer\nSupport agent"}, false); err != nil {
		t.Fatal(err)
	}
	ans, _ = a.pipeStore().LoadAnswers()
	now := ans["requirements"]["roles"].Entries
	if now[0] != ids[0] {
		t.Errorf("an unchanged line keeps its entry: %v vs %v", now, ids)
	}
	old, _ := a.Reg.Get(ids[1])
	if old.Status != register.Superseded {
		t.Errorf("Admin should be superseded by Support agent, is %s", old.Status)
	}
	// Unknown is remembered without an entry.
	if err := a.Answer("requirements", "metrics", nil, true); err != nil {
		t.Fatal(err)
	}
	ans, _ = a.pipeStore().LoadAnswers()
	if m := ans["requirements"]["metrics"]; !m.Unknown || len(m.Entries) != 0 {
		t.Errorf("metrics %+v", m)
	}
}

func TestSubmitStep(t *testing.T) {
	a := pipelineApp(t)
	a.Answer("requirements", "problem", []string{"Orders get lost at peak"}, false)
	a.Answer("requirements", "roles", []string{"Customer"}, false)
	g := &scene.Graph{Nodes: []*scene.Node{{ID: "api", Label: "Orders API", Kind: "compute.container"}}}
	g.Reindex()
	alias := a.sess.Aliases.Of("api", false)
	s := pipeline.Get("requirements")
	a.turn = &Turn{Kind: "step", Expect: "submit_step"}
	ans, _ := a.pipeStore().LoadAnswers()
	in := pipeline.Inputs{BriefHash: a.briefHash()}
	for _, x := range ans["requirements"] {
		if x.At.After(in.AnswersAt) {
			in.AnswersAt = x.At
		}
	}
	a.step = &stepSpec{Step: s, Graph: g, In: in}
	goal := ""
	for _, e := range a.Reg.Entries() {
		if e.Type == register.REQ && strings.HasPrefix(e.Statement, "Problem statement") {
			goal = e.ID
		}
	}
	sub := map[string]any{
		"step": "requirements",
		"sections": map[string]any{
			"purpose":      map[string]any{"body": "Stop losing orders at peak (" + goal + ")."},
			"scope":        map[string]any{"body": "The " + alias + " and its clients."},
			"roles":        map[string]any{"items": []string{"Customer: places orders"}},
			"journeys":     map[string]any{"items": []string{"Place an order"}},
			"verification": map[string]any{"rows": [][]string{{"+checkout", "test", "load test at 2x peak"}}},
		},
		"entries": []map[string]any{{"ref": "+checkout", "type": "REQ", "statement": "Customers can place an order through " + alias,
			"fields": map[string]string{"kind": "functional", "priority": "must", "acceptance": "an order is stored within 2 s"}, "links": []string{alias}}},
	}
	// First a broken submission: a missing section and a bad entry type.
	bad := map[string]any{"step": "requirements", "sections": map[string]any{"purpose": map[string]any{"body": "x"}}, "entries": []map[string]any{{"ref": "+d", "type": "DEC", "statement": "y"}}}
	raw, _ := json.Marshal(bad)
	if _, err := a.submitStepLocked(raw); err == nil || !strings.Contains(err.Error(), "may only add") || !strings.Contains(err.Error(), "section scope: write it") {
		t.Fatalf("expected all problems at once, got %v", err)
	}
	if n := len(a.Reg.Entries()); n == 0 {
		t.Fatal("register empty")
	}
	before := len(a.Reg.Entries())
	raw, _ = json.Marshal(sub)
	msg, err := a.submitStepLocked(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Reg.Entries()) != before+1 {
		t.Fatalf("one new entry expected")
	}
	v, _ := a.pipeStore().Latest("requirements")
	if v == nil || v.N != 1 || len(v.NewEntries) != 1 {
		t.Fatalf("version %+v (%s)", v, msg)
	}
	newID := v.NewEntries[0]
	e, _ := a.Reg.Get(newID)
	if e.Status != register.Open || e.Links[0] != "api" || !strings.Contains(e.Statement, "Orders API") {
		t.Errorf("new entry %+v (alias named, link resolved, open)", e)
	}
	var ver pipeline.SectionOut
	for _, s := range v.Sections {
		if s.ID == "verification" {
			ver = s
		}
	}
	if ver.Rows[0][0] != newID {
		t.Errorf("the +ref is replaced by the real ID: %v", ver.Rows[0])
	}
	if v.Cites[goal] != register.Confirmed || v.Cites[newID] != register.Open {
		t.Errorf("cites %v", v.Cites)
	}
	view, err := a.Pipeline()
	if err != nil {
		t.Fatal(err)
	}
	if view.Steps[0].Status.Kind != "current" {
		t.Errorf("requirements status %+v", view.Steps[0].Status)
	}
	if view.Steps[1].Status.Kind != "needs" {
		t.Errorf("design doc now needs answers: %+v", view.Steps[1].Status)
	}
	// Rejecting the new requirement makes the document stale.
	a.SetEntryStatus(newID, register.Rejected, "", "")
	view, _ = a.Pipeline()
	if view.Steps[0].Status.Kind != "stale" {
		t.Errorf("after rejecting a cited entry: %+v", view.Steps[0].Status)
	}
}
