package convert

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// Documents meet in HTML: each read becomes a page of HTML, which each
// written is made from. Markdown, HTML and text keep their own direct
// ways, which lose less.

var (
	docIn  = []string{"md", "html", "txt", "docx", "epub", "rtf"}
	docOut = []string{"md", "html", "txt", "docx", "epub", "rtf"}
)

func init() {
	register(&edge{engine: "doc", from: docIn, to: docOut, cost: 1, run: convertDocument})
}

func convertDocument(j *job) error {
	plain := func(id string) bool { return id == "md" || id == "html" || id == "txt" }
	if plain(j.from.ID) && plain(j.to.ID) {
		return writeVia(j, func(f *os.File) error { return convertDoc(j.src, f, j.to.ID) })
	}
	title, body, err := docToHTML(j.src, j.from.ID)
	if err != nil {
		return err
	}
	j.p(0.5)
	var out []byte
	switch j.to.ID {
	case "html":
		out = []byte(page(title, body))
	case "md":
		md, err := toMarkdown(body)
		if err != nil {
			return fmt.Errorf("无法转换为 Markdown：%w", err)
		}
		out = []byte(md + "\n")
	case "txt":
		out = []byte(htmlToText(body))
	case "docx":
		out, err = htmlToDOCX(title, body)
	case "epub":
		out, err = htmlToEPUB(title, body)
	case "rtf":
		out = []byte(textToRTF(htmlToText(body)))
	default:
		return ErrUnsupported
	}
	if err != nil {
		return err
	}
	return os.WriteFile(j.dst, out, 0o644)
}

// docToHTML reads a document as its title and the body of a page of HTML.
func docToHTML(src, from string) (title, body string, err error) {
	switch from {
	case "md":
		text, err := readText(src)
		if err != nil {
			return "", "", err
		}
		body, err := mdToHTML(text)
		return titleOf(text, src), body, err
	case "html":
		text, err := readText(src)
		if err != nil {
			return "", "", err
		}
		return titleOf(htmlToText(text), src), bodyOf(text), nil
	case "txt":
		text, err := readText(src)
		if err != nil {
			return "", "", err
		}
		return titleOf(text, src), textToHTML(text), nil
	case "rtf":
		data, err := os.ReadFile(src)
		if err != nil {
			return "", "", err
		}
		text := rtfToText(data)
		return titleOf(text, src), textToHTML(text), nil
	case "docx":
		return docxToHTML(src)
	case "epub":
		return epubToHTML(src)
	}
	return "", "", ErrUnsupported
}

func textToHTML(text string) string {
	var b strings.Builder
	for _, para := range strings.Split(strings.TrimSpace(text), "\n\n") {
		if strings.TrimSpace(para) == "" {
			continue
		}
		b.WriteString("<p>")
		b.WriteString(strings.ReplaceAll(html.EscapeString(strings.TrimSpace(para)), "\n", "<br>\n"))
		b.WriteString("</p>\n")
	}
	return b.String()
}

var bodyRe = regexp.MustCompile(`(?is)<body[^>]*>(.*)</body>`)

// bodyOf is what a page's body holds, or the whole when it has none.
func bodyOf(page string) string {
	if m := bodyRe.FindStringSubmatch(page); m != nil {
		return m[1]
	}
	return page
}

// Word documents.

