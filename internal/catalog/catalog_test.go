package catalog

import (
	"testing"
)

func TestBuiltInCatalogPassesChecks(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(c.Kinds); n != 48 {
		t.Errorf("want 48 Kinds, got %d", n)
	}
	for p, cl := range c.Clouds {
		if n := len(cl.Services); n < 80 || n > 120 {
			t.Errorf("%s: %d Services, want 80–120", p, n)
		}
		t.Logf("%s: %d Services, export ~%d tokens", p, len(cl.Services), EstimateTokens(c.Export(p)))
	}
	t.Logf("kinds prompt ~%d tokens", EstimateTokens(c.KindsPrompt()))
}

func TestMatchKind(t *testing.T) {
	if !MatchKind("data.*", "data.key-value") || MatchKind("data.*", "messaging.queue") || !MatchKind("data.cache", "data.cache") {
		t.Fatal("glob matching is wrong")
	}
}
