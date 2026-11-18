package dev.formigo.app.convert

/** Markdown, HTML and plain text, each to the others. */
object DocConvert {
    fun convert(text: String, from: String, to: String, title: String): String {
        val src = text.removePrefix("\uFEFF")
        return when (from to to) {
            "md" to "html" -> page(title, Markdown.toHtml(src))
            "md" to "txt" -> Html.toText(Markdown.toHtml(src))
            "html" to "md" -> Html.toMarkdown(src)
            "html" to "txt" -> Html.toText(src)
            "txt" to "html" -> page(title, src.split(Regex("\\n\\s*\\n")).filter { it.isNotBlank() }
                .joinToString("\n") { "<p>" + Html.escape(it.trim()).replace("\n", "<br>\n") + "</p>" })
            "txt" to "md" -> Markdown.escape(src)
            else -> throw ConvertException("不支持的文档转换")
        }
    }

    /** A plain text rendering of any text document, for writing it as a PDF. */
    fun plainText(text: String, from: String): String = when (from) {
        "md" -> Html.toText(Markdown.toHtml(text))
        "html" -> Html.toText(text)
        else -> text
    }

    fun page(title: String, body: String): String = """<!doctype html>
<html lang="zh">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${Html.escape(title)}</title>
<style>
body { max-width: 760px; margin: 40px auto; padding: 0 20px; font: 16px/1.7 -apple-system, "Segoe UI", Roboto, "Noto Sans SC", sans-serif; color: #14213d; }
pre, code { font-family: ui-monospace, Menlo, Consolas, monospace; background: #f2f6fc; border-radius: 6px; }
pre { padding: 12px 16px; overflow-x: auto; } code { padding: 2px 5px; } pre code { padding: 0; }
blockquote { margin: 0; padding-left: 16px; border-left: 4px solid #5cb9f7; color: #56627e; }
table { border-collapse: collapse; } td, th { border: 1px solid #d5dcea; padding: 6px 10px; }
img { max-width: 100%; } a { color: #1f6bff; }
</style>
</head>
<body>
$body
</body>
</html>
"""
}

/** A CommonMark-ish Markdown renderer: headings, paragraphs, lists, quotes, code, rules, tables and inlines. */
object Markdown {
    fun toHtml(src: String): String {
        val out = StringBuilder()
        block(src.replace("\r\n", "\n").split('\n'), out)
        return out.toString().trimEnd()
    }

    private val heading = Regex("^(#{1,6})\\s+(.*?)\\s*#*\\s*$")
    private val bullet = Regex("^\\s{0,3}([-*+])\\s+(.*)$")
    private val ordered = Regex("^\\s{0,3}(\\d{1,9})[.)]\\s+(.*)$")
    private val rule = Regex("^\\s{0,3}([-*_])(\\s*\\1){2,}\\s*$")
    private val fence = Regex("^\\s{0,3}(```|~~~)\\s*([\\w+-]*).*$")