func docxToHTML(src string) (string, string, error) {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return "", "", fmt.Errorf("无法读取 Word 文档：%w", err)
	}
	defer zr.Close()
	var doc []byte
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", "", err
			}
			doc, err = io.ReadAll(io.LimitReader(rc, 256<<20))
			rc.Close()
			if err != nil {
				return "", "", err
			}
		}
	}
	if doc == nil {
		return "", "", errors.New("不是 Word 文档")
	}
	dec := xml.NewDecoder(bytes.NewReader(doc))
	var b, para, run strings.Builder
	var style string
	var bold, italic, inList, inTable bool
	var cells []string
	var rows [][]string
	cellStart := 0
	title := ""
	flushRun := func() {
		t := html.EscapeString(run.String())
		if t != "" {
			if bold {
				t = "<strong>" + t + "</strong>"
			}
			if italic {
				t = "<em>" + t + "</em>"
			}
			para.WriteString(t)
		}
		run.Reset()
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", fmt.Errorf("Word 文档有误：%w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				style, inList = "", false
				para.Reset()
			case "pStyle":
				style = attr(t, "val")
			case "numPr":
				inList = true
			case "r":
				bold, italic = false, false
			case "b":
				bold = attr(t, "val") != "0" && attr(t, "val") != "false"
			case "i":
				italic = attr(t, "val") != "0" && attr(t, "val") != "false"
			case "tab":
				run.WriteString("\t")
			case "br":
				flushRun()
				para.WriteString("<br>")
			case "tbl":
				inTable, rows = true, nil
			case "tr":
				cells = nil
			case "tc":
				cellStart = len(cells)
			case "t":
				var s string
				if err := dec.DecodeElement(&s, &t); err == nil {
					run.WriteString(s)
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "r":
				flushRun()
			case "p":
				flushRun()
				text := para.String()
				if inTable {
					cells = append(cells, text)
					continue
				}
				if strings.TrimSpace(text) == "" {
					continue
				}
				tag := "p"
				low := strings.ToLower(style)
				switch {
				case low == "title":
					tag = "h1"
				case strings.HasPrefix(low, "heading") && len(low) == 8:
					tag = "h" + low[7:]
				case strings.Contains(low, "list") || inList:
					tag = "li"
				}
				if title == "" && tag[0] == 'h' {
					title = htmlToText(text)
				}
				if tag == "li" {
					fmt.Fprintf(&b, "<ul><li>%s</li></ul>\n", text)
				} else {
					fmt.Fprintf(&b, "<%s>%s</%s>\n", tag, text, tag)
				}
			case "tc":
				// A cell's paragraphs, joined.
				if cellStart <= len(cells) {
					joined := strings.Join(cells[cellStart:], "<br>")
					cells = append(cells[:cellStart], joined)
				}
			case "tr":
				rows = append(rows, cells)
			case "tbl":
				inTable = false
				b.WriteString("<table>\n")
				for i, r := range rows {
					tag := map[bool]string{true: "th", false: "td"}[i == 0]
					b.WriteString("<tr>")
					for _, c := range r {
						fmt.Fprintf(&b, "<%s>%s</%s>", tag, c, tag)
					}
					b.WriteString("</tr>\n")
				}
				b.WriteString("</table>\n")
			}
		}
	}
	body := strings.ReplaceAll(b.String(), "</ul>\n<ul>", "\n")
	if title == "" {
		title = titleOf(htmlToText(body), src)
	}
	return title, body, nil
}

