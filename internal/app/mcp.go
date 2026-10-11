package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vinaayakh/designer/internal/catalog"
	"github.com/vinaayakh/designer/internal/kb"
	"github.com/vinaayakh/designer/internal/obs"
)

func obj(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

func arr(desc string, items map[string]any) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": items}
}

type tool struct {
	name, desc string
	schema     map[string]any
	fn         func(args json.RawMessage) (string, error)
}

// tools are the 6 tools from tickets 011/012, plus submit_document for the arc42 export.
func (a *App) tools() []tool {
	changeSchema := map[string]any{"type": "object", "description": "One operation; see the system prompt for the fields of each op.",
		"properties": map[string]any{
			"id": str("c1, c2, …"), "op": str("add_node | add_group | add_edge | update_node | update_group | update_edge | move_node | remove_node | remove_group | remove_edge"),
			"ref": str("alias of an existing Node/Group, or +name for a new one"), "label": str(""), "kind": str("Component Kind or group kind"),
			"service": str("Cloud Catalog service id"), "service_name": str("off-catalog service name"),
			"classes": arr("Data Classification", map[string]any{"type": "string"}), "tier": str("subnet tier"),
			"parent": map[string]any{"type": []string{"string", "null"}, "description": "Group alias or +ref; null = top level"},
			"from":   str(""), "to": str(""), "style": str("sync | async | stream"), "protocol": str(""),
			"set": map[string]any{"type": "object", "description": "fields to change"},
			"why": str("one-line reason"), "practices": arr("Knowledge Base practice ids applied", map[string]any{"type": "string"}),
		}, "required": []string{"op"}}
	return []tool{
		{"get_diagram", "Get a whole diagram in the compact format. The current one is already in your messages; use this only if you need it again in full.",
			obj(map[string]any{"view": str("rough | production (default: the Production Diagram once it exists)")}),
			func(raw json.RawMessage) (string, error) {
				var in struct{ View string }
				_ = json.Unmarshal(raw, &in)
				if in.View == "" {
					in.View = a.DefaultView()
				}
				text, ver, err := a.fullDiagramLocked(in.View)
				if err != nil {
					return "", err
				}
				if in.View == ViewProduction {
					return fmt.Sprintf("Production Diagram v%d\n%s", ver, text), nil
				}
				return "Rough Draft\n" + text, nil
			}},
		{"get_nodes", "Details for a few Nodes or Groups: their line, Edges, Design Rationale entry and the Knowledge Base Records that apply to them.",
			obj(map[string]any{"aliases": arr("e.g. [\"n3\", \"g2\"]", map[string]any{"type": "string"}), "view": str("rough | production")}, "aliases"),
			a.getNodes},
		{"search_kb", "Search the curated Knowledge Base (design and security practices, SOC 2 and HIPAA controls). Returns up to 5: id | title | summary. Always try this before the web.",
			obj(map[string]any{"query": str("keywords or a question"), "type": str("control | practice (optional)"), "regime": str("soc2 | hipaa (optional)")}, "query"),
			func(raw json.RawMessage) (string, error) {
				var in struct{ Query, Type, Regime string }
				if err := json.Unmarshal(raw, &in); err != nil {
					return "", err
				}
				b, _ := a.Brief()
				o := kb.SearchOpts{Type: in.Type, Regime: in.Regime}
				if b != nil {
					o.BriefMatch = b.Matches
				}
				hits := a.KB.Search(in.Query, o)
				if len(hits) == 0 {
					return "No Knowledge Base Records match. You may search the trusted web sources.", nil
				}
				var sb strings.Builder
				for _, h := range hits {
					fmt.Fprintf(&sb, "%s | %s | %s\n", h.ID, h.Title, h.Summary)
				}
				return sb.String(), nil
			}},
		{"get_record", "The full text of one Knowledge Base Record by id.",
			obj(map[string]any{"id": str("Record id, e.g. practice.encryption-at-rest")}, "id"),
			func(raw json.RawMessage) (string, error) {
				var in struct{ ID string }
				_ = json.Unmarshal(raw, &in)
				r := a.KB.Get(in.ID)
				if r == nil {
					return "", fmt.Errorf("no Record %q (use search_kb)", in.ID)
				}
				var sb strings.Builder
				fmt.Fprintf(&sb, "%s (%s) %s\ncoverage: %s\n", r.ID, r.Type, r.Title, r.Coverage)
				if len(r.SatisfiedBy) > 0 {
					fmt.Fprintf(&sb, "satisfied by: %s\n", strings.Join(r.SatisfiedBy, ", "))
				}
				if len(r.Satisfies) > 0 {
					fmt.Fprintf(&sb, "satisfies: %s\n", strings.Join(r.Satisfies, ", "))
				}
				sb.WriteString("\n" + r.Body + "\n")
				return sb.String(), nil
			}},
		{"submit_proposal", "Submit a Proposal (or, during the Conversion Design task, the whole design with kind \"conversion\"). Nothing changes until the user approves. Returns every problem found so you can fix them in one go.",
			obj(map[string]any{
				"title": str("short title"), "summary": str("one or two sentences"),
				"kind": str("omit for a normal Proposal; \"conversion\" only in the Conversion Design task"),
				"view": str("rough | production"), "base_version": map[string]any{"type": "integer", "description": "the Production version you saw"},
				"addresses": arr("Gap Report items fixed, e.g. [\"gap:hipaa.164-312-a-2-iv\"]", map[string]any{"type": "string"}),
				"changes":   arr("the operations", changeSchema),
				"rationale": map[string]any{"type": "object", "description": "Design Rationale (required for a conversion): {summary, assumptions[], nodes:{alias|+ref:{why, practices[], cites[]}}, tradeoffs[]}"},
			}, "title", "changes"),
			a.submitProposalLocked},
		{"submit_gap_report", "Submit the Gap Report during a Gap Check task.",
			obj(map[string]any{
				"version":         map[string]any{"type": "integer"},
				"items":           arr("one per Control listed in the task", map[string]any{"type": "object"}),
				"recommendations": arr("security practices that matter here and aren't applied", map[string]any{"type": "object"}),
			}, "version", "items"),
			a.submitGapReportLocked},
		{"submit_document", "Submit the arc42 document text during a document task.",
			obj(map[string]any{
				"version":    map[string]any{"type": "integer"},
				"overview":   str("2–4 sentences"),
				"components": map[string]any{"type": "object", "description": "node alias → 1–4 responsibilities (strings); every Node"},
				"groups":     map[string]any{"type": "object", "description": "group alias → one-sentence purpose"},
				"scenarios":  arr("2–5 runtime flows: {title, summary, steps:[{from, to, what}]}; steps follow Edges", map[string]any{"type": "object"}),
				"concepts":   arr("crosscutting concepts: {topic, text, nodes[]}", map[string]any{"type": "object"}),
				"quality":    arr("quality scenarios: {attribute, requirement, scenario, response, nodes[]}", map[string]any{"type": "object"}),
			}, "version", "overview", "components", "scenarios"),
			a.submitDocumentLocked},
		{"submit_step", "Submit a Document Pipeline document during a Step task: its written sections, new Register entries (with +refs) and citations.",
			obj(map[string]any{
				"step":     str("the Step ID from the task"),
				"sections": map[string]any{"type": "object", "description": "section id → {body} | {items: [...]} | {rows: [[...]]}, optional cites"},
				"entries":  arr("new Register entries: {ref: \"+name\", type, statement, fields, links: [aliases]}", map[string]any{"type": "object"}),
				"cites":    arr("entry IDs or +refs the document relies on beyond those in the text", map[string]any{"type": "string"}),
			}, "step", "sections"),
			a.submitStepLocked},
	}
}

