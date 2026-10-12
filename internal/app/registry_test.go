package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vinaayakh/designer/internal/brief"
	"github.com/vinaayakh/designer/internal/project"
	"github.com/vinaayakh/designer/internal/proposal"
	"github.com/vinaayakh/designer/internal/register"
)

func TestSyncBrief(t *testing.T) {
	p, err := project.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := &App{P: p}
	if err := a.openRegister(); err != nil {
		t.Fatal(err)
	}
	b, _ := a.Brief()
	b.Answers["cloud"], b.Answers["cap"] = "aws", "ap"
	b.Compliance = []string{"soc2"}
	b.Answers["latency"] = "unknown"
	if err := p.SaveBrief(b); err != nil {
		t.Fatal(err)
	}
	if err := a.SyncBrief(); err != nil {
		t.Fatal(err)
	}
	byRef := func() map[string]register.Entry {
		m := map[string]register.Entry{}
		for _, e := range a.Reg.Entries() {
			if e.Live() && e.Source.Kind == "brief" {
				m[e.Source.Ref] = e
			}
		}
		return m
	}
	got := byRef()
	if e := got["cloud"]; e.Type != register.CON || e.Status != register.Confirmed {
		t.Errorf("cloud: %+v", e)
	}
	if e := got["compliance"]; e.Type != register.CON || e.Fields["origin"] != "regulatory" {
		t.Errorf("compliance: %+v", e)
	}
	lat := got["latency"]
	if lat.Type != register.ASM || lat.Status != register.Open {
		t.Errorf("an unknown answer is an open assumption: %+v", lat)
	}
	// Syncing again changes nothing.
	n := len(a.Reg.Entries())
	a.SyncBrief()
	if len(a.Reg.Entries()) != n {
		t.Errorf("a second sync added entries")
	}
	// Answering latency supersedes the assumption with a confirmed requirement.
	b.Answers["latency"] = firstValue(t, "latency")
	p.SaveBrief(b)
	a.SyncBrief()
	now := byRef()["latency"]
	if now.Type != register.REQ || now.Status != register.Confirmed || now.Supersedes != lat.ID {
		t.Errorf("latency after answering: %+v", now)
	}
	old, _ := a.Reg.Get(lat.ID)
	if old.Status != register.Superseded {
		t.Errorf("the assumption should be superseded, is %s", old.Status)
	}
}

func firstValue(t *testing.T, field string) string {
	t.Helper()
	for _, f := range brief.TheSchema().Fields {
		if f.ID == field {
			for _, v := range f.Values {
				if v.ID != "unknown" {
					return v.ID
				}
			}
		}
	}
	t.Fatalf("no value for %s", field)
	return ""
}

func TestConversionSeesRegisterAndFixesResolve(t *testing.T) {
	a := pipelineApp(t)
	req, _ := a.Reg.Create(register.ActorUser, "requirements", "", register.New{Type: register.REQ, Statement: "Customers can track an order", Fields: map[string]string{"kind": "functional"}, Source: register.Source{Kind: "wizard"}, Status: register.Confirmed}, "")
	asm, _ := a.Reg.Create(register.ActorAssistant, "requirements", "", register.New{Type: register.ASM, Statement: "Orders are paid by card", Source: register.Source{Kind: "assistant"}}, "")
	d := a.requirementsDigest()
	if !strings.Contains(d, req.ID+" Customers can track an order") || strings.Contains(d, asm.ID) || strings.Contains(d, "brief") {
		t.Errorf("digest has confirmed non-Brief REQ/CON/NG only:\n%s", d)
	}
	risk, _ := a.Reg.Create(register.ActorAssistant, "reliability", "", register.New{Type: register.RISK, Statement: "No backup for the DB", Links: []string{"db"}, Source: register.Source{Kind: "assistant"}}, "")
	p := &Pending{ID: "P-4", Payload: &proposal.Payload{Title: "Add a backup vault", Addresses: []string{"register:" + risk.ID, "gap:x"}}}
	a.resolveEntries(p, 5)
	old, _ := a.Reg.Get(risk.ID)
	if old.Status != register.Superseded {
		t.Fatalf("the risk should be superseded, is %s", old.Status)
	}
	dec, _ := a.Reg.Get(old.SupersededBy)
	if dec.Type != register.DEC || dec.Status != register.Confirmed || !strings.Contains(dec.Source.Ref, "Production v5") || len(dec.Links) != 1 {
		t.Errorf("resolution: %+v", dec)
	}
}

func TestProposeEntries(t *testing.T) {
	a := pipelineApp(t)
	a.turn = &Turn{Kind: "chat", spec: &taskSpec{kind: "chat", text: "we must keep data in the EU, no mobile app"}}
	bad := `{"entries":[{"type":"CON","statement":"Data stays in the EU"},{"type":"XX","statement":"?"},{"type":"NG","statement":"No mobile app","links":["n99"]}]}`
	if _, err := a.proposeEntriesLocked(json.RawMessage(bad)); err == nil || !strings.Contains(err.Error(), "entry 2") || !strings.Contains(err.Error(), "n99") {
		t.Fatalf("problems are all reported: %v", err)
	}
	if len(a.proposedLocked().Entries) != 0 {
		t.Fatal("nothing is proposed while there are problems")
	}
	good := `{"entries":[{"type":"CON","statement":"Data stays in the EU","fields":{"origin":"regulatory"}},{"type":"NG","statement":"No mobile app","fields":{"kind":"non-goal"}}]}`
	if _, err := a.proposeEntriesLocked(json.RawMessage(good)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.proposeEntriesLocked(json.RawMessage(good)); err == nil {
		t.Error("the same entries can't be proposed twice")
	}
	n := len(a.Reg.Entries())
	ps := a.ProposedEntries()
	e, err := a.DecideProposedEntry(ps[0].ID, true, "Customer data stays in the EU")
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != register.Confirmed || e.Source.Kind != "chat" || !strings.Contains(e.Source.Text, "EU") || e.Statement != "Customer data stays in the EU" {
		t.Errorf("accepted: %+v", e)
	}
	if _, err := a.DecideProposedEntry(ps[1].ID, false, ""); err != nil {
		t.Fatal(err)
	}
	asm, _ := a.Reg.Create(register.ActorAssistant, "requirements", "", register.New{Type: register.ASM, Statement: "About 10k users", Source: register.Source{Kind: "assistant"}}, "")
	if _, err := a.proposeEntriesLocked(json.RawMessage(`{"entries":[{"type":"REQ","statement":"Up to 50k users","fields":{"kind":"non-functional"},"replaces":"` + asm.ID + `"}]}`)); err != nil {
		t.Fatal(err)
	}
	n++
	r, err := a.DecideProposedEntry(a.ProposedEntries()[0].ID, true, "")
	if err != nil || r.Supersedes != asm.ID {
		t.Fatalf("replaces supersedes: %+v %v", r, err)
	}
	n++
	if len(a.Reg.Entries()) != n+1 || len(a.ProposedEntries()) != 0 {
		t.Errorf("one accepted, one rejected: %d entries, %d waiting", len(a.Reg.Entries())-n, len(a.ProposedEntries()))
	}
}
