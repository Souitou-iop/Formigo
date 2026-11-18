package convert

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
	"go.yaml.in/yaml/v3"
)

// obj is a map that keeps its keys in the order they were written, so that
// a file converted keeps the order its author gave it.
type obj struct {
	keys []string
	vals map[string]any
	// lists are the keys of XML's repeated elements, gathered in a list.
	lists []string
}

func newObj() *obj { return &obj{vals: map[string]any{}} }

func (o *obj) set(k string, v any) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func convertData(src string, w io.Writer, to string, o Options) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	v, err := parseData(byExt[Ext(src)].ID, data)
	if err != nil {
		return err
	}
	out, err := writeData(to, v, o)
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}

func parseData(ext string, data []byte) (any, error) {
	switch ext {
	case "json":
		v, err := parseJSON(data)
		if err != nil {
			return nil, fmt.Errorf("JSON 有误：%w", err)
		}
		return v, nil
	case "yaml", "yml":
		var n yaml.Node
		if err := yaml.Unmarshal(data, &n); err != nil {
			return nil, fmt.Errorf("YAML 有误：%w", err)
		}
		if n.Kind == 0 {
			return nil, nil
		}
		return fromYAML(&n)
	case "toml":
		m := map[string]any{}
		md, err := toml.Decode(string(data), &m)
		if err != nil {
			return nil, fmt.Errorf("TOML 有误：%w", err)
		}
		order := map[string]int{}
		for i, k := range md.Keys() {
			if _, ok := order[k.String()]; !ok {
				order[k.String()] = i
			}
		}
		return fromTOML(m, "", order), nil
	case "csv", "tsv":
		return parseTable(data, ext == "tsv")
	case "jsonl":
		return parseJSONL(data)
	case "xml":
		return parseXML(data)
	case "plist":
		return parsePlist(data)
	case "xlsx":
		return parseXLSX(data)
	}
	return nil, ErrUnsupported
}

func parseJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := jsonValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("值之后还有多余内容")
	}
	return v, nil
}

func jsonValue(dec *json.Decoder) (any, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := t.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := newObj()
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				v, err := jsonValue(dec)
				if err != nil {
					return nil, err
				}
				o.set(kt.(string), v)
			}
			_, err := dec.Token()
			return o, err
		case '[':
			arr := []any{}
			for dec.More() {
				v, err := jsonValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			_, err := dec.Token()
			return arr, err
		}
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i, nil
		}
		f, err := t.Float64()
		return f, err
	}
	return t, nil
}

func fromYAML(n *yaml.Node) (any, error) {
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return fromYAML(n.Content[0])
	case yaml.AliasNode:
		return fromYAML(n.Alias)
	case yaml.MappingNode:
		o := newObj()
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Tag == "!!merge" {
				m, err := fromYAML(v)
				if err != nil {
					return nil, err
				}
				if mo, ok := m.(*obj); ok {
					for _, kk := range mo.keys {
						o.set(kk, mo.vals[kk])
					}
				}
				continue
			}
			val, err := fromYAML(v)
			if err != nil {
				return nil, err
			}
			o.set(k.Value, val)
		}
		return o, nil
	case yaml.SequenceNode:
		arr := []any{}
		for _, c := range n.Content {
			v, err := fromYAML(c)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	case yaml.ScalarNode:
		var v any
		if err := n.Decode(&v); err != nil {
			return nil, err
		}
		switch x := v.(type) {
		case int:
			return int64(x), nil
		case uint64:
			if x <= math.MaxInt64 {
				return int64(x), nil
			}
			return float64(x), nil
		}
		return v, nil
	}
	return nil, nil
}

