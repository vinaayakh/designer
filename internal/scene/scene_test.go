package scene

import (
	"encoding/json"
	"strings"
	"testing"
)

type el map[string]any

func rect(id string, x, y, w, h float64) el {
	return el{"id": id, "type": "rectangle", "x": x, "y": y, "width": w, "height": h}
}
func text(id, t string, container any, x, y float64) el {
	return el{"id": id, "type": "text", "text": t, "containerId": container, "x": x, "y": y, "width": 40, "height": 20}
}
func arrow(id string, from, to any, x, y float64) el {
	e := el{"id": id, "type": "arrow", "x": x, "y": y, "width": 10, "height": 0, "points": [][]float64{{0, 0}, {10, 0}}}
	if from != nil {
		e["startBinding"] = el{"elementId": from}
	}
	if to != nil {
		e["endBinding"] = el{"elementId": to}
	}
	return e
}

func sceneJSON(t *testing.T, els ...el) []byte {
	b, err := json.Marshal(el{"type": "excalidraw", "version": 2, "elements": els})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRoughDraft(t *testing.T) {
	data := sceneJSON(t,
		rect("vpc", 0, 0, 500, 300), text("vpcl", "VPC", "vpc", 0, 0),
		rect("api", 20, 20, 100, 50), text("apil", "Orders API", "api", 0, 0),
		rect("db", 200, 20, 100, 50), text("dbl", "Orders DB", "db", 0, 0),
		rect("cdn", 600, 20, 100, 50), text("cdnl", "CDN", "cdn", 0, 0),
		rect("half", 450, 200, 100, 50), text("halfl", "Cache", "half", 0, 0),
		arrow("a1", "cdn", "api", 0, 0), text("a1l", "HTTPS", "a1", 0, 0),
		arrow("a2", "api", "db", 0, 0),
		arrow("a3", "db", nil, 0, 0),
		text("note", "~10k rps peak", nil, 610, 80),
		el{"id": "gone", "type": "rectangle", "isDeleted": true, "x": 0, "y": 0, "width": 1, "height": 1},
	)
	g, err := Parse(data, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Groups) != 1 || len(g.Nodes) != 4 || len(g.Edges) != 2 {
		t.Fatalf("groups %d nodes %d edges %d", len(g.Groups), len(g.Nodes), len(g.Edges))
	}
	if g.Node("api").Parent != "vpc" || g.Node("half").Parent != "" {
		t.Errorf("membership: api in %q, half in %q", g.Node("api").Parent, g.Node("half").Parent)
	}
	a := NewAliases()
	out := g.Compact(a)
	t.Log("\n" + out)
	for _, want := range []string{
		`g1 "VPC"`,
		`  n1 "Orders API"`,
		`n3 -> n1 "HTTPS"`,
		`note "~10k rps peak" near n3`,
		`note (loose arrow) "" near n2`,
		`! overlap n4 g1`,
		`! loose-arrow`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	codes := map[string]bool{}
	for _, i := range g.PreCheck() {
		codes[i.Code] = true
	}
	if !codes["overlap"] || !codes["loose-arrow"] {
		t.Errorf("pre-check %v", g.PreCheck())
	}
}

func TestProductionAndDiff(t *testing.T) {
	meta := func(e el, m map[string]any) el { e["customData"] = el{"designer": m}; return e }
	base := []el{
		meta(rect("vpc", 0, 0, 500, 300), map[string]any{"role": "group", "kind": "network", "label": "Prod VPC"}),
		meta(rect("sub", 10, 10, 400, 200), map[string]any{"role": "group", "kind": "subnet", "tier": "private", "label": "App", "parent": "vpc"}),
		meta(rect("api", 20, 20, 200, 72), map[string]any{"role": "node", "kind": "compute.container", "service": "aws.ecs-fargate", "parent": "sub"}),
		meta(text("apil", "Orders API", nil, 70, 30), map[string]any{"role": "label", "node": "api"}),
		meta(text("apis", "Amazon ECS on AWS Fargate", nil, 70, 50), map[string]any{"role": "decor", "node": "api"}),
		meta(rect("db", 20, 120, 200, 72), map[string]any{"role": "node", "kind": "data.relational", "service": "aws.rds-postgres", "classes": []string{"PHI"}, "parent": "sub", "label": "Orders DB"}),
		meta(arrow("e1", "api", "db", 0, 0), map[string]any{"role": "edge", "style": "sync", "protocol": "SQL"}),
	}
	g, err := Parse(sceneJSON(t, base...), true)
	if err != nil {
		t.Fatal(err)
	}
	a := NewAliases()
	out := g.Compact(a)
	want := "g1 network \"Prod VPC\"\n  g2 subnet:private \"App\"\n    n1 \"Orders API\" compute.container aws.ecs-fargate\n    n2 \"Orders DB\" data.relational aws.rds-postgres [PHI]\nn1 -> n2 sync SQL\n"
	if out != want {
		t.Fatalf("got\n%s\nwant\n%s", out, want)
	}
	prev := g.Snapshot(a)

	// Mark the DB encrypted, add a cache, drop the edge, and drag the cache outside its link.
	next := append([]el{}, base[:5]...)
	next = append(next,
		meta(rect("db", 20, 120, 200, 72), map[string]any{"role": "node", "kind": "data.relational", "service": "aws.rds-postgres", "classes": []string{"PHI"}, "parent": "sub", "label": "Orders DB", "attrs": map[string]string{"encrypted": "true"}}),
		meta(rect("cache", 600, 20, 200, 72), map[string]any{"role": "node", "kind": "data.cache", "parent": "sub", "label": "Cache"}),
	)
	g2, _ := Parse(sceneJSON(t, next...), true)
	diff := Diff(prev, g2.Snapshot(a), a)
	t.Log("\n" + diff)
	for _, w := range []string{"- n1 -> n2 sync SQL", `~ n2 "Orders DB" data.relational aws.rds-postgres [PHI] {encrypted} in g2`, `+ n3 "Cache" data.cache in g2`} {
		if !strings.Contains(diff, w) {
			t.Errorf("diff missing %q", w)
		}
	}
	if len(g2.Issues) != 1 || g2.Issues[0].Code != "parent-mismatch" {
		t.Errorf("issues %v", g2.Issues)
	}
}

func TestEmptyListsEncodeAsArrays(t *testing.T) {
	g, _ := Parse([]byte(`{"type":"excalidraw","elements":[]}`), false)
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") {
		t.Fatalf("empty lists must be []: %s", b)
	}
}