func (a *App) getNodes(raw json.RawMessage) (string, error) {
	var in struct {
		Aliases []string
		View    string
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	if in.View == "" {
		in.View = a.DefaultView()
	}
	g, ver, err := a.Graph(in.View)
	if err != nil {
		return "", err
	}
	g.Compactless(a.sess.Aliases)
	var rat struct {
		Nodes map[string]json.RawMessage `json:"nodes"`
	}
	if in.View == ViewProduction {
		if v, _, err := a.P.Version(ver); err == nil && len(v.Rationale) > 0 {
			_ = json.Unmarshal(v.Rationale, &rat)
		}
	}
	b, _ := a.Brief()
	var sb strings.Builder
	for _, al := range in.Aliases {
		id, ok := a.sess.Aliases.ID(al)
		line := ""
		if ok {
			line = g.Line(id, a.sess.Aliases)
		}
		if line == "" {
			fmt.Fprintf(&sb, "%s: not on this diagram\n", al)
			continue
		}
		sb.WriteString(line + "\n")
		for _, e := range g.EdgesOf(id, a.sess.Aliases) {
			sb.WriteString("  " + e + "\n")
		}
		if r, ok := rat.Nodes[id]; ok {
			fmt.Fprintf(&sb, "  rationale: %s\n", r)
		}
		if n := g.Node(id); n != nil && n.Kind != "" && b != nil {
			controls, practices := a.KB.Applies([]kb.Node{{Alias: al, Kind: n.Kind, DataClasses: n.Classes}}, b.Get("cloud"), b.Matches, catalog.MatchKind)
			var ids []string
			for _, x := range append(controls, practices...) {
				if len(x.Nodes) > 0 {
					ids = append(ids, x.Record.ID)
				}
			}
			if len(ids) > 0 {
				fmt.Fprintf(&sb, "  records: %s\n", strings.Join(ids, ", "))
			}
		}
	}
	return sb.String(), nil
}

// logTool records one MCP tool call. A submit tool's validation errors are
// normal (the AI fixes and resubmits), so they're warnings, not errors.
func logTool(name string, inBytes, outBytes int, wait, d time.Duration, err error) {
	slow := d > obs.Slow()
	obs.M.ObserveSlow("tool", name, d, err != nil, slow)
	attrs := []any{"tool", name, "ms", float64(d.Microseconds()) / 1000, "lock_wait_ms", float64(wait.Microseconds()) / 1000, "in_bytes", inBytes, "out_bytes", outBytes}
	switch {
	case err != nil:
		slog.Warn("tool call returned an error", append(attrs, "err", truncate(err.Error(), 600))...)
	case slow:
		slog.Warn("slow tool call", attrs...)
	case wait > 250*time.Millisecond:
		slog.Warn("tool call waited for the runtime lock", attrs...)
	default:
		slog.Debug("tool call", attrs...)
	}
}

// MCPHandler serves the tools over Streamable HTTP, behind the bearer token.
func (a *App) MCPHandler() http.Handler {
	srv := mcp.NewServer(&mcp.Implementation{Name: "designer", Version: "1.0.0"}, nil)
	for _, t := range a.tools() {
		t := t
		srv.AddTool(&mcp.Tool{Name: t.name, Description: t.desc, InputSchema: t.schema},
			func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				wait := time.Now()
				a.mu.Lock()
				start := time.Now()
				out, err := t.fn(req.Params.Arguments)
				a.mu.Unlock()
				logTool(t.name, len(req.Params.Arguments), len(out), start.Sub(wait), time.Since(start), err)
				if err != nil {
					return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: out}}}, nil
			})
	}
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}
