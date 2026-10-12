package app

import (
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
