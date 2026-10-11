package pipeline

import (
	"strings"
	"testing"
	"time"

	"github.com/vinaayakh/designer/internal/brief"
	"github.com/vinaayakh/designer/internal/register"
	"github.com/vinaayakh/designer/internal/scene"
)

func TestStepsLoad(t *testing.T) {
	ids := []string{}
	for _, s := range Steps() {
		ids = append(ids, s.ID)
	}
	if strings.Join(ids, ",") != "requirements,design-doc,threat-model,reliability" {
		t.Fatalf("steps %v", ids)
	}
	if q := Get("requirements").Question("problem"); q == nil || !q.Blocking || q.Creates != register.REQ {
		t.Errorf("problem question: %+v", q)
	}
}

func TestClean(t *testing.T) {
	s := Get("requirements")
	if v, err := s.Question("roles").Clean([]string{"Customer\n\n  Admin "}); err != nil || strings.Join(v, "|") != "Customer|Admin" {
		t.Errorf("list: %v %v", v, err)
	}
	if _, err := s.Question("retention").Clean([]string{"forever"}); err == nil {
		t.Error("a choice must be one of its values")
	}
	if _, err := s.Question("problem").Clean([]string{"  "}); err == nil {
		t.Error("empty answers are refused")
	}
}

func graph() *scene.Graph {
	g := &scene.Graph{
		Groups: []*scene.Group{{ID: "vpc", Label: "VPC", Kind: "network"}},
		Nodes: []*scene.Node{
			{ID: "u", Label: "User", Kind: "external.user"},
			{ID: "api", Label: "API", Kind: "compute.container", Parent: "vpc"},
			{ID: "db", Label: "DB", Kind: "data.relational", Classes: []string{"pii"}, Parent: "vpc"},
			{ID: "logs", Label: "Logs", Kind: "observability.logs", Parent: "vpc"},
		},
		Edges: []*scene.Edge{{ID: "e1", From: "u", To: "api", Protocol: "HTTPS"}, {ID: "e2", From: "api", To: "db"}, {ID: "e3", From: "api", To: "logs"}},
	}
	g.Reindex()
	return g
}

func TestDerivers(t *testing.T) {
	g := graph()
	d := Derive("dfd", Input{Graph: g})
	rows := map[string][]string{}
	for _, r := range d.Rows {
		rows[r[0]] = r
	}
	if rows["User"][1] != dfdExternal || rows["User"][4] != "S R" {
		t.Errorf("user: %v", rows["User"])
	}
	if rows["Logs"][4] != "T I D R" {
		t.Errorf("a log store also gets repudiation: %v", rows["Logs"])
	}
	if r := rows["User → API"]; r == nil || !strings.Contains(r[2], "crosses a boundary") {
		t.Errorf("User → API crosses into the VPC: %v", r)
	}
	if r := rows["API → DB"]; r == nil || !strings.Contains(r[2], "carries PII") {
		t.Errorf("API → DB carries PII: %v", r)
	}
	b := brief.New()
	b.Answers["availability"] = "99.9"
	eb := Derive("error_budget", Input{Graph: g, Brief: b})
	if eb.Rows[0][2] != "40 min 19 s" {
		t.Errorf("99.9%% over 28 days = %v", eb.Rows[0])
	}
	fm := Derive("failure_modes", Input{Graph: g})
	if !strings.Contains(table(fm), "DB | no backup component on the diagram") {
		t.Errorf("failure modes:\n%s", table(fm))
	}
}

func TestAssembleChecksSchemaAndCites(t *testing.T) {
	s := Get("design-doc")
	entries := []register.Entry{
		{ID: "REQ-001", Type: register.REQ, Status: register.Confirmed, Fields: map[string]string{"kind": "goal"}},
		{ID: "ASM-001", Type: register.ASM, Status: register.Rejected},
		{ID: "DEC-001", Type: register.DEC, Status: register.Open},
	}
	written := map[string]Written{
		"context": {Body: "Shop orders reliably (REQ-001), assuming ASM-001."},
		"bogus":   {Body: "x"},
		"detail":  {Items: []string{"Edge tier: WAF and CDN"}},
	}
	secs, problems := Assemble(s, written, Input{Graph: graph()}, entries)
	joined := strings.Join(problems, "\n")
	for _, want := range []string{`no section "bogus"`, "cites ASM-001, which was rejected", "section overview: write it", "section rollout: write it"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing problem %q in:\n%s", want, joined)
		}
	}
	var alt SectionOut
	for _, s := range secs {
		if s.ID == "alternatives" {
			alt = s
		}
	}
	if strings.Join(alt.Entries, ",") != "DEC-001" {
		t.Errorf("entries sections list the live DECs: %v", alt.Entries)
	}
	if c := Cited(secs, entries); c["REQ-001"] != register.Confirmed || c["DEC-001"] != register.Open {
		t.Errorf("cited %v", c)
	}
}

