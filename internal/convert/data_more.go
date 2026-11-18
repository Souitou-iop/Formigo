package convert

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"howett.net/plist"
)

// JSON Lines: a JSON value on each line.

func parseJSONL(data []byte) (any, error) {
	var out []any
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		v, err := parseJSON(line)
		if err != nil {
			return nil, fmt.Errorf("JSONL 第 %d 行有误：%w", n, err)
		}
		out = append(out, v)
	}
	if out == nil {
		out = []any{}
	}
	return out, sc.Err()
}

func writeJSONL(v any) []byte {
	items, ok := v.([]any)
	if !ok {
		items = []any{v}
	}
	var b bytes.Buffer
	for _, it := range items {
		writeJSON(&b, it, "", "")
		b.WriteByte('\n')
	}
	return b.Bytes()
}

// XML: an element is an object of its attributes ("@name"), its children
// by name, a list where a name repeats, and its text ("#text"); an element
// of text alone is that text.

func parseXML(data []byte) (any, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.CharsetReader = func(label string, r io.Reader) (io.Reader, error) { return r, nil }
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, errors.New("XML 是空的")
		}
		if err != nil {
			return nil, fmt.Errorf("XML 有误：%w", err)
		}
		if start, ok := tok.(xml.StartElement); ok {
			v, err := xmlElement(dec, start)
			if err != nil {
				return nil, fmt.Errorf("XML 有误：%w", err)
			}
			root := newObj()
			root.set(start.Name.Local, v)
			return root, nil
		}
	}
}

func xmlElement(dec *xml.Decoder, start xml.StartElement) (any, error) {
	o := newObj()
	for _, a := range start.Attr {
		o.set("@"+a.Name.Local, cell(a.Value))
	}
	var text strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			child, err := xmlElement(dec, t)
			if err != nil {
				return nil, err
			}
			name := t.Name.Local
			if old, ok := o.vals[name]; ok {
				if list, ok := old.([]any); ok && slices.Contains(o.lists, name) {
					o.vals[name] = append(list, child)
				} else {
					o.vals[name] = []any{old, child}
					o.lists = append(o.lists, name)
				}
			} else {
				o.set(name, child)
			}
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			s := strings.TrimSpace(text.String())
			if len(o.keys) == 0 {
				return cell(s), nil
			}
			if s != "" {
				o.set("#text", cell(s))
			}
			return o, nil
		}
	}
}

func writeXML(v any, indent string) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	name, body := "root", v
	if o, ok := v.(*obj); ok && len(o.keys) == 1 {
		if _, isList := o.vals[o.keys[0]].([]any); !isList {
			name, body = o.keys[0], o.vals[o.keys[0]]
		}
	}
	if list, ok := body.([]any); ok {
		// A list at the top: items of one root.
		o := newObj()
		o.set("item", list)
		body = o
	}
	writeXMLElement(&b, xmlName(name), body, indent, "")
	return b.Bytes(), nil
}

func writeXMLElement(b *bytes.Buffer, name string, v any, indent, prefix string) {
	nl := "\n"
	if indent == "" {
		nl = ""
	}
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			writeXMLElement(b, name, e, indent, prefix)
		}
		return
	case *obj:
		b.WriteString(prefix + "<" + name)
		var children []string
		text := ""
		for _, k := range x.keys {
			switch {
			case strings.HasPrefix(k, "@"):
				fmt.Fprintf(b, " %s=\"%s\"", xmlName(k[1:]), xmlEscape(cellText(x.vals[k])))
			case k == "#text":
				text = cellText(x.vals[k])
			default:
				children = append(children, k)
			}
		}
		if len(children) == 0 {
			if text == "" {
				b.WriteString("/>" + nl)
				return
			}
			b.WriteString(">" + xmlEscape(text) + "</" + name + ">" + nl)
			return
		}
		b.WriteString(">" + nl)
		if text != "" {
			b.WriteString(prefix + indent + xmlEscape(text) + nl)
		}
		for _, k := range children {
			writeXMLElement(b, xmlName(k), x.vals[k], indent, prefix+indent)
		}
		b.WriteString(prefix + "</" + name + ">" + nl)
	case nil:
		b.WriteString(prefix + "<" + name + "/>" + nl)
	default:
		b.WriteString(prefix + "<" + name + ">" + xmlEscape(cellText(x)) + "</" + name + ">" + nl)
	}
}

