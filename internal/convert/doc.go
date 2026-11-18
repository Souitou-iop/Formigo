package convert

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	xhtml "golang.org/x/net/html"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Footnote, extension.Typographer),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
)

// readText reads a text file as UTF-8, from UTF-16 with its byte order mark
// or from GB 18030 (GBK) when it is not UTF-8, as older Chinese files are.
func readText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	switch {
	case bytes.HasPrefix(data, []byte("\xef\xbb\xbf")):
		data = data[3:]
	case bytes.HasPrefix(data, []byte("\xff\xfe")), bytes.HasPrefix(data, []byte("\xfe\xff")):
		out, _, err := transform.Bytes(unicode.BOMOverride(unicode.UTF8.NewDecoder()), data)
		if err != nil {
			return "", err
		}
		data = out
	case !utf8.Valid(data):
		out, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), data)
		if err != nil {
			return "", fmt.Errorf("无法识别文本编码")
		}
		data = out
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n"), nil
}

func convertDoc(src string, w io.Writer, to string) error {
	text, err := readText(src)
	if err != nil {
		return err
	}
	from := Ext(src)
	switch from {
	case "markdown":
		from = "md"
	case "htm":
		from = "html"
	case "text":
		from = "txt"
	}
	var out string
	switch {
	case from == to:
		out = text
	case from == "md" && to == "html":
		body, err := mdToHTML(text)
		if err != nil {
			return err
		}
		out = page(titleOf(text, src), body)
	case from == "md" && to == "txt":
		body, err := mdToHTML(text)
		if err != nil {
			return err
		}
		out = htmlToText(body)
	case from == "html" && to == "md":
		out, err = toMarkdown(text)
		if err != nil {
			return fmt.Errorf("无法转换 HTML：%w", err)
		}
		out += "\n"
	case from == "html" && to == "txt":
		out = htmlToText(text)
	case from == "txt" && to == "html":
		var b strings.Builder
		for _, para := range strings.Split(strings.TrimSpace(text), "\n\n") {
			if strings.TrimSpace(para) == "" {
				continue
			}
			b.WriteString("<p>")
			b.WriteString(strings.ReplaceAll(html.EscapeString(strings.TrimSpace(para)), "\n", "<br>\n"))
			b.WriteString("</p>\n")
		}
		out = page(titleOf(text, src), b.String())
	case from == "txt" && to == "md":
		out = escapeMarkdown(text)
	default:
		return ErrUnsupported
	}
	_, err = io.WriteString(w, out)
	return err
}

// mdConverter writes HTML as Markdown with GitHub's tables and
// strikethrough.
var mdConverter = converter.NewConverter(converter.WithPlugins(
	base.NewBasePlugin(), commonmark.NewCommonmarkPlugin(), table.NewTablePlugin(), strikethrough.NewStrikethroughPlugin(),
))

func toMarkdown(html string) (string, error) { return mdConverter.ConvertString(html) }

func mdToHTML(src string) (string, error) {
	var b bytes.Buffer
	if err := markdown.Convert([]byte(src), &b); err != nil {
		return "", err
	}
	return b.String(), nil
}

var heading = regexp.MustCompile(`(?m)^#{1,2}\s+(.+?)\s*#*\s*$`)