func attr(t xml.StartElement, name string) string {
	for _, a := range t.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// htmlToDOCX writes a Word document of a page's headings, paragraphs,
// lists, tables and emphasis.
func htmlToDOCX(title, body string) ([]byte, error) {
	root, err := xhtml.Parse(strings.NewReader("<html><body>" + body + "</body></html>"))
	if err != nil {
		return nil, err
	}
	var doc strings.Builder
	var runs strings.Builder
	type state struct{ bold, italic, code bool }
	var inline func(n *xhtml.Node, s state)
	inline = func(n *xhtml.Node, s state) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch {
			case c.Type == xhtml.TextNode:
				if strings.TrimSpace(c.Data) == "" && !strings.Contains(c.Data, " ") {
					continue
				}
				runs.WriteString("<w:r>")
				if s.bold || s.italic || s.code {
					runs.WriteString("<w:rPr>")
					if s.code {
						runs.WriteString(`<w:rFonts w:ascii="Menlo" w:hAnsi="Menlo"/>`)
					}
					if s.bold {
						runs.WriteString("<w:b/>")
					}
					if s.italic {
						runs.WriteString("<w:i/>")
					}
					runs.WriteString("</w:rPr>")
				}
				fmt.Fprintf(&runs, `<w:t xml:space="preserve">%s</w:t></w:r>`, xmlEscape(c.Data))
			case c.DataAtom == atom.Br:
				runs.WriteString("<w:r><w:br/></w:r>")
			case c.DataAtom == atom.Strong || c.DataAtom == atom.B:
				inline(c, state{true, s.italic, s.code})
			case c.DataAtom == atom.Em || c.DataAtom == atom.I:
				inline(c, state{s.bold, true, s.code})
			case c.DataAtom == atom.Code:
				inline(c, state{s.bold, s.italic, true})
			case c.Type == xhtml.ElementNode:
				inline(c, s)
			}
		}
	}
	para := func(n *xhtml.Node, style string) {
		runs.Reset()
		inline(n, state{})
		doc.WriteString("<w:p>")
		if style != "" {
			fmt.Fprintf(&doc, `<w:pPr><w:pStyle w:val="%s"/></w:pPr>`, style)
		}
		doc.WriteString(runs.String())
		doc.WriteString("</w:p>")
	}
	var block func(n *xhtml.Node)
	block = func(n *xhtml.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch c.DataAtom {
			case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
				para(c, "Heading"+c.Data[1:])
			case atom.P, atom.Blockquote, atom.Pre:
				para(c, map[bool]string{true: "Code", false: ""}[c.DataAtom == atom.Pre])
			case atom.Ul, atom.Ol:
				for li := c.FirstChild; li != nil; li = li.NextSibling {
					if li.DataAtom == atom.Li {
						para(li, "ListBullet")
					}
				}
			case atom.Table:
				doc.WriteString(`<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/><w:tblW w:w="0" w:type="auto"/></w:tblPr>`)
				var rows func(n *xhtml.Node)
				rows = func(n *xhtml.Node) {
					for r := n.FirstChild; r != nil; r = r.NextSibling {
						if r.DataAtom != atom.Tr {
							rows(r)
							continue
						}
						doc.WriteString("<w:tr>")
						for cell := r.FirstChild; cell != nil; cell = cell.NextSibling {
							if cell.DataAtom == atom.Td || cell.DataAtom == atom.Th {
								doc.WriteString("<w:tc>")
								para(cell, "")
								doc.WriteString("</w:tc>")
							}
						}
						doc.WriteString("</w:tr>")
					}
				}
				rows(c)
				doc.WriteString("</w:tbl>")
			case atom.Hr:
				doc.WriteString(`<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="auto"/></w:pBdr></w:pPr></w:p>`)
			default:
				if c.Type == xhtml.ElementNode {
					block(c)
				} else if c.Type == xhtml.TextNode && strings.TrimSpace(c.Data) != "" {
					runs.Reset()
					fmt.Fprintf(&doc, `<w:p><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, xmlEscape(strings.TrimSpace(c.Data)))
				}
			}
		}
	}
	block(root)
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/><Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/></Relationships>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"docProps/core.xml": fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:title>%s</dc:title><dc:creator>Formigo</dc:creator><dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created></cp:coreProperties>`, xmlEscape(title), time.Now().UTC().Format(time.RFC3339)),
		"word/styles.xml": docxStyles,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + doc.String() +
			`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="708" w:footer="708" w:gutter="0"/></w:sectPr></w:body></w:document>`,
	}
	return zipFiles([]string{"[Content_Types].xml", "_rels/.rels", "word/_rels/document.xml.rels", "docProps/core.xml", "word/styles.xml", "word/document.xml"}, files, "")
}

const docxStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="PingFang SC"/><w:sz w:val="22"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="300" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>
<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>
<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:spacing w:before="360" w:after="120"/><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:b/><w:sz w:val="36"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:spacing w:before="280" w:after="100"/><w:outlineLvl w:val="1"/></w:pPr><w:rPr><w:b/><w:sz w:val="30"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:spacing w:before="240" w:after="80"/><w:outlineLvl w:val="2"/></w:pPr><w:rPr><w:b/><w:sz w:val="26"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading4"><w:name w:val="heading 4"/><w:basedOn w:val="Normal"/><w:pPr><w:outlineLvl w:val="3"/></w:pPr><w:rPr><w:b/><w:sz w:val="24"/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading5"><w:name w:val="heading 5"/><w:basedOn w:val="Normal"/><w:rPr><w:b/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading6"><w:name w:val="heading 6"/><w:basedOn w:val="Normal"/><w:rPr><w:b/><w:i/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="ListBullet"><w:name w:val="List Bullet"/><w:basedOn w:val="Normal"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:style>
<w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:basedOn w:val="Normal"/><w:rPr><w:rFonts w:ascii="Menlo" w:hAnsi="Menlo"/><w:sz w:val="20"/></w:rPr></w:style>
<w:style w:type="table" w:styleId="TableGrid"><w:name w:val="Table Grid"/><w:tblPr><w:tblBorders><w:top w:val="single" w:sz="4" w:color="auto"/><w:left w:val="single" w:sz="4" w:color="auto"/><w:bottom w:val="single" w:sz="4" w:color="auto"/><w:right w:val="single" w:sz="4" w:color="auto"/><w:insideH w:val="single" w:sz="4" w:color="auto"/><w:insideV w:val="single" w:sz="4" w:color="auto"/></w:tblBorders></w:tblPr></w:style>
</w:styles>`

// zipFiles packs files in order, the first stored when store names it, as
// an EPUB's mimetype must be.
func zipFiles(order []string, files map[string]string, store string) ([]byte, error) {
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, name := range order {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()}
		if name == store {
			// Stored, and with no extra field: its bytes follow its name.
			h = &zip.FileHeader{Name: name, Method: zip.Store}
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(w, files[name]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// E-books.

func epubToHTML(src string) (string, string, error) {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return "", "", fmt.Errorf("无法读取电子书：%w", err)
	}
	defer zr.Close()
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	read := func(name string) ([]byte, error) {
		f, ok := files[name]
		if !ok {
			return nil, fmt.Errorf("电子书缺少 %s", name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(io.LimitReader(rc, 64<<20))
	}
	cdata, err := read("META-INF/container.xml")
	if err != nil {
		return "", "", err
	}
	var container struct {
		Rootfiles []struct {
			Path string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := xml.Unmarshal(cdata, &container); err != nil || len(container.Rootfiles) == 0 {
		return "", "", errors.New("电子书目录有误")
	}
	opfPath := container.Rootfiles[0].Path
	odata, err := read(opfPath)
	if err != nil {
		return "", "", err
	}
	var opf struct {
		Title    string `xml:"metadata>title"`
		Manifest []struct {
			ID   string `xml:"id,attr"`
			Href string `xml:"href,attr"`
		} `xml:"manifest>item"`
		Spine []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"spine>itemref"`
	}
	if err := xml.Unmarshal(odata, &opf); err != nil {
		return "", "", fmt.Errorf("电子书目录有误：%w", err)
	}
	hrefs := map[string]string{}
	for _, it := range opf.Manifest {
		hrefs[it.ID] = it.Href
	}
	base := path.Dir(opfPath)
	var b strings.Builder
	for _, s := range opf.Spine {
		href, ok := hrefs[s.IDRef]
		if !ok {
			continue
		}
		name := path.Clean(path.Join(base, href))
		data, err := read(name)
		if err != nil {
			continue
		}
		b.WriteString(bodyOf(string(data)))
		b.WriteString("\n")
	}
	title := strings.TrimSpace(opf.Title)
	if title == "" {
		title = titleOf(htmlToText(b.String()), src)
	}
	return title, b.String(), nil
}