    private fun block(lines: List<String>, out: StringBuilder) {
        var i = 0
        val para = ArrayList<String>()
        fun flush() {
            if (para.isNotEmpty()) {
                out.append("<p>").append(inline(para.joinToString("\n") { it.trim() })).append("</p>\n")
                para.clear()
            }
        }
        while (i < lines.size) {
            val line = lines[i]
            when {
                line.isBlank() -> { flush(); i++ }
                fence.matches(line) -> {
                    flush()
                    val m = fence.find(line)!!
                    val lang = m.groupValues[2]
                    val code = ArrayList<String>()
                    i++
                    while (i < lines.size && !lines[i].trim().startsWith(m.groupValues[1])) code.add(lines[i++])
                    i++
                    out.append("<pre><code").append(if (lang.isNotEmpty()) " class=\"language-$lang\"" else "").append('>')
                        .append(Html.escape(code.joinToString("\n"))).append("</code></pre>\n")
                }
                heading.matches(line) -> {
                    flush()
                    val m = heading.find(line)!!
                    val n = m.groupValues[1].length
                    out.append("<h$n>").append(inline(m.groupValues[2])).append("</h$n>\n")
                    i++
                }
                rule.matches(line) -> { flush(); out.append("<hr>\n"); i++ }
                para.isNotEmpty() && Regex("^\\s{0,3}(=+|-+)\\s*$").matches(line) -> {
                    val n = if (line.trim().startsWith("=")) 1 else 2
                    val text = para.joinToString(" ") { it.trim() }
                    para.clear()
                    out.append("<h$n>").append(inline(text)).append("</h$n>\n")
                    i++
                }
                line.trimStart().startsWith(">") -> {
                    flush()
                    val quote = ArrayList<String>()
                    while (i < lines.size && lines[i].trimStart().startsWith(">")) {
                        quote.add(lines[i].trimStart().removePrefix(">").removePrefix(" ")); i++
                    }
                    out.append("<blockquote>\n")
                    block(quote, out)
                    out.append("</blockquote>\n")
                }
                bullet.matches(line) || ordered.matches(line) -> {
                    flush()
                    val isOrdered = ordered.matches(line)
                    val re = if (isOrdered) ordered else bullet
                    val start = if (isOrdered) ordered.find(line)!!.groupValues[1].toInt() else 1
                    out.append(if (isOrdered) (if (start != 1) "<ol start=\"$start\">\n" else "<ol>\n") else "<ul>\n")
                    while (i < lines.size && re.matches(lines[i])) {
                        val item = arrayListOf(re.find(lines[i])!!.groupValues[2])
                        i++
                        while (i < lines.size && lines[i].isNotBlank() && !re.matches(lines[i]) &&
                            (lines[i].startsWith("  ") || lines[i].startsWith("\t"))
                        ) item.add(lines[i++].trim())
                        val text = item.joinToString("\n")
                        val task = Regex("^\\[([ xX])]\\s+(.*)", RegexOption.DOT_MATCHES_ALL).find(text)
                        out.append("<li>")
                        if (task != null) {
                            out.append("<input type=\"checkbox\" disabled").append(if (task.groupValues[1] != " ") " checked" else "")
                                .append("> ").append(inline(task.groupValues[2]))
                        } else out.append(inline(text))
                        out.append("</li>\n")
                        while (i < lines.size && lines[i].isBlank() && i + 1 < lines.size && re.matches(lines[i + 1])) i++
                    }
                    out.append(if (isOrdered) "</ol>\n" else "</ul>\n")
                }
                line.trim().startsWith("|") && i + 1 < lines.size && Regex("^\\s*\\|?\\s*:?-+:?\\s*(\\|\\s*:?-+:?\\s*)*\\|?\\s*$").matches(lines[i + 1]) -> {
                    flush()
                    fun cells(l: String) = l.trim().removePrefix("|").removeSuffix("|").split('|').map { it.trim() }
                    out.append("<table>\n<thead><tr>")
                    cells(line).forEach { out.append("<th>").append(inline(it)).append("</th>") }
                    out.append("</tr></thead>\n<tbody>\n")
                    i += 2
                    while (i < lines.size && lines[i].trim().startsWith("|")) {
                        out.append("<tr>")
                        cells(lines[i]).forEach { out.append("<td>").append(inline(it)).append("</td>") }
                        out.append("</tr>\n")
                        i++
                    }
                    out.append("</tbody>\n</table>\n")
                }
                line.startsWith("    ") && para.isEmpty() -> {
                    val code = ArrayList<String>()
                    while (i < lines.size && (lines[i].startsWith("    ") || lines[i].isBlank())) code.add(lines[i++].removePrefix("    "))
                    out.append("<pre><code>").append(Html.escape(code.joinToString("\n").trimEnd())).append("</code></pre>\n")
                }
                else -> { para.add(line); i++ }
            }
        }
        flush()
    }

