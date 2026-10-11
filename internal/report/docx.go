package report

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"image/png"
	"strings"
	"time"
)

// Page geometry in twips (1/1440 inch). A4, 2 cm margins; the diagram's
// page is landscape with narrower margins.
const (
	pageW, pageH = 11906, 16838
	margin       = 1134
	textW        = pageW - 2*margin
	wideMargin   = 720
	wideTextW    = pageH - 2*wideMargin
	wideTextH    = pageW - 2*wideMargin - 1600 // room for the heading and caption
	emuPerTwip   = 635
	colorInk     = "1C1C21"
	colorMuted   = "7A7A86"
	colorAccent  = "5B4BD6"
	colorLine    = "E3E3EA"
	colorOK      = "1F7A4D"
	colorBad     = "B42318"
	colorInfo    = "F3F1FD"
	colorWarn    = "FFF8E6"
	docFont      = "Calibri"
	monoFont     = "Consolas"
)

// DOCX renders the document as a Word file. pngData is the diagram (may be
// nil: the figure is then left out).
func DOCX(d *Doc, pngData []byte) ([]byte, error) {
	w := &docxWriter{links: map[string]string{}}
	if len(pngData) > 0 {
		cfg, err := png.DecodeConfig(bytes.NewReader(pngData))
		if err != nil {
			return nil, fmt.Errorf("diagram image: %w", err)
		}
		w.imgW, w.imgH = cfg.Width, cfg.Height
	}
	w.body(d)

	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	files := []struct{ name, body string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", rootRels},
		{"docProps/core.xml", fmt.Sprintf(coreXML, xesc(d.Title+" – arc42"), time.Now().UTC().Format(time.RFC3339))},
		{"docProps/app.xml", appXML},
		{"word/document.xml", w.doc.String()},
		{"word/styles.xml", stylesXML},
		{"word/settings.xml", settingsXML},
		{"word/numbering.xml", w.numbering()},
		{"word/footer1.xml", fmt.Sprintf(footerXML, xesc(d.Title))},
		{"word/_rels/document.xml.rels", w.rels(len(pngData) > 0)},
	}
	for _, f := range files {
		fw, err := z.Create(f.name)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write([]byte(f.body)); err != nil {
			return nil, err
		}
	}
	if len(pngData) > 0 {
		fw, err := z.Create("word/media/diagram.png")
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(pngData); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type docxWriter struct {
	doc        strings.Builder
	links      map[string]string // url → relationship id
	linkOrder  []string
	ordered    int // ordered lists so far (each restarts at 1)
	imgW, imgH int
	figure     int
}

func xesc(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func (w *docxWriter) body(d *Doc) {
	w.doc.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><w:body>`)

	// Cover
	w.para(`<w:pStyle w:val="Kicker"/><w:spacing w:before="2400"/>`, []Run{T("ARCHITECTURE DOCUMENTATION")})
	w.para(`<w:pStyle w:val="Title"/>`, []Run{T(d.Title)})
	w.para(`<w:pStyle w:val="Subtitle"/>`, []Run{T(d.Subtitle)})
	var meta [][][]Run
	for _, m := range d.Meta {
		meta = append(meta, [][]Run{{Muted(m[0])}, {T(m[1])}})
	}
	w.table(Table{Head: []string{"", ""}, Widths: []int{28, 72}, Rows: meta}, textW, false)
	if len(d.Notice) > 0 {
		w.callout(Callout{Tone: "info", Runs: d.Notice}, textW)
	}
	w.para(`<w:spacing w:before="240"/>`, []Run{Muted("Structure based on the arc42 template (arc42.org).")})

	// Contents
	w.para(`<w:pStyle w:val="Heading1"/><w:pageBreakBefore/>`, []Run{T("Contents")})
	d.walk(func(s *Section, depth int) {
		if depth > 1 {
			return
		}
		style := `<w:spacing w:before="120" w:after="0"/>`
		runs := []Run{B(s.Num + "   " + s.Title)}
		if depth == 1 {
			style = `<w:spacing w:before="0" w:after="0"/><w:ind w:left="567"/>`
			runs = []Run{T(s.Num + "   " + s.Title)}
		}
		w.para(style, runs)
	})

	var rec func(s *Section, depth int, width int)
	rec = func(s *Section, depth int, width int) {
		if s.Landscape {
			w.sectionBreak(false) // ends the portrait pages before it
			width = wideTextW
		}
		ppr := fmt.Sprintf(`<w:pStyle w:val="Heading%d"/>`, min(depth+1, 3))
		if s.PageBreak && !s.Landscape {
			ppr += `<w:pageBreakBefore/>`
		}
		w.para(ppr, []Run{Muted(s.Num + "  "), T(s.Title)})
		for _, b := range s.Blocks {
			w.block(b, width)
		}
		for _, c := range s.Sub {
			rec(c, depth+1, width)
		}
		if s.Landscape {
			w.sectionBreak(true) // ends the landscape page
		}
	}
	for _, s := range d.Sections {
		rec(s, 0, textW)
	}
	w.doc.WriteString(sectPr(false) + `</w:body></w:document>`)
}

func sectPr(landscape bool) string {
	size := fmt.Sprintf(`<w:pgSz w:w="%d" w:h="%d"/><w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="567" w:footer="567" w:gutter="0"/>`, pageW, pageH, margin, margin, margin, margin)
	if landscape {
		size = fmt.Sprintf(`<w:pgSz w:w="%d" w:h="%d" w:orient="landscape"/><w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="400" w:footer="400" w:gutter="0"/>`, pageH, pageW, wideMargin, wideMargin, wideMargin, wideMargin)
	}
	return `<w:sectPr><w:footerReference w:type="default" r:id="rIdFooter"/><w:type w:val="nextPage"/>` + size + `</w:sectPr>`
}

// sectionBreak ends the current section (portrait or landscape).
func (w *docxWriter) sectionBreak(landscape bool) {
	w.doc.WriteString(`<w:p><w:pPr>` + sectPr(landscape) + `</w:pPr></w:p>`)
}

func (w *docxWriter) para(ppr string, runs []Run) {
	w.doc.WriteString(`<w:p><w:pPr>` + ppr + `</w:pPr>`)
	w.runs(runs, "")
	w.doc.WriteString(`</w:p>`)
}

func (w *docxWriter) linkID(url string) string {
	if id, ok := w.links[url]; ok {
		return id
	}
	id := fmt.Sprintf("rIdLink%d", len(w.links)+1)
	w.links[url] = id
	w.linkOrder = append(w.linkOrder, url)
	return id
}

// runs writes runs; extra is added to every run's properties (e.g. a smaller size).
func (w *docxWriter) runs(runs []Run, extra string) {
	for _, r := range runs {
		var pr strings.Builder
		if r.Mono {
			fmt.Fprintf(&pr, `<w:rFonts w:ascii="%s" w:hAnsi="%s" w:cs="%s"/>`, monoFont, monoFont, monoFont)
		}
		bold := r.Bold
		color := ""
		switch {
		case strings.HasPrefix(r.Text, "✘"):
			color, bold = colorBad, true
		case strings.HasPrefix(r.Text, "✔"):
			color = colorOK
		case r.Muted:
			color = colorMuted
		case r.Link != "":
			color = colorAccent
		}
		if bold {
			pr.WriteString(`<w:b/>`)
		}
		if r.Italic {
			pr.WriteString(`<w:i/>`)
		}
		if color != "" {
			fmt.Fprintf(&pr, `<w:color w:val="%s"/>`, color)
		}
		if r.Mono {
			pr.WriteString(`<w:sz w:val="18"/>`)
		}
		pr.WriteString(extra)
		var t strings.Builder
		for i, line := range strings.Split(r.Text, "\n") {
			if i > 0 {
				t.WriteString(`<w:br/>`)
			}
			fmt.Fprintf(&t, `<w:t xml:space="preserve">%s</w:t>`, xesc(line))
		}
		run := fmt.Sprintf(`<w:r><w:rPr>%s</w:rPr>%s</w:r>`, pr.String(), t.String())
		if r.Link != "" && (strings.HasPrefix(r.Link, "https://") || strings.HasPrefix(r.Link, "http://")) {
			run = fmt.Sprintf(`<w:hyperlink r:id="%s">%s</w:hyperlink>`, w.linkID(r.Link), run)
		}
		w.doc.WriteString(run)
	}
}

func (w *docxWriter) block(b Block, width int) {
	switch v := b.(type) {
	case Para:
		if v.Small {
			w.doc.WriteString(`<w:p><w:pPr><w:spacing w:after="120"/></w:pPr>`)
			w.runs(v.Runs, `<w:sz w:val="18"/>`)
			w.doc.WriteString(`</w:p>`)
			return
		}
		w.para("", v.Runs)
	case List:
		num := 1
		if v.Ordered {
			w.ordered++
			num = 1 + w.ordered
		}
		for _, it := range v.Items {
			w.para(fmt.Sprintf(`<w:pStyle w:val="ListParagraph"/><w:numPr><w:ilvl w:val="0"/><w:numId w:val="%d"/></w:numPr>`, num), it)
		}
	case Table:
		w.table(v, width, true)
	case Callout:
		w.callout(v, width)
	case Diagram:
		w.diagram(v, width)
	}
}

func (w *docxWriter) table(t Table, width int, lines bool) {
	total := 0
	for _, x := range t.Widths {
		total += x
	}
	cols := make([]int, len(t.Widths))
	for i, x := range t.Widths {
		cols[i] = width * x / max(total, 1)
	}
	border := `<w:tblBorders><w:top w:val="nil"/><w:left w:val="nil"/><w:right w:val="nil"/><w:insideV w:val="nil"/><w:bottom w:val="nil"/><w:insideH w:val="nil"/></w:tblBorders>`
	if lines {
		border = fmt.Sprintf(`<w:tblBorders><w:top w:val="nil"/><w:left w:val="nil"/><w:right w:val="nil"/><w:insideV w:val="nil"/><w:bottom w:val="single" w:sz="4" w:color="%s"/><w:insideH w:val="single" w:sz="4" w:color="%s"/></w:tblBorders>`, colorLine, colorLine)
	}
	fmt.Fprintf(&w.doc, `<w:tbl><w:tblPr><w:tblW w:w="%d" w:type="dxa"/><w:tblLayout w:type="fixed"/>%s<w:tblCellMar><w:top w:w="50" w:type="dxa"/><w:left w:w="80" w:type="dxa"/><w:bottom w:w="50" w:type="dxa"/><w:right w:w="80" w:type="dxa"/></w:tblCellMar></w:tblPr><w:tblGrid>`, width, border)
	for _, c := range cols {
		fmt.Fprintf(&w.doc, `<w:gridCol w:w="%d"/>`, c)
	}
	w.doc.WriteString(`</w:tblGrid>`)
	size := ""
	if t.Compact {
		size = `<w:sz w:val="18"/>`
	}
	head := false
	for _, h := range t.Head {
		head = head || h != ""
	}
	if head {
		w.doc.WriteString(`<w:tr><w:trPr><w:tblHeader/><w:cantSplit/></w:trPr>`)
		for i, h := range t.Head {
			fmt.Fprintf(&w.doc, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/><w:tcBorders><w:bottom w:val="single" w:sz="10" w:color="%s"/></w:tcBorders></w:tcPr><w:p><w:pPr><w:spacing w:after="0"/></w:pPr>`, cols[i], colorInk)
			w.runs([]Run{{Text: strings.ToUpper(h), Bold: true}}, `<w:sz w:val="16"/><w:spacing w:val="10"/>`)
			w.doc.WriteString(`</w:p></w:tc>`)
		}
		w.doc.WriteString(`</w:tr>`)
	}
	for _, r := range t.Rows {
		w.doc.WriteString(`<w:tr><w:trPr><w:cantSplit/></w:trPr>`)
		for i, c := range r {
			if i >= len(cols) {
				break
			}
			fmt.Fprintf(&w.doc, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/></w:tcPr><w:p><w:pPr><w:spacing w:after="0"/></w:pPr>`, cols[i])
			w.runs(c, size)
			w.doc.WriteString(`</w:p></w:tc>`)
		}
		w.doc.WriteString(`</w:tr>`)
	}
	w.doc.WriteString(`</w:tbl><w:p><w:pPr><w:spacing w:after="60"/></w:pPr></w:p>`)
}

func (w *docxWriter) callout(c Callout, width int) {
	fill := colorInfo
	if c.Tone == "warn" {
		fill = colorWarn
	}
	fmt.Fprintf(&w.doc, `<w:tbl><w:tblPr><w:tblW w:w="%d" w:type="dxa"/><w:tblLayout w:type="fixed"/><w:tblBorders><w:top w:val="nil"/><w:left w:val="nil"/><w:bottom w:val="nil"/><w:right w:val="nil"/><w:insideH w:val="nil"/><w:insideV w:val="nil"/></w:tblBorders><w:tblCellMar><w:top w:w="120" w:type="dxa"/><w:left w:w="200" w:type="dxa"/><w:bottom w:w="120" w:type="dxa"/><w:right w:w="200" w:type="dxa"/></w:tblCellMar></w:tblPr><w:tblGrid><w:gridCol w:w="%d"/></w:tblGrid><w:tr><w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/><w:shd w:val="clear" w:color="auto" w:fill="%s"/></w:tcPr><w:p><w:pPr><w:spacing w:after="0"/></w:pPr>`, width, width, width, fill)
	w.runs(c.Runs, `<w:sz w:val="19"/>`)
	w.doc.WriteString(`</w:p></w:tc></w:tr></w:tbl><w:p><w:pPr><w:spacing w:after="60"/></w:pPr></w:p>`)
}

func (w *docxWriter) diagram(d Diagram, width int) {
	if w.imgW == 0 {
		w.para("", []Run{Muted("(The diagram image was not included in this export.)")})
		return
	}
	w.figure++
	maxW, maxH := float64(width), float64(wideTextH)
	if width != wideTextW {
		maxH = float64(pageH - 2*margin - 2000)
	}
	scale := min(maxW/float64(w.imgW), maxH/float64(w.imgH))
	cx := int64(float64(w.imgW)*scale) * emuPerTwip
	cy := int64(float64(w.imgH)*scale) * emuPerTwip
	fmt.Fprintf(&w.doc, `<w:p><w:pPr><w:jc w:val="center"/><w:spacing w:after="60"/></w:pPr><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="%d" cy="%d"/><wp:docPr id="%d" name="Diagram %d" descr="%s"/><wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:nvPicPr><pic:cNvPr id="%d" name="diagram.png"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="rIdImage"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`,
		cx, cy, w.figure, w.figure, xesc(d.Caption), w.figure, cx, cy)
	w.para(`<w:pStyle w:val="Caption"/>`, []Run{T(d.Caption)})
}

func (w *docxWriter) numbering() string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:abstractNum w:abstractNumId="1"><w:multiLevelType w:val="singleLevel"/><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="360" w:hanging="260"/></w:pPr><w:rPr><w:color w:val="7A7A86"/></w:rPr></w:lvl></w:abstractNum>
<w:abstractNum w:abstractNumId="2"><w:multiLevelType w:val="singleLevel"/><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="400" w:hanging="300"/></w:pPr></w:lvl></w:abstractNum>
<w:num w:numId="1"><w:abstractNumId w:val="1"/></w:num>`)
	for i := 1; i <= w.ordered; i++ {
		fmt.Fprintf(&sb, `<w:num w:numId="%d"><w:abstractNumId w:val="2"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="1"/></w:lvlOverride></w:num>`, i+1)
	}
	sb.WriteString(`</w:numbering>`)
	return sb.String()
}

func (w *docxWriter) rels(image bool) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
<Relationship Id="rIdSettings" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
<Relationship Id="rIdNumbering" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>
<Relationship Id="rIdFooter" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>
`)
	if image {
		sb.WriteString(`<Relationship Id="rIdImage" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/diagram.png"/>` + "\n")
	}
	for _, url := range w.linkOrder {
		fmt.Fprintf(&sb, `<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="%s" TargetMode="External"/>`+"\n", w.links[url], xesc(url))
	}
	sb.WriteString(`</Relationships>`)
	return sb.String()
}

const contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Default Extension="png" ContentType="image/png"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
<Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>
<Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>
<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
</Types>`

const rootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`

const coreXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:title>%s</dc:title><dc:creator>Designer</dc:creator><dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created></cp:coreProperties>`

const appXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"><Application>Designer</Application></Properties>`

const settingsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:defaultTabStop w:val="720"/><w:characterSpacingControl w:val="doNotCompress"/><w:compat><w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="15"/></w:compat></w:settings>`

const footerXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:pPr><w:tabs><w:tab w:val="right" w:pos="9638"/></w:tabs></w:pPr><w:r><w:rPr><w:color w:val="7A7A86"/><w:sz w:val="16"/></w:rPr><w:t xml:space="preserve">%s · arc42</w:t></w:r><w:r><w:rPr><w:color w:val="7A7A86"/><w:sz w:val="16"/></w:rPr><w:tab/></w:r><w:r><w:rPr><w:color w:val="7A7A86"/><w:sz w:val="16"/></w:rPr><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:rPr><w:color w:val="7A7A86"/><w:sz w:val="16"/></w:rPr><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r><w:r><w:rPr><w:color w:val="7A7A86"/><w:sz w:val="16"/></w:rPr><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:rPr><w:color w:val="7A7A86"/><w:sz w:val="16"/></w:rPr><w:t>1</w:t></w:r><w:r><w:rPr><w:color w:val="7A7A86"/><w:sz w:val="16"/></w:rPr><w:fldChar w:fldCharType="end"/></w:r></w:p></w:ftr>`

var stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="` + docFont + `" w:hAnsi="` + docFont + `" w:eastAsia="` + docFont + `" w:cs="` + docFont + `"/><w:color w:val="` + colorInk + `"/><w:sz w:val="21"/><w:szCs w:val="21"/><w:lang w:val="en-GB"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="140" w:line="276" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>
<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>
<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:spacing w:after="120"/></w:pPr><w:rPr><w:b/><w:sz w:val="60"/><w:spacing w:val="-10"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Subtitle"><w:name w:val="Subtitle"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:spacing w:after="480"/></w:pPr><w:rPr><w:color w:val="4A4A55"/><w:sz w:val="26"/></w:rPr></w:style>
<w:style w:type="paragraph" w:customStyle="1" w:styleId="Kicker"><w:name w:val="Kicker"/><w:basedOn w:val="Normal"/><w:rPr><w:b/><w:color w:val="` + colorAccent + `"/><w:sz w:val="17"/><w:spacing w:val="30"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:pBdr><w:bottom w:val="single" w:sz="4" w:space="6" w:color="` + colorLine + `"/></w:pBdr><w:spacing w:before="0" w:after="240"/><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:b/><w:sz w:val="34"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="360" w:after="120"/><w:outlineLvl w:val="1"/></w:pPr><w:rPr><w:b/><w:sz w:val="26"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="300" w:after="100"/><w:outlineLvl w:val="2"/></w:pPr><w:rPr><w:b/><w:sz w:val="22"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="40"/><w:ind w:left="720"/><w:contextualSpacing/></w:pPr></w:style>
<w:style w:type="paragraph" w:styleId="Caption"><w:name w:val="caption"/><w:basedOn w:val="Normal"/><w:pPr><w:jc w:val="center"/></w:pPr><w:rPr><w:color w:val="` + colorMuted + `"/><w:sz w:val="18"/></w:rPr></w:style>
</w:styles>`
