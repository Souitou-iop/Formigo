package dev.formigo.app.convert

import org.snakeyaml.engine.v2.api.Dump
import org.snakeyaml.engine.v2.api.DumpSettings
import org.snakeyaml.engine.v2.api.Load
import org.snakeyaml.engine.v2.api.LoadSettings
import org.snakeyaml.engine.v2.common.FlowStyle
import org.w3c.dom.Element
import org.w3c.dom.Node
import java.io.ByteArrayInputStream
import java.math.BigDecimal
import java.math.BigInteger
import javax.xml.parsers.DocumentBuilderFactory

/**
 * Structured data, as a tree of ordered maps (LinkedHashMap<String, Any?>),
 * lists, strings, longs, doubles, booleans and nulls, read from and written
 * to each data format.
 */
object DataConvert {
    fun convert(text: String, from: String, to: String, o: Options = Options()): String =
        write(to, parse(from, text), o)

    fun parse(from: String, text: String): Any? = when (from) {
        "json" -> Json.parse(text)
        "yaml" -> parseYaml(text)
        "toml" -> Toml.parse(text)
        "csv" -> Table.parse(text, ',')
        "tsv" -> Table.parse(text, '\t')
        "xml" -> parseXml(text)
        else -> throw ConvertException("不支持的数据格式")
    }

    fun write(to: String, v: Any?, o: Options = Options()): String = when (to) {
        "json" -> Json.write(v, o.indentText) + "\n"
        "yaml" -> writeYaml(v)
        "toml" -> Toml.write(v)
        "csv" -> Table.write(v, ',')
        "tsv" -> Table.write(v, '\t')
        "xml" -> writeXml(v, o.indentText)
        else -> throw ConvertException("不支持的数据格式")
    }

    // YAML, through SnakeYAML Engine, its values made plain.

    private fun parseYaml(text: String): Any? {
        val docs = Load(LoadSettings.builder().build()).loadAllFromString(text).toList()
        val v = if (docs.size == 1) docs[0] else docs
        return plain(v)
    }

    private fun plain(v: Any?): Any? = when (v) {
        null -> null
        is Map<*, *> -> LinkedHashMap<String, Any?>().also { m -> v.forEach { (k, x) -> m[k.toString()] = plain(x) } }
        is Iterable<*> -> v.map { plain(it) }
        is Int -> v.toLong()
        is BigInteger -> if (v.bitLength() < 64) v.toLong() else v.toString()
        is Float -> v.toDouble()
        is BigDecimal -> v.toDouble()
        is ByteArray -> java.util.Base64.getEncoder().encodeToString(v)
        is String, is Long, is Double, is Boolean -> v
        else -> v.toString()
    }

    private fun writeYaml(v: Any?): String {
        val settings = DumpSettings.builder().setDefaultFlowStyle(FlowStyle.BLOCK).setIndent(2)
            .setIndicatorIndent(2).setIndentWithIndicator(true).setWidth(120).build()
        return Dump(settings).dumpToString(v)
    }

    // XML: an element is a map of its attributes ("@name"), its children by
    // name (a list when repeated) and its text ("#text", or the element is
    // only its text).

