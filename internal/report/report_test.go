package report

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"image"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/vinaayakh/designer/internal/kb"
	"github.com/vinaayakh/designer/internal/proposal"
	"github.com/vinaayakh/designer/internal/register"
	"github.com/vinaayakh/designer/internal/scene"
)

func fixture(withText bool) *Input {
	g := &scene.Graph{
		Production: true,
		Groups:     []*scene.Group{{ID: "vpc", Label: "App VPC", Kind: "network"}, {ID: "priv", Label: "Private", Kind: "subnet", Tier: "private", Parent: "vpc"}},
		Nodes: []*scene.Node{
			{ID: "user", Label: "End user", Kind: "external.user"},
			{ID: "api", Label: "API", Kind: "compute.container", Service: "aws.ecs", Parent: "priv"},
			{ID: "db", Label: "Orders DB", Kind: "data.relational", Service: "aws.rds-postgres", Classes: []string{"pii"}, Parent: "priv", Attrs: map[string]string{"multi_az": "true"}},
			{ID: "logs", Label: "Central logs & <alerts>", Kind: "observability.logs", Parent: "vpc"},
		},
		Edges: []*scene.Edge{
			{ID: "e1", From: "user", To: "api", Protocol: "HTTPS", Label: "orders", Style: "sync"},
			{ID: "e2", From: "api", To: "db", Protocol: "SQL/TLS", Style: "sync"},
			{ID: "e3", From: "api", To: "logs", Protocol: "HTTPS", Label: "app logs", Style: "async"},
		},
	}
	g.Reindex()
	in := &Input{
		Project: "Shop & Co", Version: 2, Created: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), Generated: time.Now().UTC(), Graph: g,
		Brief: []BriefRow{
			{ID: "cloud", Label: "Target cloud", Value: "AWS"},
			{ID: "availability", Label: "Availability target", Value: "99.9%"},
			{ID: "rpo", Label: "RPO (data loss)", Unknown: true},
		},
		Regimes: []string{"soc2", "hipaa"}, RegimeNames: map[string]string{"soc2": "SOC 2", "hipaa": "HIPAA"}, Cloud: "AWS",
		Kinds:      map[string]string{"external.user": "a person / actor", "compute.container": "container service", "data.relational": "relational database", "observability.logs": "central logs"},
		GroupKinds: map[string]string{"network": "network", "subnet": "subnet"},
		Services:   map[string]Service{"aws.ecs": {Name: "Amazon ECS", Docs: "https://docs.aws.amazon.com/ecs/"}, "aws.rds-postgres": {Name: "Amazon RDS for PostgreSQL", HIPAA: true}},
		Rationale: &proposal.Rationale{Summary: "A containerised API over Postgres.", Assumptions: []string{"Orders are under 1 TB"},
			Nodes:     map[string]*proposal.NodeRationale{"db": {Why: "Managed Postgres with Multi-AZ.", Practices: []string{"practice.encryption-at-rest"}}},
			Tradeoffs: []proposal.Tradeoff{{Decision: "Database", Chose: "Postgres", Over: "DynamoDB", Because: "relational orders"}}},
		Gap: &Gap{Version: 2, Created: time.Now().UTC(), Items: []GapItem{
			{Record: "soc2.cc6-1", Regime: "soc2", Status: "met", Nodes: []string{"db"}, Finding: "Encrypted."},
			{Record: "hipaa.164-312-b", Regime: "hipaa", Status: "unmet", Nodes: []string{"logs"}, Finding: "No audit trail.", Fix: []string{"practice.encryption-at-rest"}},
			{Record: "soc2.cc1-1", Regime: "soc2", Coverage: "process", Status: "outside", ByRuntime: true, Finding: "Policy."},
		}},
		Applies: map[string][]string{"db": {"soc2.cc6-1", "practice.encryption-at-rest"}},
		Records: map[string]*kb.Record{
			"soc2.cc6-1":                  {ID: "soc2.cc6-1", Type: "control", Regime: "soc2", Ref: "CC6.1", Title: "Logical access", Sources: []kb.Source{{Title: "AICPA TSC", URL: "https://www.aicpa-cima.com/", AsOf: "2025-01"}}},
			"hipaa.164-312-b":             {ID: "hipaa.164-312-b", Type: "control", Regime: "hipaa", Ref: "164.312(b)", Title: "Audit controls"},
			"soc2.cc1-1":                  {ID: "soc2.cc1-1", Type: "control", Regime: "soc2", Ref: "CC1.1", Title: "Integrity and ethics"},
			"practice.encryption-at-rest": {ID: "practice.encryption-at-rest", Type: "practice", Title: "Encryption at rest", Summary: "Encrypt stored data."},
		},
		HIPAAUnverified: true,
	}
	if withText {
		in.Narrative = &Narrative{Version: 2, Overview: "An online shop's order API.",
			Components: map[string][]string{"user": {"Places orders."}, "api": {"Validates orders.", "Writes them."}, "db": {"Stores orders."}, "logs": {"Keeps logs."}},
			Groups:     map[string]string{"vpc": "Isolates the workload."},
			Scenarios:  []Scenario{{Title: "Place an order", Steps: []Step{{From: "user", To: "api", What: "submits"}, {From: "api", To: "db", What: "inserts"}, {From: "gone", To: "db", What: "stale"}}}},
			Concepts:   []Concept{{Topic: "Encryption", Text: "KMS everywhere.", Nodes: []string{"db"}}},
			Quality:    []Quality{{Attribute: "Availability", Requirement: "99.9%", Scenario: "AZ loss", Response: "Multi-AZ failover", Nodes: []string{"db"}}},
		}
	}
	return in
}