func fromTOML(v any, path string, order map[string]int) any {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		pos := func(k string) int {
			if i, ok := order[join(path, k)]; ok {
				return i
			}
			return math.MaxInt
		}
		slices.SortStableFunc(keys, func(a, b string) int {
			if d := pos(a) - pos(b); d != 0 {
				return d
			}
			return strings.Compare(a, b)
		})
		o := newObj()
		for _, k := range keys {
			o.set(k, fromTOML(x[k], join(path, k), order))
		}
		return o
	case []map[string]any:
		arr := make([]any, len(x))
		for i, m := range x {
			arr[i] = fromTOML(m, path, order)
		}
		return arr
	case []any:
		arr := make([]any, len(x))
		for i, e := range x {
			arr[i] = fromTOML(e, path, order)
		}
		return arr
	}
	return v
}

// join is the path of key k in table path, as toml.Key.String writes it.
func join(path, k string) string {
	if !bareKey(k) {
		k = `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(k) + `"`
	}
	if path == "" {
		return k
	}
	return path + "." + k
}

// parseTable reads CSV or TSV. A first row of distinct, non-empty names is
// a header, and the rows become objects; otherwise they stay arrays.
func parseTable(data []byte, tabs bool) (any, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	if tabs {
		r.Comma = '\t'
	}
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("表格有误：%w", err)
	}
	return tableOf(rows), nil
}

// tableOf is a table's rows as data: objects by the header row when it
// names every column once, lists otherwise.
func tableOf(rows [][]string) any {
	if len(rows) == 0 {
		return []any{}
	}
	header := rows[0]
	seen := map[string]bool{}
	isHeader := len(rows) > 1
	for _, h := range header {
		if h == "" || seen[h] {
			isHeader = false
			break
		}
		seen[h] = true
	}
	out := make([]any, 0, len(rows))
	if !isHeader {
		for _, row := range rows {
			arr := make([]any, len(row))
			for i, c := range row {
				arr[i] = cell(c)
			}
			out = append(out, arr)
		}
		return out
	}
	for _, row := range rows[1:] {
		o := newObj()
		for i, h := range header {
			if i < len(row) {
				o.set(h, cell(row[i]))
			} else {
				o.set(h, "")
			}
		}
		out = append(out, o)
	}
	return out
}

// cell reads a number or a boolean that a table cell holds, keeping as text
// what would change if written back, as "007" or "1e3".
func cell(s string) any {
	switch s {
	case "true":
		return true
	case "false":
		return false
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil && strconv.FormatInt(i, 10) == s {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) &&
		strconv.FormatFloat(f, 'f', -1, 64) == s {
		return f
	}
	return s
}

func writeData(to string, v any, o Options) ([]byte, error) {
	indent := "  "
	switch {
	case o.Indent < 0:
		indent = ""
	case o.Indent > 0:
		indent = strings.Repeat(" ", o.Indent)
	}
	switch to {
	case "json":
		var b bytes.Buffer
		writeJSON(&b, v, indent, "")
		b.WriteByte('\n')
		return b.Bytes(), nil
	case "yaml":
		var b bytes.Buffer
		enc := yaml.NewEncoder(&b)
		enc.SetIndent(max(2, len(indent)))
		if err := enc.Encode(toYAML(v)); err != nil {
			return nil, err
		}
		enc.Close()
		return b.Bytes(), nil
	case "toml":
		root, ok := v.(*obj)
		if !ok {
			root = newObj()
			root.set("items", v)
		}
		var b bytes.Buffer
		if err := writeTOML(&b, root, nil); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	case "csv", "tsv":
		return writeTable(v, to == "tsv")
	case "jsonl":
		return writeJSONL(v), nil
	case "xml":
		return writeXML(v, indent)
	case "plist":
		return writePlist(v)
	case "xlsx":
		return writeXLSX(v)
	}
	return nil, ErrUnsupported
}

func writeJSON(b *bytes.Buffer, v any, indent, prefix string) {
	nl := func(p string) {
		if indent != "" {
			b.WriteByte('\n')
			b.WriteString(p)
		}
	}
	switch x := v.(type) {
	case *obj:
		if len(x.keys) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteByte('{')
		for i, k := range x.keys {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(prefix + indent)
			writeJSONString(b, k)
			b.WriteByte(':')
			if indent != "" {
				b.WriteByte(' ')
			}
			writeJSON(b, x.vals[k], indent, prefix+indent)
		}
		nl(prefix)
		b.WriteByte('}')
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(prefix + indent)
			writeJSON(b, e, indent, prefix+indent)
		}
		nl(prefix)
		b.WriteByte(']')
	case string:
		writeJSONString(b, x)
	case float64:
		if math.IsInf(x, 0) || math.IsNaN(x) {
			b.WriteString("null")
			return
		}
		b.WriteString(strconv.FormatFloat(x, 'g', -1, 64))
	case time.Time:
		writeJSONString(b, x.Format(time.RFC3339Nano))
	case nil:
		b.WriteString("null")
	default:
		out, err := json.Marshal(x)
		if err != nil {
			writeJSONString(b, fmt.Sprint(x))
			return
		}
		b.Write(out)
	}
}