func TestStatusAndStaleness(t *testing.T) {
	s := Get("design-doc")
	now := Now{BriefHash: "b1", Production: 2, Latest: map[string]*Version{}, Entries: map[string]register.Entry{}, Answers: Answers{}}
	if st := StatusOf(s, now); st.Kind != "waiting" {
		t.Errorf("design doc waits on requirements: %+v", st)
	}
	now.Latest["requirements"] = &Version{Step: "requirements", N: 1}
	if st := StatusOf(s, now); st.Kind != "needs" || st.Unanswered != 3 {
		t.Errorf("needs answers: %+v", st)
	}
	t0 := time.Now()
	now.Answers["design-doc"] = map[string]Answer{"rollout": {Values: []string{"x"}, At: t0}, "rejected": {Unknown: true, At: t0}, "reviewers": {Unknown: true, At: t0}}
	if st := StatusOf(s, now); st.Kind != "ready" {
		t.Errorf("ready: %+v", st)
	}
	v := &Version{Step: "design-doc", N: 1, Created: t0.Add(time.Second), StepVersion: 1, Inputs: Inputs{BriefHash: "b1", Production: 2, Upstream: map[string]int{"requirements": 1}, AnswersAt: t0}, Cites: map[string]register.Status{"ASM-004": register.Open}}
	now.Latest["design-doc"] = v
	now.Entries["ASM-004"] = register.Entry{ID: "ASM-004", Status: register.Open}
	if st := StatusOf(s, now); st.Kind != "current" {
		t.Errorf("current: %+v", st)
	}
	now.Production = 3
	now.Latest["requirements"] = &Version{Step: "requirements", N: 2}
	now.Entries["ASM-004"] = register.Entry{ID: "ASM-004", Status: register.Rejected}
	st := StatusOf(s, now)
	reasons := strings.Join(st.Reasons, "; ")
	if st.Kind != "stale" || !strings.Contains(reasons, "Production is now v3") || !strings.Contains(reasons, "Requirements spec is now v2") || !strings.Contains(reasons, "ASM-004 was rejected") {
		t.Errorf("stale reasons: %+v", st)
	}
}

func TestStoreAndMarkdown(t *testing.T) {
	st := Store{Dir: t.TempDir()}
	s := Get("requirements")
	entries := []register.Entry{{ID: "NG-001", Type: register.NG, Statement: "No | mobile app", Status: register.Confirmed, Source: register.Source{Kind: "wizard", Ref: "requirements·nongoals"}}}
	secs, _ := Assemble(s, map[string]Written{}, Input{Graph: graph()}, entries)
	v := &Version{Step: s.ID, Created: time.Now(), StepVersion: 1, Sections: secs, Cites: Cited(secs, entries)}
	if err := st.Save(v, Markdown(s, v, "Shop", entries)); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(&Version{Step: s.ID}, ""); err != nil {
		t.Fatal(err)
	}
	ns, _ := st.Versions(s.ID)
	if len(ns) != 2 || ns[1] != 2 {
		t.Fatalf("versions %v", ns)
	}
	l, _ := st.Latest(s.ID)
	if l.N != 2 {
		t.Errorf("latest %d", l.N)
	}
	md := Markdown(s, v, "Shop", entries)
	if !strings.Contains(md, "| NG-001 | No \\| mobile app | confirmed | wizard · requirements·nongoals |") {
		t.Errorf("markdown:\n%s", md)
	}
	a := Answers{"requirements": {"problem": {Values: []string{"x"}}}}
	st.SaveAnswers(a)
	b, _ := st.LoadAnswers()
	if b["requirements"]["problem"].Values[0] != "x" {
		t.Error("answers not saved")
	}
}
