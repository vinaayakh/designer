package app

import (
	"testing"

	"github.com/vinaayakh/designer/internal/scene"
)

func TestNameAliases(t *testing.T) {
	g := &scene.Graph{
		Nodes:  []*scene.Node{{ID: "a1", Label: "Audit trail"}, {ID: "p1", Label: "Postgres primary"}, {ID: "w1", Label: "WAF"}},
		Groups: []*scene.Group{{ID: "v1", Label: "App VPC"}},
	}
	g.Reindex()
	a := &App{sess: newSessionState()}
	a.sess.Aliases.Of("a1", false) // n1
	a.sess.Aliases.Of("p1", false) // n2
	a.sess.Aliases.Of("v1", true)  // g1
	refs := map[string]string{"+waf": "w1"}
	for in, want := range map[string]string{
		"But the Audit trail (n1) is not encrypted.":    "But the Audit trail is not encrypted.",
		"only the primary (n2) uses keys":               "only the primary (Postgres primary) uses keys",
		"n2 replicates inside g1.":                      "Postgres primary replicates inside App VPC.",
		"+waf filters traffic; n9 is unknown; +1 more.": "WAF filters traffic; n9 is unknown; +1 more.",
		"Use 2 replicas (n10) and 5G":                   "Use 2 replicas (n10) and 5G",
	} {
		if got := a.nameAliasesLocked(in, g, refs); got != want {
			t.Errorf("%q\n got %q\nwant %q", in, got, want)
		}
	}
	if got := dropAsides("The Audit trail (n15) and primary (n7) lack  keys (+kms)."); got != "The Audit trail and primary lack keys." {
		t.Errorf("dropAsides: %q", got)
	}
}