func writeJSONString(b *bytes.Buffer, s string) {
	var tmp bytes.Buffer
	enc := json.NewEncoder(&tmp)
	enc.SetEscapeHTML(false)
	enc.Encode(s)
	b.Write(bytes.TrimRight(tmp.Bytes(), "\n"))
}

func toYAML(v any) *yaml.Node {
	switch x := v.(type) {
	case *obj:
		n := &yaml.Node{Kind: yaml.MappingNode}
		for _, k := range x.keys {
			n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k, Tag: "!!str"}, toYAML(x.vals[k]))
		}
		return n
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode}
		for _, e := range x {
			n.Content = append(n.Content, toYAML(e))
		}
		return n
	}
	var n yaml.Node
	if t, ok := v.(time.Time); ok {
		v = t.Format(time.RFC3339Nano)
	}
	if err := n.Encode(v); err != nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprint(v)}
	}
	if s, ok := v.(string); ok && strings.Contains(s, "\n") {
		n.Style = yaml.LiteralStyle
	}
	return &n
}

// writeTOML writes a table: its plain values first, then its tables and
// arrays of tables, as TOML needs. Nulls, which TOML has none of, are left
// out.
func writeTOML(b *bytes.Buffer, o *obj, path []string) error {
	var tables []string
	for _, k := range o.keys {
		v := o.vals[k]
		if v == nil {
			continue
		}
		if isTable(v) || isTableArray(v) {
			tables = append(tables, k)
			continue
		}
		b.WriteString(tomlKey(k))
		b.WriteString(" = ")
		if err := tomlValue(b, v); err != nil {
			return err
		}
		b.WriteByte('\n')
	}
	for _, k := range tables {
		p := append(slices.Clone(path), k)
		name := make([]string, len(p))
		for i, s := range p {
			name[i] = tomlKey(s)
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		switch v := o.vals[k].(type) {
		case *obj:
			b.WriteString("[" + strings.Join(name, ".") + "]\n")
			if err := writeTOML(b, v, p); err != nil {
				return err
			}
		case []any:
			for i, e := range v {
				if i > 0 {
					b.WriteByte('\n')
				}
				b.WriteString("[[" + strings.Join(name, ".") + "]]\n")
				if err := writeTOML(b, e.(*obj), p); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func isTable(v any) bool { _, ok := v.(*obj); return ok }

func isTableArray(v any) bool {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return false
	}
	for _, e := range arr {
		if !isTable(e) {
			return false
		}
	}
	return true
}

func bareKey(k string) bool {
	return k != "" && strings.IndexFunc(k, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
	}) < 0
}

func tomlKey(k string) string {
	if bareKey(k) {
		return k
	}
	return tomlString(k)
}

func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func tomlValue(b *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case string:
		b.WriteString(tomlString(x))
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case float64:
		switch {
		case math.IsNaN(x):
			b.WriteString("nan")
		case math.IsInf(x, 1):
			b.WriteString("inf")
		case math.IsInf(x, -1):
			b.WriteString("-inf")
		default:
			s := strconv.FormatFloat(x, 'g', -1, 64)
			if !strings.ContainsAny(s, ".eE") {
				s += ".0"
			}
			b.WriteString(s)
		}
	case time.Time:
		b.WriteString(x.Format(time.RFC3339Nano))
	case []any:
		b.WriteByte('[')
		first := true
		for _, e := range x {
			if e == nil {
				continue
			}
			if !first {
				b.WriteString(", ")
			}
			first = false
			if err := tomlValue(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case *obj:
		b.WriteByte('{')
		first := true
		for _, k := range x.keys {
			if x.vals[k] == nil {
				continue
			}
			if !first {
				b.WriteString(", ")
			}
			first = false
			b.WriteString(tomlKey(k) + " = ")
			if err := tomlValue(b, x.vals[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		b.WriteString(tomlString(fmt.Sprint(x)))
	}
	return nil
}

// writeTable writes rows of objects with their keys as the header, rows of
// arrays as they are, or an object's keys and values as two columns.
func writeTable(v any, tabs bool) ([]byte, error) {
	rows := tableRows(v)
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if tabs {
		w.Comma = '\t'
	}
	if err := w.WriteAll(rows); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// tableRows lays data out as a table's rows.
func tableRows(v any) [][]string {
	var rows [][]string
	switch x := v.(type) {
	case []any:
		objects, arrays := true, true
		for _, e := range x {
			_, isObj := e.(*obj)
			_, isArr := e.([]any)
			objects = objects && isObj
			arrays = arrays && isArr
		}
		switch {
		case len(x) == 0:
		case objects:
			var header []string
			seen := map[string]bool{}
			for _, e := range x {
				for _, k := range e.(*obj).keys {
					if !seen[k] {
						seen[k] = true
						header = append(header, k)
					}
				}
			}
			rows = append(rows, header)
			for _, e := range x {
				o := e.(*obj)
				row := make([]string, len(header))
				for i, k := range header {
					if v, ok := o.vals[k]; ok {
						row[i] = cellText(v)
					}
				}
				rows = append(rows, row)
			}
		case arrays:
			for _, e := range x {
				var row []string
				for _, c := range e.([]any) {
					row = append(row, cellText(c))
				}
				rows = append(rows, row)
			}
		default:
			rows = append(rows, []string{"value"})
			for _, e := range x {
				rows = append(rows, []string{cellText(e)})
			}
		}
	case *obj:
		// An object holding one list of rows, as {"items": [...]}, is that list.
		if len(x.keys) == 1 {
			if arr, ok := x.vals[x.keys[0]].([]any); ok {
				return tableRows(arr)
			}
		}
		rows = append(rows, []string{"key", "value"})
		for _, k := range x.keys {
			rows = append(rows, []string{k, cellText(x.vals[k])})
		}
	default:
		rows = append(rows, []string{cellText(x)})
	}
	return rows
}

func cellText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case *obj, []any:
		var b bytes.Buffer
		writeJSON(&b, x, "", "")
		return b.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	}
	return fmt.Sprint(v)
}

// DataSummary describes structured data in a few words, as "12 条记录".
func DataSummary(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || !utf8.Valid(data) {
		return ""
	}
	v, err := parseData(Ext(path), bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")))
	if err != nil {
		return "格式有误"
	}
	switch x := v.(type) {
	case []any:
		return fmt.Sprintf("%d 条记录", len(x))
	case *obj:
		return fmt.Sprintf("%d 个字段", len(x.keys))
	}
	return ""
}

// dataFormats are the formats of structured data Go reads and writes.
var dataFormats = []string{"json", "jsonl", "yaml", "toml", "xml", "csv", "tsv", "xlsx", "plist"}

func init() {
	register(&edge{engine: "data", from: dataFormats, to: dataFormats, cost: 1, run: func(j *job) error {
		return writeVia(j, func(f *os.File) error { return convertData(j.src, f, j.to.ID, j.o) })
	}})
}
