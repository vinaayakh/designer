package scene

import (
	"strings"
	"testing"
)

// prodScene builds a Production scene: nodes are id → kind, edges are "from>to".
func prodScene(t *testing.T, nodes [][2]string, edges ...string) *Graph {
	t.Helper()
	var els []el
	for i, n := range nodes {
		r := rect(n[0], float64(i*300), 0, 200, 72)
		r["customData"] = el{"designer": map[string]any{"role": "node", "kind": n[1], "label": n[0]}}
		els = append(els, r)
	}
	for i, e := range edges {
		from, to, _ := strings.Cut(e, ">")
		a := arrow("e"+string(rune('a'+i)), from, to, 0, 0)
		a["customData"] = el{"designer": map[string]any{"role": "edge", "style": "sync"}}
		els = append(els, a)
	}
	g, err := Parse(sceneJSON(t, els...), true)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func ids(ds []Detached) []string {
	var out []string
	for _, d := range ds {
		out = append(out, strings.Join(d.IDs, "+"))
	}
	return out
}

func TestDetached(t *testing.T) {
	system := [][2]string{{"user", "external.user"}, {"cdn", "network.cdn"}, {"api", "compute.container"}, {"db", "data.relational"}}

	t.Run("connected diagram has nothing detached", func(t *testing.T) {
		g := prodScene(t, system, "user>cdn", "cdn>api", "api>db")
		if d := g.Detached(); d != nil {
			t.Fatalf("got %v", d)
		}
	})

	t.Run("a pair linked only to each other, and a lone node", func(t *testing.T) {
		nodes := append(system, [2]string{"trail", "observability.audit"}, [2]string{"guard", "security.threat-detection"}, [2]string{"kms", "security.key-management"})
		g := prodScene(t, nodes, "user>cdn", "cdn>api", "api>db", "trail>guard")
		d := g.Detached()
		if got := ids(d); len(got) != 2 || got[0] != "trail+guard" || got[1] != "kms" {
			t.Fatalf("got %v", got)
		}
		if d[0].Isolated || !d[1].Isolated {
			t.Errorf("isolated flags %+v", d)
		}
		// It reaches the AI as an issue, and doesn't block Conversion.
		out := g.Compact(NewAliases())
		if !strings.Contains(out, `! detached n5 n6: "trail", "guard" are connected only to each other`) || !strings.Contains(out, `! detached n7: "kms" isn't connected to anything`) {
			t.Errorf("compact:\n%s", out)
		}
		for _, i := range g.PreCheck() {
			if i.Code == "detached" {
				t.Error("detached blocks the Conversion pre-check")
			}
		}
	})

	t.Run("the part users enter is the main system, even if smaller", func(t *testing.T) {
		nodes := [][2]string{{"user", "external.user"}, {"web", "compute.container"}, {"a", "compute.vm"}, {"b", "compute.vm"}, {"c", "data.relational"}}
		g := prodScene(t, nodes, "user>web", "a>b", "b>c")
		if got := ids(g.Detached()); len(got) != 1 || got[0] != "a+b+c" {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("a watcher observing a Group is attached to everything in it", func(t *testing.T) {
		node := func(id, kind, parent string) el {
			r := rect(id, 0, 0, 200, 72)
			r["customData"] = el{"designer": map[string]any{"role": "node", "kind": kind, "label": id, "parent": parent}}
			return r
		}
		acct := rect("acct", -50, -50, 2000, 1000)
		acct["customData"] = el{"designer": map[string]any{"role": "group", "kind": "account", "label": "AWS account"}}
		vpc := rect("vpc", 0, 0, 1500, 800)
		vpc["customData"] = el{"designer": map[string]any{"role": "group", "kind": "network", "label": "VPC", "parent": "acct"}}
		edge := func(id, from, to, style string) el {
			a := arrow(id, from, to, 0, 0)
			a["customData"] = el{"designer": map[string]any{"role": "edge", "style": style}}
			return a
		}
		els := []el{acct, vpc,
			node("user", "external.user", ""), node("api", "compute.container", "vpc"), node("db", "data.relational", "vpc"),
			node("trail", "observability.audit", "acct"), node("guard", "security.threat-detection", "acct"),
			edge("e1", "user", "api", "sync"), edge("e2", "api", "db", "sync"), edge("e3", "trail", "guard", "stream")}
		g, err := Parse(sceneJSON(t, els...), true)
		if err != nil {
			t.Fatal(err)
		}
		if got := ids(g.Detached()); len(got) != 1 || got[0] != "trail+guard" {
			t.Fatalf("before: %v", got)
		}
		g, _ = Parse(sceneJSON(t, append(els, edge("e4", "acct", "trail", "observe"))...), true)
		if d := g.Detached(); d != nil {
			t.Fatalf("CloudTrail observes the account, so nothing is detached: %v", ids(d))
		}
		if !strings.Contains(g.Compact(NewAliases()), "g1 -> n3 observe") {
			t.Errorf("compact:\n%s", g.Compact(NewAliases()))
		}
	})

	t.Run("no arrows yet: nothing is detached", func(t *testing.T) {
		g := prodScene(t, system)
		if d := g.Detached(); d != nil {
			t.Fatalf("got %v", d)
		}
	})

	t.Run("becoming detached mid-session shows in the change list", func(t *testing.T) {
		a := NewAliases()
		before := prodScene(t, system, "user>cdn", "cdn>api", "api>db").Snapshot(a)
		after := prodScene(t, system, "user>cdn", "cdn>api") // the API → DB arrow was deleted
		diff := Diff(before, after.Snapshot(a), a)
		if !strings.Contains(diff, `+ ! detached n4: "db" isn't connected to anything`) {
			t.Errorf("diff:\n%s", diff)
		}
		back := Diff(after.Snapshot(a), before, a)
		if !strings.Contains(back, `- ! detached n4`) {
			t.Errorf("reconnecting should clear it:\n%s", back)
		}
	})
}
