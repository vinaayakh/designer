package project

import (
	"errors"
	"os"
	"testing"
)

func TestScenarios(t *testing.T) {
	p, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if l, _ := p.Scenarios(); len(l) != 0 {
		t.Fatalf("new Project has Scenarios: %v", l)
	}
	if _, err := p.Scenario("baseline"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing Scenario: %v", err)
	}
	in := map[string]any{"name": "Baseline", "base": 3, "seed": 1234, "changes": []any{map[string]any{"op": "set_sizing", "node": "n1"}}}
	if err := p.SaveScenario("baseline", in); err != nil {
		t.Fatal(err)
	}
	out, err := p.Scenario("baseline")
	if err != nil {
		t.Fatal(err)
	}
	if out["name"] != "Baseline" || out["base"] != 3 || len(out["changes"].([]any)) != 1 {
		t.Fatalf("round trip: %v", out)
	}
	if l, _ := p.Scenarios(); len(l) != 1 || l[0] != "baseline" {
		t.Fatalf("list: %v", l)
	}
	for _, bad := range []string{"../x", "Black Friday", "", "a/b"} {
		if err := p.SaveScenario(bad, in); !errors.Is(err, ErrBadSlug) {
			t.Errorf("slug %q accepted", bad)
		}
	}
}
