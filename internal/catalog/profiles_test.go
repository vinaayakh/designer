package catalog

import (
	"testing"
)

func TestProfilesLint(t *testing.T) {
	ps, err := Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) < 10 {
		t.Fatalf("only %d Profiles loaded", len(ps))
	}
	for kind, p := range ps {
		for _, e := range lintProfile(kind, p.(map[string]any)) {
			t.Error(e)
		}
	}
}

func TestProfilesCoverKinds(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	ps, _ := Profiles()
	for kind := range ps {
		if c.Kind(kind) == nil {
			t.Errorf("Profile for unknown Kind %s", kind)
		}
	}
	// The request path of a typical web system must simulate.
	for _, k := range []string{"network.load-balancer", "network.cdn", "compute.container", "compute.vm", "compute.function", "data.relational", "data.cache", "messaging.queue"} {
		if ps[k] == nil {
			t.Errorf("no Profile for %s", k)
		}
	}
}

func TestLintCatchesUnorderedRange(t *testing.T) {
	p := map[string]any{"as_of": "x", "unit": "x", "latency": map[string]any{}, "capacity": map[string]any{
		"model": "cpu", "cpu_ms": map[string]any{"low": 5, "typical": 1, "high": 9},
	}}
	if len(lintProfile("k", p)) != 1 {
		t.Fatal("unordered range not reported")
	}
}