func titleOf(text, path string) string {
	if m := heading.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	base := path[strings.LastIndexAny(path, `/\`)+1:]
	return strings.TrimSuffix(base, "."+Ext(base))
}

func page(title, body string) string {
	return `<!doctype html>
<html lang="zh">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + html.EscapeString(title) + `</title>
<style>
:root { color-scheme: light dark; --ink: #18181b; --muted: #6b6b74; --line: #e7e5e0; --code: #f4f3ef; --accent: #5b5bd6; }
@media (prefers-color-scheme: dark) { :root { --ink: #ececef; --muted: #9a9ba3; --line: #2a2c33; --code: #17181c; --accent: #9b9bff; } }
body { margin: 0; color: var(--ink); background: Canvas; font: 16px/1.7 -apple-system, "PingFang SC", "Segoe UI", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif; }
main { max-width: 46rem; margin: 0 auto; padding: 3.5rem 1.25rem 5rem; }
h1, h2, h3 { line-height: 1.3; letter-spacing: -0.01em; }
h1 { font-size: 2.1rem; margin-top: 0; }
h2 { margin-top: 2.4rem; padding-bottom: .3rem; border-bottom: 1px solid var(--line); }
a { color: var(--accent); }
code, pre { font-family: ui-monospace, "SF Mono", Menlo, Consolas, monospace; font-size: .9em; background: var(--code); border-radius: 6px; }
code { padding: .15em .35em; }
pre { padding: 1rem 1.1rem; overflow-x: auto; }
pre code { padding: 0; background: none; }
blockquote { margin: 1.2rem 0; padding: .2rem 1rem; border-left: 3px solid var(--line); color: var(--muted); }
table { border-collapse: collapse; width: 100%; display: block; overflow-x: auto; }
th, td { border: 1px solid var(--line); padding: .45rem .7rem; text-align: left; }
img { max-width: 100%; }
hr { border: 0; border-top: 1px solid var(--line); margin: 2.4rem 0; }
</style>
</head>
<body>
<main>
` + body + `</main>
</body>
</html>
`
}

var blocks = map[string]bool{
	"p": true, "div": true, "br": true, "li": true, "tr": true, "h1": true, "h2": true,
	"h3": true, "h4": true, "h5": true, "h6": true, "pre": true, "blockquote": true,
	"section": true, "article": true, "header": true, "footer": true, "hr": true,
	"table": true, "ul": true, "ol": true, "dt": true, "dd": true, "figure": true,
}

// htmlToText keeps what a page reads as: its text, a line for each block,
// a paragraph's break between blocks, and the addresses of links.
func htmlToText(src string) string {
	z := xhtml.NewTokenizer(strings.NewReader(src))
	var b strings.Builder
	skip, pre := 0, 0
	var href string
	space := func() {
		s := b.String()
		if s != "" && !strings.HasSuffix(s, "\n") && !strings.HasSuffix(s, " ") && !strings.HasSuffix(s, "\t") {
			b.WriteByte(' ')
		}
	}
	newline := func(n int) {
		s := strings.TrimRight(b.String(), " ")
		b.Reset()
		b.WriteString(s)
		trail := len(s) - len(strings.TrimRight(s, "\n"))
		for i := trail; i < n && b.Len() > 0; i++ {
			b.WriteByte('\n')
		}
	}
	for {
		tt := z.Next()
		switch tt {
		case xhtml.ErrorToken:
			out := strings.TrimSpace(b.String())
			return regexp.MustCompile(`\n{3,}`).ReplaceAllString(out, "\n\n") + "\n"
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			tag := string(name)
			switch tag {
			case "script", "style", "head", "noscript", "template":
				if tt == xhtml.StartTagToken {
					skip++
				}
			case "pre":
				pre++
			case "a":
				href = ""
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					if string(k) == "href" {
						href = string(v)
					}
				}
			case "li":
				newline(1)
				b.WriteString("• ")
				continue
			case "thead", "tbody":
				continue
			case "td", "th":
				if s := b.String(); s != "" && !strings.HasSuffix(s, "\n") {
					b.WriteString("\t")
				}
			}
			if blocks[tag] {
				if tag == "br" || tag == "tr" {
					newline(1)
				} else {
					newline(2)
				}
			}
		case xhtml.EndTagToken:
			name, _ := z.TagName()
			tag := string(name)
			switch tag {
			case "script", "style", "head", "noscript", "template":
				skip = max(0, skip-1)
			case "pre":
				pre = max(0, pre-1)
			case "a":
				if href != "" && !strings.HasPrefix(href, "#") && !strings.HasPrefix(href, "javascript:") {
					b.WriteString(" (" + href + ")")
				}
				href = ""
			}
			switch {
			case tag == "li" || tag == "tr" || tag == "dt" || tag == "dd":
				newline(1)
			case blocks[tag] && tag != "br":
				newline(2)
			}
		case xhtml.TextToken:
			if skip > 0 {
				continue
			}
			t := string(z.Text())
			if pre == 0 {
				// Runs of white space are one space, kept where they were,
				// and none at the start of a line.
				lead := strings.TrimLeft(t, " \t\r\n") != t
				trail := strings.TrimRight(t, " \t\r\n") != t
				t = strings.Join(strings.Fields(t), " ")
				if t == "" {
					if lead {
						space()
					}
					continue
				}
				if lead {
					space()
				}
				b.WriteString(t)
				if trail {
					space()
				}
				continue
			}
			b.WriteString(t)
		}
	}
}

var mdSpecial = regexp.MustCompile("([\\\\`*_\\[\\]#<>|])")

// escapeMarkdown keeps plain text plain when read as Markdown: its marks
// escaped and its lines kept.
func escapeMarkdown(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		l = mdSpecial.ReplaceAllString(l, `\$1`)
		if i < len(lines)-1 && strings.TrimSpace(l) != "" && strings.TrimSpace(lines[i+1]) != "" {
			l += "  "
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}

// TextSummary counts the characters of a text file, for its row.
func TextSummary(path string) string {
	text, err := readText(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d 字", utf8.RuneCountInString(text))
}
