package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/vinaayakh/designer/internal/app"
	"github.com/vinaayakh/designer/internal/brief"
	"github.com/vinaayakh/designer/internal/kb"
)

// state is everything the UI needs on load.
func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	a := s.A
	meta, err := a.P.Meta()
	if err != nil {
		fail(w, 500, err)
		return
	}
	b, err := a.Brief()
	if err != nil {
		fail(w, 500, err)
		return
	}
	vs, _ := a.P.Versions()
	reports, _ := a.P.Reports()
	writeJSON(w, map[string]any{
		"project":     map[string]any{"name": meta.Name, "dir": a.P.Dir, "created": meta.Created, "frozenAt": meta.FrozenAt},
		"brief":       b,
		"briefDone":   b.Complete(),
		"briefSchema": brief.TheSchema(),
		"catalog":     a.Cat,
		"kbVersion":   a.KB.Version,
		"versions":    vs,
		"reports":     reports,
		"pending":     a.PendingProposal(),
		"assistant":   a.Status(),
		"settings":    a.Settings(),
		"trusted":     app.TrustedList(),
		"view":        a.DefaultView(),
	})
}

func kbOpts(r *http.Request) kb.SearchOpts {
	q := r.URL.Query()
	return kb.SearchOpts{Type: q.Get("type"), Regime: q.Get("regime"), Limit: 10}
}

// static serves the built frontend with an SPA fallback. Hashed assets are
// cached forever; index.html never is.
func (s *Server) static() http.Handler {
	files := http.FileServer(http.FS(s.Web))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(s.Web, p); err != nil {
			p = "index.html"
			r.URL.Path = "/"
		}
		if strings.HasPrefix(p, "assets/") || strings.HasPrefix(p, "fonts/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-store")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		files.ServeHTTP(w, r)
	})
}