// xmlName makes a key a name XML takes.
func xmlName(k string) string {
	var b strings.Builder
	for i, r := range k {
		ok := r == '_' || r == '-' || r == '.' || r == ':' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 127 || r >= '0' && r <= '9'
		if !ok {
			r = '_'
		}
		if i == 0 && (r >= '0' && r <= '9' || r == '-' || r == '.') {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "item"
	}
	return b.String()
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// Property lists: Apple's, in XML or binary.

func parsePlist(data []byte) (any, error) {
	var v any
	if _, err := plist.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("PLIST 有误：%w", err)
	}
	return fromPlist(v), nil
}

func fromPlist(v any) any {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		o := newObj()
		for _, k := range keys {
			o.set(k, fromPlist(x[k]))
		}
		return o
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = fromPlist(e)
		}
		return out
	case uint64:
		return int64(x)
	case []byte:
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339)
	}
	return v
}

func writePlist(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := plist.NewEncoderForFormat(&b, plist.XMLFormat)
	enc.Indent("\t")
	if err := enc.Encode(toPlain(v)); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// toPlain is data as Go's maps and slices, for the encoders that take them.
func toPlain(v any) any {
	switch x := v.(type) {
	case *obj:
		m := make(map[string]any, len(x.keys))
		for _, k := range x.keys {
			if x.vals[k] != nil {
				m[k] = toPlain(x.vals[k])
			}
		}
		return m
	case []any:
		out := make([]any, 0, len(x))
		for _, e := range x {
			if e != nil {
				out = append(out, toPlain(e))
			}
		}
		return out
	case nil:
		return ""
	}
	return v
}

// Excel workbooks: a sheet is a table; a workbook of several is an object
// of them by name.

func parseXLSX(data []byte) (any, error) {
	wb, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("无法读取 Excel：%w", err)
	}
	defer wb.Close()
	sheets := wb.GetSheetList()
	read := func(name string) (any, error) {
		rows, err := wb.GetRows(name)
		if err != nil {
			return nil, err
		}
		return tableOf(rows), nil
	}
	if len(sheets) == 1 {
		return read(sheets[0])
	}
	o := newObj()
	for _, s := range sheets {
		t, err := read(s)
		if err != nil {
			return nil, err
		}
		o.set(s, t)
	}
	return o, nil
}

func writeXLSX(v any) ([]byte, error) {
	wb := excelize.NewFile()
	defer wb.Close()
	sheets := map[string]any{"Sheet1": v}
	order := []string{"Sheet1"}
	// An object of tables is a workbook of sheets.
	if o, ok := v.(*obj); ok && len(o.keys) > 1 {
		all := true
		for _, k := range o.keys {
			_, isList := o.vals[k].([]any)
			all = all && isList
		}
		if all {
			sheets, order = map[string]any{}, nil
			for i, k := range o.keys {
				name := sheetName(k, i)
				sheets[name], order = o.vals[k], append(order, name)
			}
		}
	}
	for i, name := range order {
		if i == 0 {
			wb.SetSheetName("Sheet1", name)
		} else if _, err := wb.NewSheet(name); err != nil {
			return nil, err
		}
		for r, row := range tableRows(sheets[name]) {
			for c, text := range row {
				ref, _ := excelize.CoordinatesToCellName(c+1, r+1)
				var val any = text
				if r > 0 {
					val = cellValue(text)
				}
				if err := wb.SetCellValue(name, ref, val); err != nil {
					return nil, err
				}
			}
		}
		if r := tableRows(sheets[name]); len(r) > 1 {
			// The header row stays in view, in bold.
			wb.SetPanes(name, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
			if st, err := wb.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}}); err == nil {
				end, _ := excelize.CoordinatesToCellName(max(1, len(r[0])), 1)
				wb.SetCellStyle(name, "A1", end, st)
			}
		}
	}
	var b bytes.Buffer
	if err := wb.Write(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// sheetName is a key as Excel takes a sheet's name.
func sheetName(k string, i int) string {
	k = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`[]:*?/\`, r) {
			return '_'
		}
		return r
	}, k)
	if r := []rune(k); len(r) > 31 {
		k = string(r[:31])
	}
	if strings.TrimSpace(k) == "" {
		k = "Sheet" + strconv.Itoa(i+1)
	}
	return k
}

// cellValue is a cell's text as a number where it is one.
func cellValue(s string) any {
	switch v := cell(s).(type) {
	case int64, float64, bool:
		return v
	}
	return s
}