// htmlToEPUB writes an EPUB 3 of a page, a chapter at each top heading.
func htmlToEPUB(title, body string) ([]byte, error) {
	clean, err := xhtmlOf(body)
	if err != nil {
		return nil, err
	}
	chapters := splitChapters(clean)
	files := map[string]string{
		"mimetype": "application/epub+zip",
		"META-INF/container.xml": `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
	}
	order := []string{"mimetype", "META-INF/container.xml"}
	var manifest, spine, nav strings.Builder
	for i, ch := range chapters {
		name := fmt.Sprintf("chapter%d.xhtml", i+1)
		files["OEBPS/"+name] = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh"><head><meta charset="utf-8"/><title>%s</title></head><body>
%s
</body></html>`, xmlEscape(ch.title), ch.body)
		order = append(order, "OEBPS/"+name)
		fmt.Fprintf(&manifest, `<item id="c%d" href="%s" media-type="application/xhtml+xml"/>`, i+1, name)
		fmt.Fprintf(&spine, `<itemref idref="c%d"/>`, i+1)
		fmt.Fprintf(&nav, `<li><a href="%s">%s</a></li>`, name, xmlEscape(ch.title))
	}
	files["OEBPS/nav.xhtml"] = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><meta charset="utf-8"/><title>%s</title></head><body><nav epub:type="toc"><ol>%s</ol></nav></body></html>`, xmlEscape(title), nav.String())
	files["OEBPS/content.opf"] = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="id">urn:formigo:%d</dc:identifier><dc:title>%s</dc:title><dc:language>zh</dc:language><meta property="dcterms:modified">%s</meta></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>%s</manifest><spine>%s</spine></package>`,
		time.Now().UnixNano(), xmlEscape(title), time.Now().UTC().Format("2006-01-02T15:04:05Z"), manifest.String(), spine.String())
	order = append(order, "OEBPS/nav.xhtml", "OEBPS/content.opf")
	return zipFiles(order, files, "mimetype")
}

