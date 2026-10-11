package kb

import (
	"math"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Fields are weighted title > aliases > summary > body (ticket 007).
var stop = map[string]bool{}

func init() {
	for _, w := range strings.Fields("a an the and or of to in for on with by is are be it its as at from that this how what where should my i we our do does without into not") {
		stop[w] = true
	}
}

var tokRe = regexp.MustCompile(`[a-z0-9]+`)

func toks(s string) []string {
	var out []string
	for _, w := range tokRe.FindAllString(strings.ToLower(s), -1) {
		if !stop[w] {
			out = append(out, w)
		}
	}
	return out
}

var regimeNames = map[string]string{"soc2": "SOC 2 soc2", "hipaa": "HIPAA"}

type field struct {
	w   float64
	get func(*Record) string
}

var fields = []field{
	{3, func(r *Record) string { return r.Title + " " + r.Ref + " " + r.ID + " " + regimeNames[r.Regime] }},
	{2.5, func(r *Record) string { return strings.Join(r.Aliases, " ") }},
	{1.5, func(r *Record) string { return r.Summary }},
	{1, func(r *Record) string { return r.Body }},
}

type index struct {
	recs []*Record
	tf   [][]map[string]int
	dl   [][]int
	avg  []float64
	df   map[string]int
}

func build(recs []*Record) *index {
	ix := &index{recs: recs, df: map[string]int{}}
	for fi, f := range fields {
		ix.tf = append(ix.tf, nil)
		ix.dl = append(ix.dl, nil)
		tot := 0
		for _, r := range recs {
			m := map[string]int{}
			t := toks(f.get(r))
			for _, w := range t {
				m[w]++
			}
			ix.tf[fi] = append(ix.tf[fi], m)
			ix.dl[fi] = append(ix.dl[fi], len(t))
			tot += len(t)
		}
		ix.avg = append(ix.avg, float64(tot)/math.Max(1, float64(len(recs))))
	}
	for d := range recs {
		seen := map[string]bool{}
		for fi := range fields {
			for w := range ix.tf[fi][d] {
				seen[w] = true
			}
		}
		for w := range seen {
			ix.df[w]++
		}
	}
	return ix
}

func (ix *index) score(q []string, d int) float64 {
	const k1, b = 1.2, 0.75
	N := float64(len(ix.recs))
	s := 0.0
	for _, w := range q {
		df := float64(ix.df[w])
		if df == 0 {
			continue
		}
		idf := math.Log(1 + (N-df+0.5)/(df+0.5))
		for fi, f := range fields {
			tf := float64(ix.tf[fi][d][w])
			if tf == 0 {
				continue
			}
			s += f.w * idf * tf * (k1 + 1) / (tf + k1*(1-b+b*float64(ix.dl[fi][d])/ix.avg[fi]))
		}
	}
	return s
}

type SearchOpts struct {
	Type   string // control | practice | ""
	Regime string // soc2 | hipaa | ""
	Limit  int
	// BriefMatch, when set, boosts practices whose brief filter matches (a
	// ranking hint only; nothing is hidden).
	BriefMatch func(map[string][]string) bool
}

type Hit struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Summary string  `json:"summary"`
	Score   float64 `json:"-"`
}

func (k *KB) Search(q string, o SearchOpts) []Hit {
	if o.Limit <= 0 {
		o.Limit = 5
	}
	qt := toks(q)
	var hits []Hit
	for d, r := range k.ix.recs {
		if o.Type != "" && r.Type != o.Type || o.Regime != "" && r.Regime != o.Regime {
			continue
		}
		s := k.ix.score(qt, d)
		if s <= 0 {
			continue
		}
		if o.BriefMatch != nil && r.Type == "practice" && len(r.AppliesTo.Brief) > 0 && o.BriefMatch(r.AppliesTo.Brief) {
			s *= 1.25
		}
		hits = append(hits, Hit{r.ID, r.Title, r.Summary, s})
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > o.Limit {
		hits = hits[:o.Limit]
	}
	return hits
}

// ---- applies_to lookup

// Node is what the lookup needs to know about a Production Node.
type Node struct {
	Alias       string
	Kind        string
	DataClasses []string
}

type Applied struct {
	Record *Record
	Nodes  []string // aliases of the Nodes that made it apply (empty for Brief-only Records)
}

// Applies returns the Controls (brief is a hard filter) and Practices (brief
// ignored here; it is a ranking hint) that apply to a diagram. A Record
// applies when every key it lists matches; Kinds and Data Classes must match
// on the same Node.
func (k *KB) Applies(nodes []Node, cloud string, briefMatch func(map[string][]string) bool, kindMatch func(glob, kind string) bool) (controls, practices []Applied) {
	for _, r := range k.Records {
		a := r.AppliesTo
		if len(a.Clouds) > 0 && !slices.Contains(a.Clouds, cloud) {
			continue
		}
		if r.Type == "control" && len(a.Brief) > 0 && !briefMatch(a.Brief) {
			continue
		}
		var matched []string
		if len(a.Kinds) > 0 || len(a.DataClasses) > 0 {
			for _, n := range nodes {
				if len(a.Kinds) > 0 && !slices.ContainsFunc(a.Kinds, func(g string) bool { return kindMatch(g, n.Kind) }) {
					continue
				}
				if len(a.DataClasses) > 0 && !slices.ContainsFunc(a.DataClasses, func(c string) bool { return slices.Contains(n.DataClasses, c) }) {
					continue
				}
				matched = append(matched, n.Alias)
			}
			if len(matched) == 0 {
				continue
			}
		}
		ap := Applied{r, matched}
		if r.Type == "control" {
			controls = append(controls, ap)
		} else {
			practices = append(practices, ap)
		}
	}
	return
}
