package kb

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/vinaayakh/designer/internal/brief"
	"github.com/vinaayakh/designer/internal/catalog"
	"gopkg.in/yaml.v3"
)

const root = "../.."

func load(t *testing.T) *KB {
	t.Helper()
	k, err := Load(os.DirFS(root), "kb/records")
	if err != nil {
		t.Fatal(err)
	}
	if len(k.Records) == 0 {
		t.Fatal("no records")
	}
	return k
}

// TestLint is the Knowledge Base CI check: format rules, Kind globs, Data
// Classes and brief filters checked against the catalog and brief schema.
func TestLint(t *testing.T) {
	k := load(t)
	c, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	kindOK := func(g string) bool {
		for _, kd := range c.Kinds {
			if catalog.MatchKind(g, kd.ID) {
				return true
			}
		}
		return false
	}
	for _, p := range k.Lint(kindOK, brief.ValidFilter, c.ValidDataClass) {
		t.Error(p)
	}
}

var wordRe = regexp.MustCompile(`[a-z0-9]+(?:[.'-][a-z0-9]+)*`)

// TestCopyCheck rejects verbatim runs of 12+ words from facts-only sources (ADR 0002).
func TestCopyCheck(t *testing.T) {
	k := load(t)
	words := func(s string) []string { return wordRe.FindAllString(strings.ToLower(s), -1) }
	for _, r := range k.Records {
		for _, s := range r.Sources {
			if s.LicenceTier != "facts-only" || s.Copy == "" {
				continue
			}
			b, err := os.ReadFile(filepath.Join(root, s.Copy))
			if err != nil {
				t.Errorf("%s: copy %s missing", r.ID, s.Copy)
				continue
			}
			if n := longestRun(words(r.Summary+" "+r.Body), words(string(b))); n >= 12 {
				t.Errorf("%s: %d-word verbatim run from %s", r.ID, n, s.Copy)
			}
		}
	}
}

func longestRun(a, b []string) int {
	best := 0
	prev := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
				best = max(best, cur[j])
			}
		}
		prev = cur
	}
	return best
}

// TestSearchEval runs kb/eval/queries.yaml: every expected Record in the top 5.
func TestSearchEval(t *testing.T) {
	k := load(t)
	b, err := os.ReadFile(filepath.Join(root, "kb/eval/queries.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var qs []struct {
		Query  string   `yaml:"query"`
		Expect []string `yaml:"expect"`
	}
	if err := yaml.Unmarshal(b, &qs); err != nil {
		t.Fatal(err)
	}
	top1 := 0
	for _, q := range qs {
		hits := k.Search(q.Query, SearchOpts{})
		var ids []string
		for _, h := range hits {
			ids = append(ids, h.ID)
		}
		for _, e := range q.Expect {
			if !strings.Contains(" "+strings.Join(ids, " ")+" ", " "+e+" ") {
				t.Errorf("%q: %s not in top 5 %v", q.Query, e, ids)
			}
		}
		if len(ids) > 0 && strings.Contains(" "+strings.Join(q.Expect, " ")+" ", " "+ids[0]+" ") {
			top1++
		}
	}
	t.Logf("top-1 %d/%d", top1, len(qs))
}

func TestApplies(t *testing.T) {
	k := load(t)
	b := brief.New()
	b.Compliance = []string{"hipaa"}
	nodes := []Node{{Alias: "n1", Kind: "data.relational", DataClasses: []string{"PHI"}}, {Alias: "n2", Kind: "compute.container"}}
	controls, practices := k.Applies(nodes, "aws", b.Matches, catalog.MatchKind)
	ids := func(as []Applied) (s []string) {
		for _, a := range as {
			s = append(s, a.Record.ID)
		}
		return
	}
	if got := ids(controls); len(got) != 1 || got[0] != "hipaa.164-312-a-2-iv" {
		t.Errorf("controls %v", got)
	}
	if controls[0].Nodes[0] != "n1" {
		t.Errorf("matched nodes %v", controls[0].Nodes)
	}
	if !slicesContains(ids(practices), "practice.encryption-at-rest") || !slicesContains(ids(practices), "practice.cache-aside") {
		t.Errorf("practices %v", ids(practices))
	}
}

func slicesContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