type chapter struct{ title, body string }

var h1Re = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)

func splitChapters(body string) []chapter {
	locs := h1Re.FindAllStringSubmatchIndex(body, -1)
	if len(locs) < 2 {
		t := "正文"
		if len(locs) == 1 {
			t = htmlToText(body[locs[0][2]:locs[0][3]])
		}
		return []chapter{{strings.TrimSpace(t), body}}
	}
	var out []chapter
	if pre := strings.TrimSpace(body[:locs[0][0]]); pre != "" {
		out = append(out, chapter{"前言", pre})
	}
	for i, l := range locs {
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out = append(out, chapter{strings.TrimSpace(htmlToText(body[l[2]:l[3]])), body[l[0]:end]})
	}
	return out
}

// xhtmlOf rewrites HTML as XHTML, which EPUB requires.
func xhtmlOf(body string) (string, error) {
	nodes, err := xhtml.ParseFragment(strings.NewReader(body), &xhtml.Node{Type: xhtml.ElementNode, Data: "body", DataAtom: atom.Body})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	var write func(n *xhtml.Node)
	write = func(n *xhtml.Node) {
		switch n.Type {
		case xhtml.TextNode:
			b.WriteString(xmlEscape(n.Data))
		case xhtml.ElementNode:
			if n.DataAtom == atom.Script || n.DataAtom == atom.Style {
				return
			}
			b.WriteString("<" + n.Data)
			for _, a := range n.Attr {
				if a.Namespace == "" && !strings.HasPrefix(a.Key, "on") {
					fmt.Fprintf(&b, ` %s="%s"`, a.Key, xmlEscape(a.Val))
				}
			}
			if n.FirstChild == nil {
				b.WriteString("/>")
				return
			}
			b.WriteString(">")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				write(c)
			}
			b.WriteString("</" + n.Data + ">")
		}
	}
	for _, n := range nodes {
		write(n)
	}
	return b.String(), nil
}

// Rich text.