    private fun parseXml(text: String): Any? {
        val f = DocumentBuilderFactory.newInstance().apply {
            isNamespaceAware = false
            isCoalescing = true
            isIgnoringComments = true
            try {
                setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)
            } catch (_: Exception) {
            }
        }
        val doc = try {
            f.newDocumentBuilder().parse(ByteArrayInputStream(text.toByteArray()))
        } catch (e: Exception) {
            throw ConvertException("XML 无法解析：${e.message}")
        }
        val root = doc.documentElement
        return linkedMapOf<String, Any?>(root.tagName to element(root))
    }

    private fun element(e: Element): Any? {
        val m = LinkedHashMap<String, Any?>()
        val attrs = e.attributes
        for (i in 0 until attrs.length) {
            val a = attrs.item(i)
            m["@" + a.nodeName] = Table.cell(a.nodeValue)
        }
        val text = StringBuilder()
        val kids = e.childNodes
        for (i in 0 until kids.length) {
            val n = kids.item(i)
            when (n.nodeType) {
                Node.ELEMENT_NODE -> {
                    val child = n as Element
                    val v = element(child)
                    when (val had = m[child.tagName]) {
                        null -> if (m.containsKey(child.tagName)) m[child.tagName] = mutableListOf(null, v) else m[child.tagName] = v
                        is MutableList<*> -> @Suppress("UNCHECKED_CAST") (had as MutableList<Any?>).add(v)
                        else -> m[child.tagName] = mutableListOf(had, v)
                    }
                }
                Node.TEXT_NODE, Node.CDATA_SECTION_NODE -> text.append(n.nodeValue)
            }
        }
        val t = text.toString().trim()
        if (m.isEmpty()) return if (t.isEmpty()) null else Table.cell(t)
        if (t.isNotEmpty()) m["#text"] = Table.cell(t)
        return m
    }

    private fun writeXml(v: Any?, indent: String): String {
        val b = StringBuilder("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
        if (v is Map<*, *> && v.size == 1 && v.values.first() !is List<*>) {
            val (k, x) = v.entries.first()
            xmlNode(b, tagName(k.toString()), x, indent, 0)
        } else {
            xmlNode(b, "root", v, indent, 0)
        }
        return b.toString()
    }

    private fun tagName(k: String): String {
        val s = k.replace(Regex("[^A-Za-z0-9_.\\-]"), "_")
        return if (s.isEmpty() || !(s[0].isLetter() || s[0] == '_')) "_$s" else s
    }

    private fun xmlNode(b: StringBuilder, tag: String, v: Any?, indent: String, depth: Int) {
        val pad = indent.repeat(depth)
        when (v) {
            is List<*> -> v.forEach { xmlNode(b, tag, it, indent, depth) }
            is Map<*, *> -> {
                b.append(pad).append('<').append(tag)
                val kids = v.entries.filter { !it.key.toString().startsWith("@") && it.key != "#text" }
                v.entries.filter { it.key.toString().startsWith("@") }.forEach { (k, x) ->
                    b.append(' ').append(tagName(k.toString().drop(1))).append("=\"").append(escapeXml(scalarText(x), true)).append('"')
                }
                val text = v["#text"]
                when {
                    kids.isEmpty() && text == null -> b.append("/>\n")
                    kids.isEmpty() -> b.append('>').append(escapeXml(scalarText(text), false)).append("</").append(tag).append(">\n")
                    else -> {
                        b.append(">\n")
                        if (text != null) b.append(pad).append(indent).append(escapeXml(scalarText(text), false)).append('\n')
                        kids.forEach { (k, x) -> xmlNode(b, tagName(k.toString()), x, indent, depth + 1) }
                        b.append(pad).append("</").append(tag).append(">\n")
                    }
                }
            }
            null -> b.append(pad).append('<').append(tag).append("/>\n")
            else -> b.append(pad).append('<').append(tag).append('>').append(escapeXml(scalarText(v), false))
                .append("</").append(tag).append(">\n")
        }
    }

    private fun scalarText(v: Any?): String = when (v) {
        null -> ""
        is Double -> Json.number(v)
        is Map<*, *>, is List<*> -> Json.write(v, "")
        else -> v.toString()
    }

    private fun escapeXml(s: String, attr: Boolean): String {
        val b = StringBuilder(s.length)
        for (c in s) when (c) {
            '&' -> b.append("&amp;")
            '<' -> b.append("&lt;")
            '>' -> b.append("&gt;")
            '"' -> if (attr) b.append("&quot;") else b.append(c)
            else -> b.append(c)
        }
        return b.toString()
    }

    /** What a data file holds, in a few words, for its row. */
    fun summary(v: Any?): String = when (v) {
        is Map<*, *> -> "${v.size} 个键"
        is List<*> -> "${v.size} 项"
        null -> "空"
        else -> "单个值"
    }
}

/** JSON, read keeping the order of keys, written with the indent chosen. */
object Json {
    fun parse(text: String): Any? {
        val p = Parser(text)
        p.ws()
        val v = p.value()
        p.ws()
        if (p.i < text.length) p.fail("多余的内容")
        return v
    }

    private class Parser(val s: String) {
        var i = 0
        fun fail(msg: String): Nothing {
            val line = s.substring(0, minOf(i, s.length)).count { it == '\n' } + 1
            throw ConvertException("JSON 第 $line 行：$msg")
        }

        fun ws() {
            while (i < s.length && s[i].isWhitespace()) i++
        }

        fun value(): Any? {
            if (i >= s.length) fail("意外结束")
            return when (val c = s[i]) {
                '{' -> obj()
                '[' -> arr()
                '"' -> str()
                't' -> word("true", true)
                'f' -> word("false", false)
                'n' -> word("null", null)
                else -> if (c == '-' || c.isDigit()) num() else fail("无法识别的字符 '$c'")
            }
        }