    fun inline(s: String): String {
        val b = StringBuilder()
        var i = 0
        while (i < s.length) {
            val c = s[i]
            when {
                c == '\\' && i + 1 < s.length && s[i + 1] in "\\`*_{}[]()#+-.!|>~" -> { b.append(Html.escape(s[i + 1].toString())); i += 2 }
                c == '`' -> {
                    val end = s.indexOf('`', i + 1)
                    if (end > 0) { b.append("<code>").append(Html.escape(s.substring(i + 1, end))).append("</code>"); i = end + 1 } else { b.append('`'); i++ }
                }
                c == '!' && s.startsWith("![", i) -> {
                    val link = link(s, i + 1)
                    if (link != null) {
                        b.append("<img src=\"").append(Html.escape(link.second)).append("\" alt=\"").append(Html.escape(link.first)).append("\">")
                        i = link.third
                    } else { b.append('!'); i++ }
                }
                c == '[' -> {
                    val link = link(s, i)
                    if (link != null) {
                        b.append("<a href=\"").append(Html.escape(link.second)).append("\">").append(inline(link.first)).append("</a>")
                        i = link.third
                    } else { b.append('['); i++ }
                }
                c == '<' && Regex("^<(https?://[^>\\s]+)>").find(s.substring(i)) != null -> {
                    val url = Regex("^<(https?://[^>\\s]+)>").find(s.substring(i))!!.groupValues[1]
                    b.append("<a href=\"").append(Html.escape(url)).append("\">").append(Html.escape(url)).append("</a>")
                    i += url.length + 2
                }
                (c == '*' || c == '_') && s.startsWith("$c$c", i) -> {
                    val end = s.indexOf("$c$c", i + 2)
                    if (end > i + 2) { b.append("<strong>").append(inline(s.substring(i + 2, end))).append("</strong>"); i = end + 2 } else { b.append(c); i++ }
                }
                c == '~' && s.startsWith("~~", i) -> {
                    val end = s.indexOf("~~", i + 2)
                    if (end > i + 2) { b.append("<del>").append(inline(s.substring(i + 2, end))).append("</del>"); i = end + 2 } else { b.append(c); i++ }
                }
                (c == '*' || c == '_') && i + 1 < s.length && !s[i + 1].isWhitespace() -> {
                    val end = s.indexOf(c, i + 1)
                    val wordy = c == '_' && i > 0 && s[i - 1].isLetterOrDigit()
                    if (end > i + 1 && !wordy) { b.append("<em>").append(inline(s.substring(i + 1, end))).append("</em>"); i = end + 1 } else { b.append(c); i++ }
                }
                c == '\n' -> { if (b.endsWith("  ")) { b.setLength(b.length - 2); b.append("<br>") }; b.append('\n'); i++ }
                else -> { b.append(Html.escape(c.toString())); i++ }
            }
        }
        return b.toString()
    }

    /** A link at s[i] == '[': its text, its target and the index past it. */
    private fun link(s: String, i: Int): Triple<String, String, Int>? {
        if (s.getOrNull(i) != '[') return null
        var depth = 0
        var j = i
        while (j < s.length) {
            if (s[j] == '[') depth++
            if (s[j] == ']') { depth--; if (depth == 0) break }
            j++
        }
        if (j >= s.length || s.getOrNull(j + 1) != '(') return null
        val end = s.indexOf(')', j + 2)
        if (end < 0) return null
        val target = s.substring(j + 2, end).trim().split(Regex("\\s+"))[0].removeSurrounding("<", ">")
        return Triple(s.substring(i + 1, j), target, end + 1)
    }

    fun escape(text: String): String = text.lines().joinToString("\n") { line ->
        val l = line.replace("\\", "\\\\").replace("*", "\\*").replace("_", "\\_").replace("`", "\\`")
        if (Regex("^\\s*(#|>|[-+]\\s|\\d+\\.\\s)").containsMatchIn(l)) "\\" + l.trimStart() else l
    }
}

/** HTML read loosely, as browsers do, into text or Markdown. */
object Html {
    fun escape(s: String): String {
        val b = StringBuilder(s.length)
        for (c in s) when (c) {
            '&' -> b.append("&amp;"); '<' -> b.append("&lt;"); '>' -> b.append("&gt;"); '"' -> b.append("&quot;")
            else -> b.append(c)
        }
        return b.toString()
    }

    private val entities = mapOf(
        "amp" to "&", "lt" to "<", "gt" to ">", "quot" to "\"", "apos" to "'", "nbsp" to "\u00a0",
        "copy" to "©", "reg" to "®", "hellip" to "…", "mdash" to "—", "ndash" to "–", "laquo" to "«", "raquo" to "»",
        "ldquo" to "\u201c", "rdquo" to "\u201d", "lsquo" to "\u2018", "rsquo" to "\u2019", "middot" to "·", "times" to "×",
    )

    fun unescape(s: String): String = Regex("&(#x[0-9a-fA-F]+|#\\d+|[a-zA-Z]+);").replace(s) { m ->
        val e = m.groupValues[1]
        when {
            e.startsWith("#x") -> e.drop(2).toIntOrNull(16)?.let { String(Character.toChars(it)) } ?: m.value
            e.startsWith("#") -> e.drop(1).toIntOrNull()?.let { String(Character.toChars(it)) } ?: m.value
            else -> entities[e] ?: m.value
        }
    }

