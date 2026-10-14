package app

import (
	"encoding/json"
	"testing"

	"github.com/vinaayakh/designer/internal/brief"
	"github.com/vinaayakh/designer/internal/catalog"
	"github.com/vinaayakh/designer/internal/proposal"
)

// simApp is an App with one Production version: an API container calling a database.
func simApp(t *testing.T) *App {
	t.Helper()
	a := pipelineApp(t)
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	a.Cat = cat
	b := brief.New()
	b.Answers["cloud"] = "aws"
	if err := a.P.SaveBrief(b); err != nil {
		t.Fatal(err)
	}
	node := func(id, label, kind, service string, attrs map[string]string) map[string]any {
		return map[string]any{"id": id, "type": "rectangle", "x": 0, "y": 0, "width": 200, "height": 72,
			"customData":    map[string]any{"designer": map[string]any{"role": "node", "kind": kind, "service": service, "attrs": attrs}},
			"boundElements": []any{map[string]any{"id": id + "-t", "type": "text"}}}
	}
	label := func(id, text string) map[string]any {
		return map[string]any{"id": id + "-t", "type": "text", "text": text, "containerId": id, "x": 0, "y": 0, "width": 40, "height": 20}
	}
	sc, _ := json.Marshal(map[string]any{"type": "excalidraw", "version": 2, "elements": []any{
		node("api", "API", "compute.container", "aws.ecs-fargate", map[string]string{"tasks": "2-4"}), label("api", "API"),
		node("db", "DB", "data.relational", "aws.rds-postgres", nil), label("db", "DB"),
		map[string]any{"id": "e1", "type": "arrow", "x": 0, "y": 0, "width": 10, "height": 0, "points": [][]float64{{0, 0}, {10, 0}},
			"startBinding": map[string]any{"elementId": "api"}, "endBinding": map[string]any{"elementId": "db"},
			"customData": map[string]any{"designer": map[string]any{"role": "edge", "style": "sync"}}},
	}})
	if _, err := a.P.SaveVersion(sc, "test", "v1", "", nil); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestUserProposalFromSimMode(t *testing.T) {
	a := simApp(t)
	// The browser refers to element IDs, not Aliases.
	p := &proposal.Payload{Title: "Baseline: scale the API", Changes: []*proposal.Change{
		{Op: "update_node", Ref: "api", Set: map[string]any{"instances": "4", "autoscaling": "4-10, target 60% requests"}},
		{Op: "add_node", Ref: "+cache", Label: "Cache", Kind: "data.cache", Service: "aws.elasticache-valkey"},
		{Op: "add_edge", From: "api", To: "+cache", Style: "sync"},
	}}
	pend, err := a.SubmitUserProposal(p, "Scenario Baseline")
	if err != nil {
		t.Fatal(err)
	}
	if pend.Author != "user" || pend.BaseVersion != 1 || pend.Payload.Changes[0].ElementID != "api" {
		t.Fatalf("pending %+v / change %+v", pend, pend.Payload.Changes[0])
	}
	if len(pend.Payload.Changes[2].Needs) != 1 {
		t.Errorf("the edge to the new cache depends on adding it: %+v", pend.Payload.Changes[2])
	}
	// Only one Proposal waits at a time.
	if _, err := a.SubmitUserProposal(&proposal.Payload{Title: "again", Changes: p.Changes}, "x"); err == nil {
		t.Error("a second Proposal was accepted while one is waiting")
	}
	// Nobody to revise it: reply is refused; reject sends the Assistant nothing.
	if _, err := a.Decide(Decision{ID: pend.ID, Revision: 1, Action: "reply", Reply: "smaller"}); err == nil {
		t.Error("reply accepted on a Sim Mode Proposal")
	}
	if _, err := a.Decide(Decision{ID: pend.ID, Revision: 1, Action: "reject"}); err != nil {
		t.Fatal(err)
	}
	if len(a.sess.Feedback) != 0 || a.PendingProposal() != nil {
		t.Errorf("feedback %v, pending %v", a.sess.Feedback, a.PendingProposal())
	}
}

func TestUserProposalRejectsStaleBase(t *testing.T) {
	a := simApp(t)
	p := &proposal.Payload{Title: "x", BaseVersion: 7, Changes: []*proposal.Change{{Op: "update_node", Ref: "api", Set: map[string]any{"instances": "3"}}}}
	if _, err := a.SubmitUserProposal(p, "x"); err == nil {
		t.Fatal("a Scenario built on another version was accepted")
	}
}