        fun word(w: String, v: Any?): Any? {
            if (!s.startsWith(w, i)) fail("无法识别的值")
            i += w.length
            return v
        }

        fun obj(): Map<String, Any?> {
            val m = LinkedHashMap<String, Any?>()
            i++
            ws()
            if (s.getOrNull(i) == '}') { i++; return m }
            while (true) {
                ws()
                if (s.getOrNull(i) != '"') fail("应为键名")
                val k = str()
                ws()
                if (s.getOrNull(i) != ':') fail("应为 ':'")
                i++
                ws()
                m[k] = value()
                ws()
                when (s.getOrNull(i)) {
                    ',' -> i++
                    '}' -> { i++; return m }
                    else -> fail("应为 ',' 或 '}'")
                }
            }
        }

        fun arr(): List<Any?> {
            val l = ArrayList<Any?>()
            i++
            ws()
            if (s.getOrNull(i) == ']') { i++; return l }
            while (true) {
                ws()
                l.add(value())
                ws()
                when (s.getOrNull(i)) {
                    ',' -> i++
                    ']' -> { i++; return l }
                    else -> fail("应为 ',' 或 ']'")
                }
            }
        }

        fun str(): String {
            val b = StringBuilder()
            i++
            while (true) {
                if (i >= s.length) fail("字符串未结束")
                val c = s[i++]
                when (c) {
                    '"' -> return b.toString()
                    '\\' -> {
                        val e = s.getOrNull(i++) ?: fail("字符串未结束")
                        when (e) {
                            'n' -> b.append('\n'); 't' -> b.append('\t'); 'r' -> b.append('\r')
                            'b' -> b.append('\b'); 'f' -> b.append('\u000c')
                            'u' -> {
                                if (i + 4 > s.length) fail("转义不完整")
                                b.append(s.substring(i, i + 4).toInt(16).toChar()); i += 4
                            }
                            else -> b.append(e)
                        }
                    }
                    else -> b.append(c)
                }
            }
        }

        fun num(): Any {
            val start = i
            if (s[i] == '-') i++
            while (i < s.length && (s[i].isDigit() || s[i] in ".eE+-")) i++
            val t = s.substring(start, i)
            return t.toLongOrNull() ?: t.toDoubleOrNull() ?: fail("无效的数字")
        }
    }

    fun number(d: Double): String =
        if (d.isNaN() || d.isInfinite()) "null"
        else if (d == Math.floor(d) && kotlin.math.abs(d) < 1e15) d.toLong().toString()
        else d.toString()

    fun write(v: Any?, indent: String): String = StringBuilder().also { write(it, v, indent, "") }.toString()

    private fun write(b: StringBuilder, v: Any?, indent: String, prefix: String) {
        val nl = if (indent.isEmpty()) "" else "\n"
        val sep = if (indent.isEmpty()) ":" else ": "
        when (v) {
            null -> b.append("null")
            is String -> string(b, v)
            is Boolean, is Long, is Int -> b.append(v.toString())
            is Double -> b.append(number(v))
            is Number -> b.append(v.toString())
            is Map<*, *> -> {
                if (v.isEmpty()) { b.append("{}"); return }
                b.append('{').append(nl)
                var first = true
                for ((k, x) in v) {
                    if (!first) b.append(',').append(nl)
                    first = false
                    b.append(prefix).append(indent)
                    string(b, k.toString())
                    b.append(sep)
                    write(b, x, indent, prefix + indent)
                }
                b.append(nl).append(prefix).append('}')
            }
            is List<*> -> {
                if (v.isEmpty()) { b.append("[]"); return }
                b.append('[').append(nl)
                v.forEachIndexed { n, x ->
                    if (n > 0) b.append(',').append(nl)
                    b.append(prefix).append(indent)
                    write(b, x, indent, prefix + indent)
                }
                b.append(nl).append(prefix).append(']')
            }
            else -> string(b, v.toString())
        }
    }

    fun string(b: StringBuilder, s: String) {
        b.append('"')
        for (c in s) when {
            c == '"' -> b.append("\\\"")
            c == '\\' -> b.append("\\\\")
            c == '\n' -> b.append("\\n")
            c == '\r' -> b.append("\\r")
            c == '\t' -> b.append("\\t")
            c < ' ' -> b.append(String.format("\\u%04x", c.code))
            else -> b.append(c)
        }
        b.append('"')
    }
}