func tinyPNG(t *testing.T) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 300, 160))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBuildSections(t *testing.T) {
	for _, withText := range []bool{false, true} {
		d := Build(fixture(withText))
		var nums []string
		for _, s := range d.Sections {
			nums = append(nums, s.Num)
		}
		if got := strings.Join(nums, ","); got != "1,2,3,4,5,6,7,8,9,10,11,12,A,B" {
			t.Fatalf("sections = %s", got)
		}
		comps := d.Sections[4].Sub[1].Sub
		if len(comps) != 4 {
			t.Fatalf("component details = %d, want 4", len(comps))
		}
	}
}

func TestNarrativeStale(t *testing.T) {
	in := fixture(true)
	if in.Narrative.Stale(in.Graph) == false {
		t.Fatal("a step about a removed Node should make the text stale")
	}
	in.Narrative.Scenarios[0].Steps = in.Narrative.Scenarios[0].Steps[:2]
	if in.Narrative.Stale(in.Graph) {
		t.Fatal("text covering every Node should not be stale")
	}
	delete(in.Narrative.Components, "logs")
	if !in.Narrative.Stale(in.Graph) {
		t.Fatal("a Node without responsibilities should make the text stale")
	}
}

func TestHTML(t *testing.T) {
	out := string(HTML(Build(fixture(true)), `<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`))
	for _, want := range []string{`<h2><span class="num">7</span>Deployment View</h2>`, `class="sec break landscape"`, "<svg", "Shop &amp; Co", "Central logs &amp; &lt;alerts&gt;", "Validates orders.", "✘ Unmet", "HIPAA 164.312(b)", "Place an order"} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML is missing %q", want)
		}
	}
	if strings.Contains(out, "stale") {
		t.Error("a step about a removed Node was rendered")
	}
}

func TestDOCXIsWellFormed(t *testing.T) {
	for _, withText := range []bool{false, true} {
		out, err := DOCX(Build(fixture(withText)), tinyPNG(t))
		if err != nil {
			t.Fatal(err)
		}
		z, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
		if err != nil {
			t.Fatal(err)
		}
		parts := map[string]string{}
		for _, f := range z.File {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			parts[f.Name] = string(b)
			if strings.HasSuffix(f.Name, ".xml") || strings.HasSuffix(f.Name, ".rels") {
				dec := xml.NewDecoder(bytes.NewReader(b))
				for {
					if _, err := dec.Token(); err == io.EOF {
						break
					} else if err != nil {
						t.Fatalf("%s is not well-formed: %v", f.Name, err)
					}
				}
			}
		}
		for _, name := range []string{"[Content_Types].xml", "word/document.xml", "word/styles.xml", "word/numbering.xml", "word/media/diagram.png", "word/_rels/document.xml.rels"} {
			if _, ok := parts[name]; !ok {
				t.Fatalf("missing part %s", name)
			}
		}
		doc := parts["word/document.xml"]
		for _, want := range []string{"Deployment View", `w:orient="landscape"`, `r:embed="rIdImage"`, "Shop &amp; Co", "w:hyperlink"} {
			if !strings.Contains(doc, want) {
				t.Errorf("document.xml is missing %q", want)
			}
		}
		if !strings.Contains(parts["word/_rels/document.xml.rels"], `Target="https://docs.aws.amazon.com/ecs/" TargetMode="External"`) {
			t.Error("hyperlink relationship missing")
		}
	}
}

func TestDOCXRejectsBadImage(t *testing.T) {
	if _, err := DOCX(Build(fixture(false)), []byte("not a png")); err == nil {
		t.Fatal("want an error for a non-PNG diagram")
	}
	if _, err := DOCX(Build(fixture(false)), nil); err != nil {
		t.Fatalf("no image should still work: %v", err)
	}
}

func TestRegisterSections(t *testing.T) {
	in := fixture(true)
	e := func(id string, ty register.Type, st register.Status, text string, f map[string]string) register.Entry {
		return register.Entry{ID: id, Type: ty, Status: st, Statement: text, Fields: f, Source: register.Source{Kind: "wizard"}}
	}
	in.Entries = []register.Entry{
		e("REQ-004", register.REQ, register.Open, "Customers can track an order", map[string]string{"kind": "functional", "priority": "must"}),
		e("CON-007", register.CON, register.Confirmed, "Data stays in the EU", nil),
		e("ASM-002", register.ASM, register.Open, "Orders are paid by card", nil),
		e("ASM-001", register.ASM, register.Superseded, "Old assumption", nil),
		e("NG-001", register.NG, register.Confirmed, "No mobile app in v1", nil),
		e("DEC-012", register.DEC, register.Confirmed, "Database for orders", map[string]string{"chose": "Postgres", "over": "DynamoDB", "because": "orders are relational"}),
		e("RISK-003", register.RISK, register.Open, "No backup for the DB", map[string]string{"likelihood": "medium", "impact": "high", "mitigation": "add a backup vault"}),
		e("Q-001", register.Q, register.Open, "Which payment provider?", nil),
	}
	out := string(HTML(Build(in), ""))
	for _, want := range []string{"Customers can track an order", "Data stays in the EU", "Orders are paid by card", "No mobile app in v1", "ADR-0012: Database for orders", "No backup for the DB", "medium / high", "Which payment provider?", "(unconfirmed)", "1 assumptions in 2.2 are not confirmed"} {
		if !strings.Contains(out, want) {
			t.Errorf("arc42 is missing %q", want)
		}
	}
	for _, gone := range []string{"Old assumption", "Orders are under 1 TB", "ADR-001:"} {
		if strings.Contains(out, gone) {
			t.Errorf("with a Register, arc42 shouldn't show %q (superseded, or the Rationale's copy)", gone)
		}
	}
}
