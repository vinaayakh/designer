package project

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestProjectLifecycle(t *testing.T) {
	p, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.Brief()
	if err != nil || b.Complete() {
		t.Fatalf("new brief: %v complete=%v", err, b.Complete())
	}
	if err := p.SaveRoughDraft([]byte(EmptyScene)); err != nil {
		t.Fatal(err)
	}

	v1, err := p.SaveVersion([]byte(EmptyScene), "conversion", "first", "", json.RawMessage(`{"summary":"x"}`))
	if err != nil || v1.N != 1 {
		t.Fatalf("v1 %v %v", v1, err)
	}
	if err := p.Freeze(); err != nil {
		t.Fatal(err)
	}
	if err := p.SaveRoughDraft([]byte(EmptyScene)); !errors.Is(err, ErrFrozen) {
		t.Fatalf("frozen draft saved: %v", err)
	}

	e1, _ := p.SaveVersion([]byte(EmptyScene), "edit", "", "", nil)
	e2, _ := p.SaveVersion([]byte(EmptyScene), "edit", "", "", nil)
	if e1.N != 2 || e2.N != 2 {
		t.Fatalf("hand edits should coalesce: %d %d", e1.N, e2.N)
	}
	got, _, _ := p.Version(2)
	var m map[string]string
	if json.Unmarshal(got.Rationale, &m) != nil || m["summary"] != "x" {
		t.Fatalf("rationale not carried forward: %s", got.Rationale)
	}
	if _, err := p.SaveReport(2, json.RawMessage(`{"items":[]}`)); err != nil {
		t.Fatal(err)
	}
	e3, _ := p.SaveVersion([]byte(EmptyScene), "edit", "", "", nil)
	if e3.N != 3 {
		t.Fatalf("a checked version must not be overwritten, got v%d", e3.N)
	}
	refs, _ := p.Reports()
	if len(refs) != 1 || refs[0].Version != 2 {
		t.Fatalf("reports %v", refs)
	}
}