/** CSV and TSV: a table is a list of maps, its first row the keys. */
object Table {
    fun parse(text: String, sep: Char): List<Map<String, Any?>> {
        val rows = rows(text.removePrefix("﻿"), sep).filter { r -> r.any { it.isNotEmpty() } }
        if (rows.isEmpty()) return emptyList()
        val head = rows[0].mapIndexed { i, h -> h.ifBlank { "列${i + 1}" } }
        return rows.drop(1).map { r ->
            LinkedHashMap<String, Any?>().also { m -> head.forEachIndexed { i, h -> m[h] = cell(r.getOrElse(i) { "" }) } }
        }
    }

    private fun rows(s: String, sep: Char): List<List<String>> {
        val out = ArrayList<List<String>>()
        var row = ArrayList<String>()
        val f = StringBuilder()
        var i = 0
        var quoted = false
        while (i < s.length) {
            val c = s[i]
            if (quoted) {
                if (c == '"') {
                    if (s.getOrNull(i + 1) == '"') { f.append('"'); i++ } else quoted = false
                } else f.append(c)
            } else when (c) {
                '"' -> if (f.isEmpty()) quoted = true else f.append(c)
                sep -> { row.add(f.toString()); f.setLength(0) }
                '\r' -> {}
                '\n' -> { row.add(f.toString()); f.setLength(0); out.add(row); row = ArrayList() }
                else -> f.append(c)
            }
            i++
        }
        if (f.isNotEmpty() || row.isNotEmpty()) { row.add(f.toString()); out.add(row) }
        return out
    }

    /** A cell's value: a number or boolean when it reads as one, else its text. */
    fun cell(s: String): Any? {
        val t = s.trim()
        if (t.isEmpty()) return ""
        if (t == "true") return true
        if (t == "false") return false
        if (Regex("-?(0|[1-9][0-9]{0,17})").matches(t)) return t.toLong()
        if (Regex("-?(0|[1-9][0-9]*)(\\.[0-9]+)?([eE][+-]?[0-9]+)?").matches(t) && t.length < 25) return t.toDouble()
        return s
    }

    fun write(v: Any?, sep: Char): String {
        // A map holding one list of records is that list; any other map, its pairs.
        val pairs = v is Map<*, *> && !(v.size == 1 && v.values.first() is List<*>)
        val rows: List<Any?> = when (v) {
            is List<*> -> v
            is Map<*, *> -> if (pairs) v.map { (k, x) -> linkedMapOf("key" to k.toString(), "value" to x) } else v.values.first() as List<*>
            null -> emptyList()
            else -> listOf(linkedMapOf("value" to v))
        }
        val keys = LinkedHashSet<String>()
        val flat = rows.map { r ->
            val m = LinkedHashMap<String, Any?>()
            when (r) {
                is Map<*, *> -> if (pairs) r.forEach { (k, x) -> m[k.toString()] = x } else flatten(m, "", r)
                is List<*> -> r.forEachIndexed { i, x -> m["${i + 1}"] = x }
                else -> m["value"] = r
            }
            keys.addAll(m.keys)
            m
        }
        val b = StringBuilder()
        b.append(keys.joinToString(sep.toString()) { field(it, sep) }).append('\n')
        for (m in flat) b.append(keys.joinToString(sep.toString()) { field(text(m[it]), sep) }).append('\n')
        return b.toString()
    }

    private fun flatten(out: MutableMap<String, Any?>, prefix: String, m: Map<*, *>) {
        for ((k, x) in m) {
            val key = if (prefix.isEmpty()) k.toString() else "$prefix.$k"
            if (x is Map<*, *> && x.isNotEmpty()) flatten(out, key, x) else out[key] = x
        }
    }

    private fun text(v: Any?): String = when (v) {
        null -> ""
        is Double -> Json.number(v)
        is Map<*, *>, is List<*> -> Json.write(v, "")
        else -> v.toString()
    }

    private fun field(s: String, sep: Char): String {
        if (sep == '\t') return s.replace('\t', ' ').replace('\n', ' ')
        return if (s.any { it == ',' || it == '"' || it == '\n' || it == '\r' } || s != s.trim())
            "\"" + s.replace("\"", "\"\"") + "\"" else s
    }
}
