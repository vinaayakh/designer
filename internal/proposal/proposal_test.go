package proposal

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vinaayakh/designer/internal/catalog"
	"github.com/vinaayakh/designer/internal/scene"
)

const prodScene = `{"type":"excalidraw","elements":[
 {"id":"vpc","type":"rectangle","x":0,"y":0,"width":900,"height":400,"customData":{"designer":{"role":"group","kind":"network","label":"Prod VPC"}}},
 {"id":"sub","type":"rectangle","x":10,"y":10,"width":800,"height":300,"customData":{"designer":{"role":"group","kind":"subnet","tier":"private","label":"App","parent":"vpc"}}},
 {"id":"api","type":"rectangle","x":20,"y":20,"width":200,"height":72,"customData":{"designer":{"role":"node","kind":"compute.container","service":"aws.ecs-fargate","label":"Orders API","parent":"sub"}}},
 {"id":"db","type":"rectangle","x":300,"y":20,"width":200,"height":72,"customData":{"designer":{"role":"node","kind":"data.relational","service":"aws.rds-postgres","classes":["PHI"],"label":"Orders DB","parent":"sub"}}},
 {"id":"e1","type":"arrow","x":0,"y":0,"width":1,"height":1,"startBinding":{"elementId":"api"},"endBinding":{"elementId":"db"},"customData":{"designer":{"role":"edge","style":"sync","protocol":"SQL"}}}
]}`

func ctx(t *testing.T, data string, production bool, cloud string) *Context {
	t.Helper()
	g, err := scene.Parse([]byte(data), production)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	a := scene.NewAliases()
	g.Compactless(a) // g1 vpc, g2 sub, n1 api, n2 db
	return &Context{Graph: g, Aliases: a, Catalog: cat, Cloud: cloud,
		RecordExists: func(id string) bool { return id == "practice.cache-aside" || id == "practice.encryption-at-rest" },
		TrustedHost:  func(h string) bool { return h == "docs.aws.amazon.com" }}
}

func decode(t *testing.T, s string) *Payload {
	t.Helper()
	p, err := Decode(json.RawMessage(s))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestValidProposalWithDependencies(t *testing.T) {
	c := ctx(t, prodScene, true, "aws")
	p := decode(t, `{"title":"Add a cache","summary":"Read-heavy","view":"production","changes":[
	 {"id":"c1","op":"add_node","ref":"+cache","label":"Orders cache","kind":"data.cache","service":"aws.elasticache-redis","classes":["PHI"],"parent":"g2","why":"read-heavy","practices":["practice.cache-aside"]},
	 {"id":"c2","op":"add_edge","from":"n1","to":"+cache","style":"sync","protocol":"RESP"},
	 {"id":"c3","op":"update_node","ref":"n2","set":{"encrypted":true}},
	 {"id":"c4","op":"update_edge","from":"n1","to":"n2","set":{"protocol":"SQL over TLS"}}
	]}`)
	if err := Prepare(p, c); err != nil {
		t.Fatal(err)
	}
	if p.Changes[0].ElementID == "" || p.Changes[1].ToID != p.Changes[0].ElementID {
		t.Fatal("+cache was not resolved to one element id")
	}
	if got := p.Changes[1].Needs; len(got) != 1 || got[0] != "c1" {
		t.Fatalf("c2 needs %v", got)
	}
	// Skipping the new Node skips the Edge that needs it.
	acc := Closure(p.Changes, map[string]bool{"c2": true, "c3": true, "c4": true})
	if acc["c2"] || !acc["c3"] {
		t.Fatalf("closure %v", acc)
	}
	res, _ := Simulate(c.Graph, p.Changes, nil)
	db := res.Node("db")
	if db.Attrs["encrypted"] != "true" {
		t.Fatalf("attrs %v", db.Attrs)
	}
	if n := res.Node(p.Changes[0].ElementID); n == nil || n.Parent != "sub" {
		t.Fatal("cache not added inside the App subnet")
	}
}

func TestInvalidProposalListsEveryProblem(t *testing.T) {
	c := ctx(t, prodScene, true, "aws")
	p := decode(t, `{"title":"Bad","view":"production","changes":[
	 {"id":"c1","op":"add_node","ref":"+q","label":"Queue","kind":"messaging.queue","service":"aws.rds-postgres"},
	 {"id":"c2","op":"add_node","ref":"+fn","label":"Fn","kind":"compute.function","classes":["PHI"]},
	 {"id":"c3","op":"add_edge","from":"n1","to":"n2","style":"sync"},
	 {"id":"c4","op":"update_node","ref":"n9","set":{"label":"x"}},
	 {"id":"c5","op":"teleport"}
	]}`)
	err := Prepare(p, c)
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"is a data.relational, not a messaging.queue", "compute.function doesn't hold data", "already exists", `"n9" is not an existing Node`, "unknown op"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in\n%v", want, err)
		}
	}
}

func TestConversionNeedsEveryNodeTyped(t *testing.T) {
	draft := `{"type":"excalidraw","elements":[
	 {"id":"a","type":"rectangle","x":0,"y":0,"width":100,"height":50},{"id":"al","type":"text","text":"API","containerId":"a","x":0,"y":0,"width":10,"height":10},
	 {"id":"b","type":"rectangle","x":300,"y":0,"width":100,"height":50},{"id":"bl","type":"text","text":"DB","containerId":"b","x":0,"y":0,"width":10,"height":10},
	 {"id":"e","type":"arrow","x":0,"y":0,"width":1,"height":1,"startBinding":{"elementId":"a"},"endBinding":{"elementId":"b"}}]}`
	c := ctx(t, draft, false, "aws")
	p := decode(t, `{"title":"Design","kind":"conversion","view":"production","rationale":{"summary":"s","nodes":{"n1":{"why":"api","cites":[{"url":"https://evil.example.com/x"}]}}},"changes":[
	 {"id":"c1","op":"add_group","ref":"+vpc","label":"VPC","kind":"network"},
	 {"id":"c2","op":"update_node","ref":"n1","set":{"kind":"compute.container","service":"aws.ecs-fargate"},"parent":"+vpc"}
	]}`)
	err := Prepare(p, c)
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{`"DB" has no kind`, "no style after the conversion", "not a Trusted Source"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in\n%v", want, err)
		}
	}
}

func TestAddNodeKeepsAttributes(t *testing.T) {
	c := ctx(t, prodScene, true, "aws")
	p := decode(t, `{"title":"Cache","view":"production","changes":[
	 {"id":"c1","op":"add_node","ref":"+cache","label":"Cache","kind":"data.cache","service":"aws.elasticache-redis","parent":"g2","set":{"encrypted":true}}]}`)
	if err := Prepare(p, c); err != nil {
		t.Fatal(err)
	}
	res, _ := Simulate(c.Graph, p.Changes, nil)
	if n := res.Node(p.Changes[0].ElementID); n == nil || n.Attrs["encrypted"] != "true" {
		t.Fatalf("attributes lost: %+v", n)
	}
}

func TestSameIgnoresRuntimeFields(t *testing.T) {
	a := &Change{ID: "c1", Op: "add_node", Ref: "+cache", Label: "Cache", Kind: "data.cache", Why: "fast", ElementID: "x1"}
	b := &Change{ID: "c1", Op: "add_node", Ref: "+cache", Label: "Cache", Kind: "data.cache", Why: "faster reads", ElementID: "y2"}
	if !Same(a, b) {
		t.Fatal("changes that say the same thing should be Same")
	}
	b.Label = "Redis"
	if Same(a, b) {
		t.Fatal("a different label is a different change")
	}
}