// rtfToText is an RTF document's text: its groups of fonts, colors and
// pictures left out, its escapes read in the code page it names.
func rtfToText(data []byte) string {
	var out strings.Builder
	var pending []byte // bytes of \'hh escapes, decoded together
	dec := charmap.Windows1252.NewDecoder()
	flush := func() {
		if len(pending) > 0 {
			s, err := dec.String(string(pending))
			if err != nil {
				s = string(pending)
			}
			out.WriteString(s)
			pending = pending[:0]
		}
	}
	skip := []bool{false}
	uc := 1
	var high rune // the first half of a character beyond the BMP
	skipChars := 0
	for i := 0; i < len(data); i++ {
		c := data[i]
		hidden := skip[len(skip)-1]
		switch c {
		case '{':
			skip = append(skip, hidden)
		case '}':
			if len(skip) > 1 {
				skip = skip[:len(skip)-1]
			}
		case '\\':
			if i+1 >= len(data) {
				break
			}
			n := data[i+1]
			switch {
			case n == '\'' && i+3 < len(data):
				if v, err := strconv.ParseUint(string(data[i+2:i+4]), 16, 8); err == nil && !hidden {
					if skipChars > 0 {
						skipChars--
					} else {
						pending = append(pending, byte(v))
					}
				}
				i += 3
			case n == '\\' || n == '{' || n == '}':
				if !hidden {
					flush()
					out.WriteByte(n)
				}
				i++
			case n == '*':
				skip[len(skip)-1] = true
				i++
			case n >= 'a' && n <= 'z' || n >= 'A' && n <= 'Z':
				j := i + 1
				for j < len(data) && (data[j] >= 'a' && data[j] <= 'z' || data[j] >= 'A' && data[j] <= 'Z') {
					j++
				}
				word := string(data[i+1 : j])
				k := j
				if k < len(data) && (data[k] == '-' || data[k] >= '0' && data[k] <= '9') {
					k++
					for k < len(data) && data[k] >= '0' && data[k] <= '9' {
						k++
					}
				}
				arg, hasArg := 0, k > j
				if hasArg {
					arg, _ = strconv.Atoi(string(data[j:k]))
				}
				if k < len(data) && data[k] == ' ' {
					k++
				}
				i = k - 1
				switch word {
				case "fonttbl", "colortbl", "stylesheet", "info", "pict", "header", "footer", "listtable", "listoverridetable", "rsidtbl", "generator", "xmlnstbl", "themedata", "colorschememapping", "latentstyles", "datastore":
					skip[len(skip)-1] = true
				case "ansicpg":
					flush()
					switch arg {
					case 936:
						dec = simplifiedchinese.GBK.NewDecoder()
					case 950:
						dec = traditionalchinese.Big5.NewDecoder()
					case 932:
						dec = japanese.ShiftJIS.NewDecoder()
					}
				case "uc":
					uc = arg
				case "u":
					if !hidden {
						flush()
						r := arg
						if r < 0 {
							r += 65536
						}
						switch {
						case r >= 0xD800 && r < 0xDC00:
							high = rune(r)
						case r >= 0xDC00 && r < 0xE000 && high != 0:
							out.WriteRune(0x10000 + (high-0xD800)<<10 + (rune(r) - 0xDC00))
							high = 0
						default:
							out.WriteRune(rune(r))
						}
						skipChars = uc
					}
				case "par", "line", "sect", "page":
					if !hidden {
						flush()
						out.WriteString("\n")
					}
				case "tab":
					if !hidden {
						flush()
						out.WriteString("\t")
					}
				case "row":
					if !hidden {
						flush()
						out.WriteString("\n")
					}
				case "cell":
					if !hidden {
						flush()
						out.WriteString("\t")
					}
				}
			default:
				i++
			}
		case '\r', '\n':
		default:
			if !hidden {
				if skipChars > 0 {
					skipChars--
					continue
				}
				flush()
				out.WriteByte(c)
			}
		}
	}
	flush()
	text := strings.ReplaceAll(out.String(), "\n\n\n", "\n\n")
	return strings.TrimSpace(text) + "\n"
}

// textToRTF writes text as RTF, its paragraphs apart and every character
// beyond ASCII as a Unicode escape.
func textToRTF(text string) string {
	var b strings.Builder
	b.WriteString("{\\rtf1\\ansi\\ansicpg1252\\deff0\\uc1{\\fonttbl{\\f0\\fswiss Helvetica;}{\\f1\\fnil PingFang SC;}}\\f0\\fs24\n")
	for i, para := range strings.Split(strings.TrimSpace(text), "\n\n") {
		if i > 0 {
			b.WriteString("\\par\n")
		}
		for _, r := range para {
			switch {
			case r == '\n':
				b.WriteString("\\line ")
			case r == '\\' || r == '{' || r == '}':
				b.WriteRune('\\')
				b.WriteRune(r)
			case r == '\t':
				b.WriteString("\\tab ")
			case r < 128:
				b.WriteRune(r)
			case r > 0xFFFF:
				for _, u := range utf16Pair(r) {
					fmt.Fprintf(&b, "\\u%d?", int16(u))
				}
			default:
				fmt.Fprintf(&b, "\\u%d?", int16(r))
			}
		}
		b.WriteString("\\par\n")
	}
	b.WriteString("}\n")
	return b.String()
}

// utf16Pair is a rune beyond the BMP as its two surrogates.
func utf16Pair(r rune) []rune {
	r -= 0x10000
	return []rune{0xD800 + (r>>10)&0x3FF, 0xDC00 + r&0x3FF}
}
