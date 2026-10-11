package app

import (
	"testing"

	"github.com/vinaayakh/designer/internal/brief"
	"github.com/vinaayakh/designer/internal/project"
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
