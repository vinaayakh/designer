// Package server is Designer's local HTTP server: the JSON API and event
// stream for the browser UI, the MCP endpoint and hook for Claude Code, and
// the embedded frontend.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vinaayakh/designer/internal/app"
	"github.com/vinaayakh/designer/internal/brief"
	"github.com/vinaayakh/designer/internal/obs"
	"github.com/vinaayakh/designer/internal/project"
	"github.com/vinaayakh/designer/internal/report"
	"github.com/vinaayakh/designer/internal/scene"
)

type Server struct {
	A    *app.App
	Web  fs.FS // built frontend
	Addr string
}

const maxBody = 32 << 20

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func read(r *http.Request, v any) error {
	b, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// Listen binds to 127.0.0.1 (preferred port first, then any free port).
func Listen(port int) (net.Listener, error) {
	if port > 0 {
		if ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err == nil {
			return ln, nil
		}
	}
	return net.Listen("tcp", "127.0.0.1:0")
}

// Handler builds the routes.
func (s *Server) Handler() http.Handler {
	a := s.A
	mux := http.NewServeMux()

	mux.Handle("/mcp", a.MCPHandler())
	mux.HandleFunc("POST /hook", a.ServeHook)

	api := http.NewServeMux()
	api.HandleFunc("GET /api/state", s.state)
	api.HandleFunc("GET /api/events", s.events)

	api.HandleFunc("PUT /api/project", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Name string }
		if err := read(r, &in); err != nil || strings.TrimSpace(in.Name) == "" {
			fail(w, 400, errors.New("name is required"))
			return
		}
		if err := a.P.SetName(strings.TrimSpace(in.Name)); err != nil {
			fail(w, 500, err)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	})

	api.HandleFunc("PUT /api/brief", func(w http.ResponseWriter, r *http.Request) {
		var b brief.Brief
		if err := read(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		if b.Answers == nil {
			fail(w, 400, errors.New("answers missing"))
			return
		}
		if latest, _ := a.P.Latest(); latest > 0 {
			if cur, err := a.Brief(); err == nil && cur.Get("cloud") != b.Answers["cloud"] {
				fail(w, 409, errors.New("the target cloud can't change after Conversion: the Production Diagram uses its Services"))
				return
			}
		}
		if err := a.P.SaveBrief(&b); err != nil {
			fail(w, 400, err)
			return
		}
		a.Notify("brief.saved", map[string]any{"client": r.Header.Get("X-Client")})
		writeJSON(w, map[string]any{"ok": true, "complete": b.Complete(), "compact": b.Compact()})
	})

	api.HandleFunc("GET /api/rough", func(w http.ResponseWriter, r *http.Request) {
		b, err := a.P.RoughDraft()
		if err != nil {
			fail(w, 500, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(b)
	})
	api.HandleFunc("PUT /api/rough", func(w http.ResponseWriter, r *http.Request) {
		if p := a.PendingProposal(); p != nil && p.View == app.ViewRough && p.Kind == "proposal" {
			fail(w, 409, fmt.Errorf("the canvas is locked while Proposal %s waits for review", p.ID))
			return
		}
		b, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
		if err != nil {
			fail(w, 400, err)
			return
		}
		if err := a.P.SaveRoughDraft(b); err != nil {
			code := 400
			if errors.Is(err, project.ErrFrozen) {
				code = 409
			}
			fail(w, code, err)
			return
		}
		a.Notify("scene.saved", map[string]any{"view": "rough", "client": r.Header.Get("X-Client")})
		writeJSON(w, map[string]bool{"ok": true})
	})

	api.HandleFunc("GET /api/versions", func(w http.ResponseWriter, r *http.Request) {
		vs, err := a.P.Versions()
		if err != nil {
			fail(w, 500, err)
			return
		}
		writeJSON(w, vs)
	})
	api.HandleFunc("GET /api/versions/{n}", func(w http.ResponseWriter, r *http.Request) {
		n, err := strconv.Atoi(r.PathValue("n"))
		if err != nil {
			fail(w, 400, err)
			return
		}
		v, sc, err := a.P.Version(n)
		if err != nil {
			fail(w, 404, err)
			return
		}
		writeJSON(w, map[string]any{"version": v, "scene": json.RawMessage(sc)})
	})
	// Hand edits, Tidy up and restoring an older version save a new version.
	api.HandleFunc("POST /api/versions", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Scene   json.RawMessage `json:"scene"`
			Source  string          `json:"source"`
			Summary string          `json:"summary"`
			Base    int             `json:"base"`
		}
		if err := read(r, &in); err != nil {
			fail(w, 400, err)
			return
		}
		switch in.Source {
		case "edit", "tidy", "restore":
		default:
			fail(w, 400, errors.New("source must be edit, tidy or restore"))
			return
		}
		if p := a.PendingProposal(); p != nil && p.View == app.ViewProduction {
			fail(w, 409, fmt.Errorf("the canvas is locked while Proposal %s waits for review", p.ID))
			return
		}
		latest, _ := a.P.Latest()
		if latest == 0 {
			fail(w, 409, errors.New("there is no Production Diagram yet"))
			return
		}
		if in.Base != 0 && in.Base != latest && in.Source != "restore" {
			fail(w, 409, fmt.Errorf("the diagram changed (now v%d); reload", latest))
			return
		}
		if _, err := scene.Parse(in.Scene, true); err != nil {
			fail(w, 400, err)
			return
		}
		v, err := a.P.SaveVersion(in.Scene, in.Source, in.Summary, "", nil)
		if err != nil {
			fail(w, 500, err)
			return
		}
		a.Notify("scene.saved", map[string]any{"view": "production", "version": v.N, "client": r.Header.Get("X-Client")})
		writeJSON(w, v)
	})

	api.HandleFunc("GET /api/graph/{view}", func(w http.ResponseWriter, r *http.Request) {
		g, ver, err := a.Graph(r.PathValue("view"))
		if err != nil {
			fail(w, 404, err)
			return
		}
		writeJSON(w, map[string]any{"graph": g, "version": ver})
	})

	api.HandleFunc("GET /api/reports", func(w http.ResponseWriter, r *http.Request) {
		refs, err := a.P.Reports()
		if err != nil {
			fail(w, 500, err)
			return
		}
		writeJSON(w, refs)
	})
	api.HandleFunc("GET /api/reports/{file}", func(w http.ResponseWriter, r *http.Request) {
		b, err := a.P.Report(r.PathValue("file"))
		if err != nil {
			fail(w, 404, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	})

	api.HandleFunc("GET /api/kb/search", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.KB.Search(r.URL.Query().Get("q"), kbOpts(r)))
	})
	api.HandleFunc("GET /api/kb/records/{id}", func(w http.ResponseWriter, r *http.Request) {
		rec := a.KB.Get(r.PathValue("id"))
		if rec == nil {
			fail(w, 404, errors.New("no such Record"))
			return
		}
		writeJSON(w, rec)
	})

	api.HandleFunc("GET /api/precheck", func(w http.ResponseWriter, r *http.Request) {
		issues, err := a.PreCheck()
		if err != nil {
			fail(w, 500, err)
			return
		}
		writeJSON(w, issues)
	})
	api.HandleFunc("POST /api/convert", s.action(func(r *http.Request) error { return a.Convert() }))
	api.HandleFunc("POST /api/gapcheck", s.action(func(r *http.Request) error {
		var in struct{ Version int }
		if err := read(r, &in); err != nil {
			return err
		}
		if in.Version == 0 {
			in.Version, _ = a.P.Latest()
		}
		return a.GapCheck(in.Version)
	}))
	api.HandleFunc("POST /api/fix", s.action(func(r *http.Request) error {
		var in struct {
			Report  string   `json:"report"`
			Records []string `json:"records"`
		}
		if err := read(r, &in); err != nil {
			return err
		}
		return a.Fix(in.Report, in.Records)
	}))

	// arc42 document: its AI-written text, then the export itself.
	api.HandleFunc("GET /api/document/{n}", func(w http.ResponseWriter, r *http.Request) {
		n, err := strconv.Atoi(r.PathValue("n"))
		if err != nil {
			fail(w, 400, err)
			return
		}
		written, stale, err := a.DocumentStatus(n)
		if err != nil {
			fail(w, 404, err)
			return
		}
		writeJSON(w, map[string]bool{"written": written, "stale": stale})
	})
	api.HandleFunc("POST /api/document", s.action(func(r *http.Request) error {
		var in struct{ Version int }
		if err := read(r, &in); err != nil {
			return err
		}
		return a.WriteDocument(in.Version)
	}))
	api.HandleFunc("POST /api/export/{n}/{format}", func(w http.ResponseWriter, r *http.Request) {
		n, err := strconv.Atoi(r.PathValue("n"))
		if err != nil {
			fail(w, 400, err)
			return
		}
		var in struct {
			SVG string `json:"svg"`
			PNG []byte `json:"png"` // base64 in JSON
		}
		if err := read(r, &in); err != nil {
			fail(w, 400, err)
			return
		}
		format := r.PathValue("format")
		t := obs.Start("export.input", "version", n)
		din, err := a.DocumentInput(n)
		t.End(err)
		if err != nil {
			fail(w, 404, err)
			return
		}
		t = obs.Start("export.build", "version", n, "nodes", len(din.Graph.Nodes), "edges", len(din.Graph.Edges))
		doc := report.Build(din)
		t.End(nil)
		t = obs.Start("export."+format, "version", n, "svg_bytes", len(in.SVG), "png_bytes", len(in.PNG))
		defer func() { t.End(nil) }()
		name := fmt.Sprintf("%s-v%d-arc42", fileSafe(din.Project), n)
		switch format {
		case "docx":
			out, err := report.DOCX(doc, in.PNG)
			if err != nil {
				fail(w, 400, err)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.docx"`, name))
			_, _ = w.Write(out)
		case "html":
			if in.SVG != "" && !strings.HasPrefix(strings.TrimSpace(in.SVG), "<svg") {
				fail(w, 400, errors.New("svg must be an SVG document"))
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s.html"`, name))
			_, _ = w.Write(report.HTML(doc, in.SVG))
		default:
			fail(w, 400, errors.New("format must be docx or html"))
		}
	})

	// Observability: this run's metrics, and timings and errors from the browser.
	api.HandleFunc("GET /api/metrics", func(w http.ResponseWriter, r *http.Request) {
		m := obs.M.Snapshot()
		m["assistant"] = a.Status()
		writeJSON(w, m)
	})
	api.HandleFunc("POST /api/clientlog", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Entries []struct {
				Level string         `json:"level"`
				Msg   string         `json:"msg"`
				Name  string         `json:"name"`
				Ms    float64        `json:"ms"`
				Slow  bool           `json:"slow"`
				Data  map[string]any `json:"data"`
			} `json:"entries"`
		}
		if err := read(r, &in); err != nil {
			fail(w, 400, err)
			return
		}
		client := r.Header.Get("X-Client")
		for i, e := range in.Entries {
			if i == 200 {
				slog.Warn("client log batch truncated", "client", client, "entries", len(in.Entries))
				break
			}
			lvl := slog.LevelDebug
			switch e.Level {
			case "info":
				lvl = slog.LevelInfo
			case "warn":
				lvl = slog.LevelWarn
			case "error":
				lvl = slog.LevelError
			}
			attrs := []any{"source", "web", "client", client}
			if e.Name != "" {
				attrs = append(attrs, "name", e.Name)
			}
			if e.Ms > 0 {
				attrs = append(attrs, "ms", e.Ms)
				obs.M.ObserveSlow("client", e.Name, time.Duration(e.Ms*float64(time.Millisecond)), lvl >= slog.LevelError, e.Slow)
			}
			for _, k := range sortedKeys(e.Data) {
				attrs = append(attrs, k, e.Data[k])
			}
			slog.Log(r.Context(), lvl, truncate(e.Msg, 300), attrs...)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	api.HandleFunc("GET /api/assistant", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, a.Status()) })
	api.HandleFunc("POST /api/assistant/warm", s.action(func(r *http.Request) error { return a.Warm() }))
	api.HandleFunc("POST /api/assistant/chat", s.action(func(r *http.Request) error {
		var in struct {
			Text      string   `json:"text"`
			View      string   `json:"view"`
			Selection []string `json:"selection"`
		}
		if err := read(r, &in); err != nil {
			return err
		}
		return a.Chat(in.Text, in.View, in.Selection)
	}))
	api.HandleFunc("POST /api/assistant/interrupt", s.action(func(r *http.Request) error { return a.Interrupt() }))
	api.HandleFunc("POST /api/assistant/retry", s.action(func(r *http.Request) error { return a.Retry() }))
	api.HandleFunc("POST /api/assistant/reset", s.action(func(r *http.Request) error { a.ResetSession(); return nil }))

	api.HandleFunc("GET /api/proposal", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, a.PendingProposal()) })
	api.HandleFunc("POST /api/proposal/simulate", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Accepted []string `json:"accepted"`
		}
		if err := read(r, &in); err != nil {
			fail(w, 400, err)
			return
		}
		base, result, err := a.SimulatePending(in.Accepted)
		if err != nil {
			fail(w, 409, err)
			return
		}
		writeJSON(w, map[string]any{"base": base, "graph": result})
	})
	api.HandleFunc("POST /api/proposal/decision", func(w http.ResponseWriter, r *http.Request) {
		var d app.Decision
		if err := read(r, &d); err != nil {
			fail(w, 400, err)
			return
		}
		res, err := a.Decide(d)
		if err != nil {
			fail(w, 409, err)
			return
		}
		if res.Version > 0 {
			a.Notify("scene.saved", map[string]any{"view": "production", "version": res.Version, "client": r.Header.Get("X-Client")})
		} else if d.Action == "accept" {
			a.Notify("scene.saved", map[string]any{"view": "rough", "client": r.Header.Get("X-Client")})
		}
		writeJSON(w, res)
	})

	api.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, a.Settings()) })
	api.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var st app.Settings
		if err := read(r, &st); err != nil {
			fail(w, 400, err)
			return
		}
		for _, m := range []string{st.Model, st.ConversionModel} {
			switch m {
			case "", "opus", "sonnet", "haiku":
			default:
				fail(w, 400, fmt.Errorf("unknown model %q", m))
				return
			}
		}
		if err := a.SetSettings(st); err != nil {
			fail(w, 500, err)
			return
		}
		writeJSON(w, st)
	})

	mux.Handle("/api/", s.guard(api))
	mux.Handle("/", s.static())
	return obs.Middleware(mux)
}

