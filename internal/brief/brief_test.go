package brief

import (
	"strings"
	"testing"
)

func TestRoundTripWritesEveryField(t *testing.T) {
	b := New()
	b.Answers["cloud"] = "aws"
	b.Answers["cap"] = "ap"
	b.Answers["availability"] = "99.9"
	b.Answers["users"] = "1k-100k"
	b.Compliance = []string{"hipaa"}
	b.Notes = "Must integrate with on-prem SAP: via VPN"
	out := b.Marshal()
	for _, f := range TheSchema().Fields {
		if !strings.Contains(string(out), "\n"+f.ID+":") {
			t.Errorf("brief.yaml is missing %s:\n%s", f.ID, out)
		}
	}
	got, err := Parse(out)
	if err != nil {
		t.Fatal(err, "\n", string(out))
	}
	if got.Answers["availability"] != "99.9" || got.Answers["users"] != "1k-100k" || got.Notes != b.Notes || got.Compliance[0] != "hipaa" {
		t.Fatalf("round trip lost data: %+v", got)
	}
	if !got.Complete() {
		t.Fatal("expected complete")
	}
}

func TestNoneIsExclusive(t *testing.T) {
	b := New()
	b.Compliance = []string{"none", "soc2"}
	if b.Validate() == nil {
		t.Fatal("none + soc2 should be rejected")
	}
}

func TestMatches(t *testing.T) {
	b := New()
	b.Compliance = []string{"soc2"}
	if b.Matches(map[string][]string{"compliance": {"hipaa"}}) {
		t.Error("soc2 brief matched a hipaa filter")
	}
	if !b.Matches(map[string][]string{"read_write_bias": {"read-heavy"}}) {
		t.Error("unknown answer should match")
	}
	b.Answers["read_write_bias"] = "balanced"
	if b.Matches(map[string][]string{"read_write_bias": {"read-heavy"}}) {
		t.Error("balanced matched read-heavy")
	}
}

func TestCompact(t *testing.T) {
	b := New()
	b.Answers["cloud"] = "aws"
	b.Answers["cap"] = "ap"
	b.Answers["read_write_bias"] = "read-heavy"
	b.Compliance = []string{"hipaa"}
	got := b.Compact()
	if !strings.HasPrefix(got, "brief: cloud=aws compliance=hipaa cap=ap rw=read-heavy\nunknown: users latency") {
		t.Fatalf("got %q", got)
	}
}
