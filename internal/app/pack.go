package app

import (
	"archive/zip"
	"io"
	"os"
	"time"

	"github.com/vinaayakh/designer/internal/pipeline"
	"github.com/vinaayakh/designer/internal/register"
)

// DocumentPack writes the audit hand-off zip (wayfinder/doc-pipeline ticket 009): each Step's
// latest document as it was written, an ADR per decision, the Register as Markdown and its raw
// hash-chained log, and an index saying what each document was built from and whether it's stale.
func (a *App) DocumentPack(w io.Writer) error {
	now, err := a.pipelineNow()
	if err != nil {
		return err
	}
	m, _ := a.P.Meta()
	st := a.pipeStore()
	entries := a.Reg.Entries()
	integrity := a.Reg.Integrity()

	// Name linked elements by their labels on the latest diagram.
	g, _, _ := a.Graph(a.DefaultView())
	label := func(id string) string {
		if g == nil {
			return ""
		}
		if n := g.Node(id); n != nil {
			return n.Label
		}
		if gr := g.Group(id); gr != nil {
			return gr.Label
		}
		return ""
	}

	var files []pipeline.File
	var docs []pipeline.PackDoc
	for _, s := range pipeline.Steps() {
		d := pipeline.PackDoc{Step: s, Version: now.Latest[s.ID], Status: pipeline.StatusOf(s, now)}
		docs = append(docs, d)
		if d.Version == nil {
			continue
		}
		md, err := st.Markdown(s.ID, d.Version.N)
		if err != nil {
			md = []byte(pipeline.Markdown(s, d.Version, m.Name, entries))
		}
		files = append(files, pipeline.File{Name: pipeline.DocName(d), Data: md})
	}
	byID := map[string]register.Entry{}
	for _, e := range entries {
		byID[e.ID] = e
	}
	for _, e := range entries {
		if e.Type == register.DEC {
			files = append(files, pipeline.File{Name: pipeline.ADRName(e), Data: []byte(pipeline.ADR(e, byID, label))})
		}
	}
	files = append(files, pipeline.File{Name: "register.md", Data: []byte(pipeline.RegisterMarkdown(m.Name, entries, integrity, label))})
	if log, err := os.ReadFile(a.P.RegisterPath()); err == nil {
		files = append(files, pipeline.File{Name: "register/log.jsonl", Data: log})
	}
	files = append([]pipeline.File{{Name: "README.md", Data: []byte(pipeline.Index(m.Name, docs, entries, integrity, time.Now()))}}, files...)

	zw := zip.NewWriter(w)
	for _, f := range files {
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			return err
		}
		if _, err := fw.Write(f.Data); err != nil {
			return err
		}
	}
	return zw.Close()
}