// guard protects the browser API from other websites: the Host must be our
// own loopback address (DNS rebinding), and a cross-origin request is refused.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, port, err := net.SplitHostPort(r.Host)
		_, myPort, _ := net.SplitHostPort(s.Addr)
		if err != nil || port != myPort || (host != "127.0.0.1" && host != "localhost") {
			fail(w, 403, errors.New("forbidden host"))
			return
		}
		if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host {
			fail(w, 403, errors.New("cross-origin request refused"))
			return
		}
		if r.Method != http.MethodGet && r.Header.Get("X-Designer") != "1" {
			fail(w, 403, errors.New("missing X-Designer header"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// fileSafe makes a Project name usable in a download's file name.
func fileSafe(name string) string {
	out := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		case r == ' ' || r == '.':
			return '-'
		}
		return -1
	}, name)
	if out == "" {
		return "design"
	}
	return out
}

func (s *Server) action(fn func(r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(r); err != nil {
			code := 400
			if errors.Is(err, app.ErrBusy) {
				code = 409
			}
			fail(w, code, err)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		fail(w, 500, errors.New("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	ch, done := s.A.Subscribe()
	defer done()
	fmt.Fprintf(w, "data: %s\n\n", `{"type":"hello"}`)
	fl.Flush()
	tick := time.NewTicker(25 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case b := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
		case <-tick.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			fl.Flush()
		}
	}
}