    /** A node of the loose tree: an element or a run of text. */
    sealed class N {
        class Text(val text: String) : N()
        class El(val tag: String, val attrs: Map<String, String>, val kids: MutableList<N> = ArrayList()) : N()
    }

    private val voids = setOf("br", "hr", "img", "input", "meta", "link", "area", "base", "col", "embed", "source", "track", "wbr")
    private val skipped = setOf("script", "style", "head", "noscript", "template", "svg")

    fun parse(src: String): N.El {
        val root = N.El("#root", emptyMap())
        val stack = ArrayList<N.El>().apply { add(root) }
        var i = 0
        val tagRe = Regex("^<(/?)([a-zA-Z][a-zA-Z0-9-]*)([^>]*?)(/?)>", RegexOption.DOT_MATCHES_ALL)
        val attrRe = Regex("([a-zA-Z_:][-a-zA-Z0-9_:.]*)(?:\\s*=\\s*(\"[^\"]*\"|'[^']*'|[^\\s\"'>]+))?")
        while (i < src.length) {
            if (src.startsWith("<!--", i)) {
                val end = src.indexOf("-->", i + 4)
                i = if (end < 0) src.length else end + 3
                continue
            }
            if (src.startsWith("<!", i) || src.startsWith("<?", i)) {
                val end = src.indexOf('>', i)
                i = if (end < 0) src.length else end + 1
                continue
            }
            if (src[i] == '<') {
                val m = tagRe.find(src.substring(i, minOf(src.length, i + 4000)))
                if (m != null) {
                    val tag = m.groupValues[2].lowercase()
                    i += m.value.length
                    if (m.groupValues[1] == "/") {
                        val at = stack.indexOfLast { it.tag == tag }
                        if (at > 0) while (stack.size > at) stack.removeAt(stack.size - 1)
                        continue
                    }
                    val attrs = attrRe.findAll(m.groupValues[3]).associate {
                        it.groupValues[1].lowercase() to unescape(it.groupValues[2].removeSurrounding("\"").removeSurrounding("'"))
                    }
                    if (tag in skipped) {
                        val end = src.indexOf("</$tag", i, ignoreCase = true)
                        i = if (end < 0) src.length else src.indexOf('>', end).let { if (it < 0) src.length else it + 1 }
                        continue
                    }
                    // A new item or paragraph closes the one open.
                    if (tag == "li" || tag == "p" || tag == "tr" || tag == "td" || tag == "th") {
                        val closes = when (tag) { "li" -> setOf("li"); "tr" -> setOf("tr", "td", "th"); "td", "th" -> setOf("td", "th"); else -> setOf("p") }
                        val at = stack.indexOfLast { it.tag in closes }
                        val fence = stack.indexOfLast { it.tag in setOf("ul", "ol", "table", "tbody", "thead", "tr", "div", "blockquote") }
                        if (at > 0 && at > fence) while (stack.size > at) stack.removeAt(stack.size - 1)
                    }
                    val el = N.El(tag, attrs)
                    stack.last().kids.add(el)
                    if (tag !in voids && m.groupValues[4] != "/") stack.add(el)
                    continue
                }
            }
            val next = src.indexOf('<', i + 1).let { if (it < 0) src.length else it }
            stack.last().kids.add(N.Text(unescape(src.substring(i, next))))
            i = next
        }
        return root
    }

    private val blocks = setOf(
        "p", "div", "section", "article", "header", "footer", "main", "nav", "aside", "h1", "h2", "h3", "h4", "h5", "h6",
        "ul", "ol", "li", "blockquote", "pre", "table", "tr", "hr", "br", "figure", "figcaption", "dl", "dt", "dd", "title", "body", "html",
    )

    fun toText(src: String): String {
        val b = StringBuilder()
        text(parse(src), b, false)
        return tidy(b.toString())
    }

    private fun text(n: N, b: StringBuilder, pre: Boolean) {
        when (n) {
            is N.Text -> b.append(if (pre) n.text else n.text.replace(Regex("\\s+"), " "))
            is N.El -> {
                if (n.tag in blocks) b.append('\n')
                if (n.tag == "li") b.append("• ")
                if (n.tag == "td" || n.tag == "th") b.append('\t')
                n.kids.forEach { text(it, b, pre || n.tag == "pre") }
                if (n.tag in blocks) b.append('\n')
                if (n.tag in setOf("p", "h1", "h2", "h3", "h4", "h5", "h6", "pre", "blockquote", "table", "ul", "ol")) b.append('\n')
            }
        }
    }

