package register

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func open(t *testing.T) (*Register, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "register", "log.jsonl")
	r, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return r, p
}

func TestCreateIDsAndReopen(t *testing.T) {
	r, p := open(t)
	a, err := r.Create(ActorRuntime, "brief", "", New{Type: CON, Statement: "Target cloud: AWS", Source: Source{Kind: "brief", Ref: "cloud"}, Status: Confirmed}, "")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := r.Create(ActorAssistant, "requirements", "requirements/v001", New{Type: ASM, Statement: "Peak is 2x average", Source: Source{Kind: "assistant"}, Status: Confirmed}, "")
	c, _ := r.Create(ActorUser, "requirements", "", New{Type: ASM, Statement: "Users are in the EU", Source: Source{Kind: "wizard", Ref: "requirements·users"}, Status: Confirmed}, "")
	if a.ID != "CON-001" || b.ID != "ASM-001" || c.ID != "ASM-002" {
		t.Fatalf("ids %s %s %s", a.ID, b.ID, c.ID)
	}
	if b.Status != Open {
		t.Errorf("the Assistant's entries must start open, got %s", b.Status)
	}
	if c.Status != Confirmed {
		t.Errorf("the user's own statement starts confirmed, got %s", c.Status)
	}
	// Rejected entries keep their ID; numbering never reuses one.
	if err := r.SetStatus(ActorUser, "ASM-001", Rejected, "not true"); err != nil {
		t.Fatal(err)
	}
	r2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := r2.Create(ActorUser, "", "", New{Type: ASM, Statement: "x", Source: Source{Kind: "user"}}, "")
	if d.ID != "ASM-003" {
		t.Errorf("next ID after reopening = %s, want ASM-003", d.ID)
	}
	e, _ := r2.Get("ASM-001")
	if e.Status != Rejected || len(e.History) != 2 || e.History[1].Reason != "not true" {
		t.Errorf("history not kept: %+v", e)
	}
	if r2.Integrity() != "" {
		t.Errorf("chain should verify: %s", r2.Integrity())
	}
}

func TestOnlyTheUserDecides(t *testing.T) {
	r, _ := open(t)
	e, _ := r.Create(ActorAssistant, "", "", New{Type: RISK, Statement: "Single region", Source: Source{Kind: "assistant"}}, "")
	if err := r.SetStatus(ActorAssistant, e.ID, Confirmed, ""); err == nil {
		t.Fatal("the Assistant must not confirm entries")
	}
	if err := r.SetStatus(ActorUser, e.ID, Status("open"), ""); err == nil {
		t.Fatal("only confirmed or rejected can be set")
	}
}

func TestSupersede(t *testing.T) {
	r, p := open(t)
	old, _ := r.Create(ActorAssistant, "", "", New{Type: ASM, Statement: "Latency target is 500 ms", Source: Source{Kind: "brief", Ref: "latency"}}, "")
	ne, err := r.Supersede(ActorRuntime, "brief", "", old.ID, New{Type: REQ, Statement: "Latency: p95 under 300 ms", Fields: map[string]string{"kind": "non-functional"}, Source: Source{Kind: "brief", Ref: "latency"}, Status: Confirmed}, "the Brief now answers it")
	if err != nil {
		t.Fatal(err)
	}
	r2, _ := Load(p)
	o, _ := r2.Get(old.ID)
	n, _ := r2.Get(ne.ID)
	if o.Status != Superseded || o.SupersededBy != ne.ID || n.Supersedes != old.ID || n.Status != Confirmed {
		t.Fatalf("old %+v new %+v", o, n)
	}
	if _, err := r2.Supersede(ActorUser, "", "", old.ID, New{Type: ASM, Statement: "x", Source: Source{Kind: "user"}}, ""); err == nil {
		t.Error("an entry can only be superseded once")
	}
	if err := r2.SetStatus(ActorUser, old.ID, Confirmed, ""); err == nil {
		t.Error("a superseded entry can't be confirmed")
	}
}

func TestValidate(t *testing.T) {
	for _, c := range []struct {
		n    New
		want string
	}{
		{New{Type: "X", Statement: "s", Source: Source{Kind: "user"}}, "type must be"},
		{New{Type: REQ, Statement: " ", Source: Source{Kind: "user"}}, "empty"},
		{New{Type: REQ, Statement: "s", Fields: map[string]string{"colour": "red"}, Source: Source{Kind: "user"}}, `no field "colour"`},
		{New{Type: REQ, Statement: "s", Fields: map[string]string{"priority": "urgent"}, Source: Source{Kind: "user"}}, "must be one of must, should, could"},
		{New{Type: RISK, Statement: "s", Source: Source{Kind: "guess"}}, "source kind"},
	} {
		err := Validate(c.n)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: got %v, want %q", c.n, err, c.want)
		}
	}
	if err := Validate(New{Type: RISK, Statement: "Region outage", Fields: map[string]string{"likelihood": "low", "impact": "high", "owner": "ops"}, Source: Source{Kind: "assistant"}}); err != nil {
		t.Error(err)
	}
}

func TestTamperingIsDetected(t *testing.T) {
	r, p := open(t)
	r.Create(ActorUser, "", "", New{Type: NG, Statement: "No mobile app", Source: Source{Kind: "user"}}, "")
	r.Create(ActorUser, "", "", New{Type: NG, Statement: "No offline mode", Source: Source{Kind: "user"}}, "")
	b, _ := os.ReadFile(p)
	edited := strings.Replace(string(b), "No mobile app", "A mobile app", 1)
	os.WriteFile(p, []byte(edited), 0o644)
	r2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r2.Integrity(), "line 2") {
		t.Errorf("an edit to line 1 should break the link from line 2, got %q", r2.Integrity())
	}
}

func TestNotesAndLinks(t *testing.T) {
	r, p := open(t)
	e, _ := r.Create(ActorAssistant, "", "", New{Type: RISK, Statement: "s", Source: Source{Kind: "assistant"}, Links: []string{"n1"}}, "")
	r.Link(ActorRuntime, e.ID, []string{"n1", "n2"}, "")
	r.Note("imported the Design Rationale")
	r2, _ := Load(p)
	got, _ := r2.Get(e.ID)
	if strings.Join(got.Links, ",") != "n1,n2" || !r2.HasNote("imported the Design Rationale") {
		t.Errorf("links %v notes ok=%v", got.Links, r2.HasNote("imported the Design Rationale"))
	}
}
