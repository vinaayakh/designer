// Package report builds the arc42 architecture document of a Production
// Diagram version and renders it as Word (.docx) or as a print-styled HTML
// page (saved as PDF from the browser).
//
// Build turns the stored data (Design Brief, diagram, Design Rationale, Gap
// Report, Knowledge Base) plus the AI-written Narrative into a small document
// model; the renderers only lay that model out, so both formats always say
// the same thing.
package report

import "strings"

// Doc is a whole document.
type Doc struct {
	Title    string
	Subtitle string
	Meta     [][2]string // cover page facts: label, value
	Notice   []Run       // shown on the cover (e.g. "not an attestation")
	Sections []*Section
}

// Section is an arc42 section or subsection. Num is "1", "1.2", "A"...
type Section struct {
	Num       string
	Title     string
	Blocks    []Block
	Sub       []*Section
	Landscape bool // the diagram: its own landscape page
	PageBreak bool // starts on a new page
}

func (s *Section) add(b ...Block) *Section { s.Blocks = append(s.Blocks, b...); return s }

func (s *Section) sub(num, title string) *Section {
	c := &Section{Num: num, Title: title}
	s.Sub = append(s.Sub, c)
	return c
}

// Run is a piece of inline text.
type Run struct {
	Text   string
	Bold   bool
	Italic bool
	Mono   bool
	Link   string // URL
	Muted  bool
}

// Block is one of Para, List, Table, Callout, Diagram.
type Block interface{ block() }

type Para struct {
	Runs  []Run
	Small bool
}

type List struct {
	Ordered bool
	Items   [][]Run
}

// Table: Cells are paragraphs of runs; a "\n" in a run's text is a line break.
type Table struct {
	Head    []string
	Rows    [][][]Run
	Widths  []int // relative column widths
	Compact bool  // smaller type (long checklists)
}

// Callout is a highlighted note: Tone is "info" or "warn".
type Callout struct {
	Tone string
	Runs []Run
}

// Diagram is the exported image of the diagram itself.
type Diagram struct{ Caption string }

func (Para) block()    {}
func (List) block()    {}
func (Table) block()   {}
func (Callout) block() {}
func (Diagram) block() {}

// ---- helpers for building

func T(s string) Run         { return Run{Text: s} }
func B(s string) Run         { return Run{Text: s, Bold: true} }
func I(s string) Run         { return Run{Text: s, Italic: true} }
func M(s string) Run         { return Run{Text: s, Mono: true} }
func L(s, url string) Run    { return Run{Text: s, Link: url} }
func Muted(s string) Run     { return Run{Text: s, Muted: true} }
func P(runs ...Run) Para     { return Para{Runs: runs} }
func Ptext(s string) Para    { return Para{Runs: []Run{T(s)}} }
func cell(runs ...Run) []Run { return runs }
func ctext(s string) []Run   { return []Run{T(s)} }

// Plain flattens runs to text (tests, titles).
func Plain(runs []Run) string {
	var sb strings.Builder
	for _, r := range runs {
		sb.WriteString(r.Text)
	}
	return sb.String()
}

// walk visits every section in document order.
func (d *Doc) walk(fn func(s *Section, depth int)) {
	var rec func(ss []*Section, depth int)
	rec = func(ss []*Section, depth int) {
		for _, s := range ss {
			fn(s, depth)
			rec(s.Sub, depth+1)
		}
	}
	rec(d.Sections, 0)
}
