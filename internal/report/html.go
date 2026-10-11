package report

import (
	"fmt"
	"html"
	"strings"
)

// HTML renders the document as one self-contained, print-styled page. svg is
// the diagram (inlined as-is; it comes from the user's own canvas export).
func HTML(d *Doc, svg string) []byte {
	var sb strings.Builder
	esc := html.EscapeString
	fmt.Fprintf(&sb, `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s – arc42</title>
<style>%s</style></head><body>
`, esc(d.Title), css)

	// Cover
	sb.WriteString(`<section class="cover">`)
	fmt.Fprintf(&sb, `<p class="kicker">Architecture documentation</p><h1>%s</h1><p class="subtitle">%s</p><table class="meta">`, esc(d.Title), esc(d.Subtitle))
	for _, m := range d.Meta {
		fmt.Fprintf(&sb, `<tr><th>%s</th><td>%s</td></tr>`, esc(m[0]), esc(m[1]))
	}
	sb.WriteString(`</table>`)
	if len(d.Notice) > 0 {
		fmt.Fprintf(&sb, `<div class="callout info">%s</div>`, runsHTML(d.Notice))
	}
	sb.WriteString(`<p class="template">Structure based on the arc42 template (arc42.org).</p></section>`)

	// Contents
	sb.WriteString(`<nav class="toc"><h2>Contents</h2><ol>`)
	d.walk(func(s *Section, depth int) {
		if depth > 1 {
			return
		}
		fmt.Fprintf(&sb, `<li class="d%d"><a href="#s-%s"><span class="num">%s</span>%s</a></li>`, depth, anchor(s.Num), esc(s.Num), esc(s.Title))
	})
	sb.WriteString(`</ol></nav>`)

	var rec func(s *Section, depth int)
	rec = func(s *Section, depth int) {
		cls := []string{"sec"}
		if s.PageBreak {
			cls = append(cls, "break")
		}
		if s.Landscape {
			cls = append(cls, "landscape")
		}
		fmt.Fprintf(&sb, `<section class="%s" id="s-%s">`, strings.Join(cls, " "), anchor(s.Num))
		h := min(depth+2, 4)
		fmt.Fprintf(&sb, `<h%d><span class="num">%s</span>%s</h%d>`, h, esc(s.Num), esc(s.Title), h)
		for _, b := range s.Blocks {
			blockHTML(&sb, b, svg)
		}
		for _, c := range s.Sub {
			rec(c, depth+1)
		}
		sb.WriteString(`</section>`)
	}
	for _, s := range d.Sections {
		rec(s, 0)
	}
	sb.WriteString("</body></html>\n")
	return []byte(sb.String())
}

func anchor(num string) string { return strings.ReplaceAll(num, ".", "-") }

func blockHTML(sb *strings.Builder, b Block, svg string) {
	switch v := b.(type) {
	case Para:
		cls := ""
		if v.Small {
			cls = ` class="small"`
		}
		fmt.Fprintf(sb, `<p%s>%s</p>`, cls, runsHTML(v.Runs))
	case List:
		tag := "ul"
		if v.Ordered {
			tag = "ol"
		}
		fmt.Fprintf(sb, "<%s>", tag)
		for _, it := range v.Items {
			fmt.Fprintf(sb, "<li>%s</li>", runsHTML(it))
		}
		fmt.Fprintf(sb, "</%s>", tag)
	case Table:
		cls := "grid"
		if v.Compact {
			cls += " compact"
		}
		head := false
		for _, h := range v.Head {
			head = head || h != ""
		}
		if !head {
			cls += " facts"
		}
		fmt.Fprintf(sb, `<table class="%s"><colgroup>`, cls)
		for _, w := range v.Widths {
			fmt.Fprintf(sb, `<col style="width:%d%%">`, w)
		}
		sb.WriteString(`</colgroup>`)
		if head {
			sb.WriteString(`<thead><tr>`)
			for _, h := range v.Head {
				fmt.Fprintf(sb, `<th>%s</th>`, html.EscapeString(h))
			}
			sb.WriteString(`</tr></thead>`)
		}
		sb.WriteString(`<tbody>`)
		for _, r := range v.Rows {
			sb.WriteString(`<tr>`)
			for _, c := range r {
				fmt.Fprintf(sb, `<td>%s</td>`, runsHTML(c))
			}
			sb.WriteString(`</tr>`)
		}
		sb.WriteString(`</tbody></table>`)
	case Callout:
		fmt.Fprintf(sb, `<div class="callout %s">%s</div>`, html.EscapeString(v.Tone), runsHTML(v.Runs))
	case Diagram:
		fmt.Fprintf(sb, `<figure class="diagram">%s<figcaption>%s</figcaption></figure>`, svg, html.EscapeString(v.Caption))
	}
}