    private fun tidy(s: String): String = s.lines().joinToString("\n") { it.trimEnd().replace(Regex("^ +"), "").replace("\u00a0", " ") }
        .replace(Regex("\n{3,}"), "\n\n").trim() + "\n"

    fun toMarkdown(src: String): String {
        val b = StringBuilder()
        md(parse(src), b, ListCtx(null, 0))
        return tidy(b.toString()).lines().joinToString("\n") { it }.replace(Regex("\n{3,}"), "\n\n")
    }

    private class ListCtx(val ordered: Boolean?, var n: Int, val depth: Int = 0)

    private fun inlineMd(n: N.El): String {
        val b = StringBuilder()
        n.kids.forEach { md(it, b, ListCtx(null, 0)) }
        return b.toString().trim()
    }

    private fun md(n: N, b: StringBuilder, list: ListCtx) {
        when (n) {
            is N.Text -> b.append(n.text.replace(Regex("\\s+"), " ").replace("*", "\\*").replace("_", "\\_"))
            is N.El -> when (n.tag) {
                "h1", "h2", "h3", "h4", "h5", "h6" -> b.append("\n\n").append("#".repeat(n.tag[1] - '0')).append(' ').append(inlineMd(n)).append("\n\n")
                "p", "div", "section", "article", "header", "footer", "main", "figure" -> { b.append("\n\n"); n.kids.forEach { md(it, b, list) }; b.append("\n\n") }
                "br" -> b.append("  \n")
                "hr" -> b.append("\n\n---\n\n")
                "strong", "b" -> inlineMd(n).let { if (it.isNotEmpty()) b.append("**").append(it).append("**") }
                "em", "i" -> inlineMd(n).let { if (it.isNotEmpty()) b.append('*').append(it).append('*') }
                "del", "s" -> inlineMd(n).let { if (it.isNotEmpty()) b.append("~~").append(it).append("~~") }
                "code" -> b.append('`').append(plain(n)).append('`')
                "pre" -> b.append("\n\n```\n").append(plain(n).trimEnd('\n')).append("\n```\n\n")
                "a" -> b.append('[').append(inlineMd(n)).append("](").append(n.attrs["href"] ?: "").append(')')
                "img" -> b.append("![").append(n.attrs["alt"] ?: "").append("](").append(n.attrs["src"] ?: "").append(')')
                "ul", "ol" -> {
                    val inner = ListCtx(n.tag == "ol", 0, if (list.ordered == null) 0 else list.depth + 1)
                    b.append("\n")
                    n.kids.forEach { md(it, b, inner) }
                    b.append("\n")
                }
                "li" -> {
                    list.n++
                    val pad = "  ".repeat(list.depth)
                    b.append('\n').append(pad).append(if (list.ordered == true) "${list.n}. " else "- ")
                    val item = StringBuilder()
                    n.kids.forEach { md(it, item, list) }
                    b.append(item.toString().trim().replace(Regex("\n{2,}"), "\n"))
                }
                "blockquote" -> {
                    val inner = StringBuilder()
                    n.kids.forEach { md(it, inner, list) }
                    b.append("\n\n").append(tidy(inner.toString()).trim().lines().joinToString("\n") { "> $it".trimEnd() }).append("\n\n")
                }
                "table" -> table(n, b)
                else -> n.kids.forEach { md(it, b, list) }
            }
        }
    }

    private fun plain(n: N): String = when (n) {
        is N.Text -> n.text
        is N.El -> if (n.tag == "br") "\n" else n.kids.joinToString("") { plain(it) }
    }

    private fun table(n: N.El, b: StringBuilder) {
        val rows = ArrayList<List<String>>()
        fun walk(e: N.El) {
            if (e.tag == "tr") rows.add(e.kids.filterIsInstance<N.El>().filter { it.tag == "td" || it.tag == "th" }.map { inlineMd(it).replace("|", "\\|") })
            else e.kids.filterIsInstance<N.El>().forEach { walk(it) }
        }
        walk(n)
        if (rows.isEmpty()) return
        val w = rows.maxOf { it.size }
        b.append("\n\n")
        rows.forEachIndexed { i, r ->
            b.append("| ").append((r + List(w - r.size) { "" }).joinToString(" | ")).append(" |\n")
            if (i == 0) b.append("|").append(" --- |".repeat(w)).append('\n')
        }
        b.append('\n')
    }
}