func runsHTML(runs []Run) string {
	var sb strings.Builder
	for _, r := range runs {
		t := strings.ReplaceAll(html.EscapeString(r.Text), "\n", "<br>")
		if r.Mono {
			t = "<code>" + t + "</code>"
		}
		if r.Bold {
			t = "<strong>" + t + "</strong>"
		}
		if r.Italic {
			t = "<em>" + t + "</em>"
		}
		if r.Muted {
			t = `<span class="muted">` + t + "</span>"
		}
		if r.Link != "" && (strings.HasPrefix(r.Link, "https://") || strings.HasPrefix(r.Link, "http://")) {
			t = fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(r.Link), t)
		}
		if strings.HasPrefix(r.Text, "✘") {
			t = `<span class="bad">` + t + "</span>"
		} else if strings.HasPrefix(r.Text, "✔") {
			t = `<span class="ok">` + t + "</span>"
		}
		sb.WriteString(t)
	}
	return sb.String()
}

// Same palette and type as the app: ink, one accent, colour only for status.
const css = `
:root{--ink:#1c1c21;--ink-2:#4a4a55;--muted:#7a7a86;--line:#e3e3ea;--soft:#f6f6f9;--accent:#5b4bd6;--ok:#1f7a4d;--bad:#b42318;--warn:#9a6700;--warn-bg:#fff8e6;--info-bg:#f3f1fd}
*{box-sizing:border-box}
html{background:#fff}
body{margin:0 auto;max-width:860px;padding:48px 32px;color:var(--ink);background:#fff;font:10.5pt/1.55 "Inter","Inter Variable","Segoe UI",system-ui,-apple-system,sans-serif;-webkit-print-color-adjust:exact;print-color-adjust:exact}
h1{font-size:30pt;line-height:1.15;letter-spacing:-.02em;margin:0 0 8px}
h2{font-size:17pt;letter-spacing:-.01em;margin:0 0 14px;padding-bottom:8px;border-bottom:1px solid var(--line)}
h3{font-size:12.5pt;margin:26px 0 8px}
h4{font-size:11pt;margin:22px 0 6px}
.num{color:var(--muted);font-weight:500;margin-right:.6em;font-variant-numeric:tabular-nums}
p{margin:0 0 10px}
.small{font-size:9pt}
.muted{color:var(--muted)}
.ok{color:var(--ok)}.bad{color:var(--bad)}
a{color:var(--accent);text-decoration:none}
code{font:9pt "JetBrains Mono","Cascadia Mono",Consolas,monospace;background:var(--soft);padding:1px 4px;border-radius:3px}
ul,ol{margin:0 0 12px;padding-left:22px}li{margin:2px 0}
table{width:100%;border-collapse:collapse;margin:6px 0 16px;table-layout:fixed}
th,td{text-align:left;vertical-align:top;padding:6px 8px;border-bottom:1px solid var(--line);overflow-wrap:anywhere}
thead th{font-size:8.5pt;font-weight:600;text-transform:uppercase;letter-spacing:.04em;color:var(--ink-2);border-bottom:1.5px solid var(--ink)}
table.compact{font-size:9pt}table.compact td{padding:4px 6px}
table.facts td:first-child{color:var(--muted)}
tr{break-inside:avoid}
.callout{border-radius:6px;padding:10px 14px;margin:8px 0 16px;font-size:9.5pt}
.callout.info{background:var(--info-bg)}.callout.warn{background:var(--warn-bg);color:#5c3d00}
.cover{min-height:85vh;display:flex;flex-direction:column;justify-content:center}
.kicker{text-transform:uppercase;letter-spacing:.12em;font-size:8.5pt;color:var(--accent);font-weight:600}
.subtitle{font-size:13pt;color:var(--ink-2);margin-bottom:28px}
table.meta{width:auto;margin-bottom:24px}table.meta th{color:var(--muted);font-weight:500;padding-right:28px;border:0}table.meta td{border:0}
.template{font-size:8.5pt;color:var(--muted)}
.toc ol{list-style:none;padding:0}.toc li{margin:3px 0}.toc li.d1{padding-left:28px;font-size:9.5pt}.toc a{color:var(--ink)}
.toc li.d0{margin-top:8px;font-weight:600}
section.sec{margin-bottom:28px}
figure.diagram{margin:8px 0}figure.diagram svg{width:100%;height:auto;max-height:165mm;display:block}
figcaption{font-size:9pt;color:var(--muted);margin-top:6px;text-align:center}
@page{size:A4;margin:18mm 16mm}
@page wide{size:A4 landscape;margin:12mm}
@media print{
  body{max-width:none;padding:0}
  .break,.toc{break-before:page}
  .cover{min-height:auto;padding-top:60mm}
  section.landscape{page:wide;break-before:page;break-after:page}
  h2,h3,h4{break-after:avoid}
  a{color:var(--ink)}
}
`
