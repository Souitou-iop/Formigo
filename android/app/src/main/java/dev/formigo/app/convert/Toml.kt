package dev.formigo.app.convert

/** TOML 1.0: tables, arrays of tables, inline tables, arrays and scalars. Dates are kept as text. */
object Toml {
    fun parse(text: String): Map<String, Any?> = Parser(text).doc()

    private class Parser(val s: String) {
        var i = 0
        val root = LinkedHashMap<String, Any?>()
        var line = 1

        fun fail(msg: String): Nothing = throw ConvertException("TOML 第 ${s.substring(0, minOf(i, s.length)).count { it == '\n' } + 1} 行：$msg")

        fun spaces() { while (i < s.length && (s[i] == ' ' || s[i] == '\t')) i++ }

        fun skipComment() { if (s.getOrNull(i) == '#') while (i < s.length && s[i] != '\n') i++ }

        /** Whitespace, newlines and comments, as between array values. */
        fun blank() {
            while (i < s.length) {
                when (s[i]) {
                    ' ', '\t', '\r', '\n' -> i++
                    '#' -> skipComment()
                    else -> return
                }
            }
        }

        fun endOfLine() {
            spaces(); skipComment()
            if (s.getOrNull(i) == '\r') i++
            if (i < s.length && s[i] != '\n') fail("行尾有多余内容")
            if (i < s.length) i++
        }

        @Suppress("UNCHECKED_CAST")
        fun doc(): Map<String, Any?> {
            var table: MutableMap<String, Any?> = root
            while (true) {
                blank()
                if (i >= s.length) return root
                if (s[i] == '[') {
                    val array = s.getOrNull(i + 1) == '['
                    i += if (array) 2 else 1
                    spaces()
                    val keys = key()
                    spaces()
                    if (array) {
                        if (!s.startsWith("]]", i)) fail("应为 ']]'")
                        i += 2
                        val parent = descend(root, keys.dropLast(1))
                        val list = parent.getOrPut(keys.last()) { ArrayList<Any?>() } as? MutableList<Any?> ?: fail("键 ${keys.last()} 重复")
                        val t = LinkedHashMap<String, Any?>()
                        list.add(t)
                        table = t
                    } else {
                        if (s.getOrNull(i) != ']') fail("应为 ']'")
                        i++
                        table = descend(root, keys)
                    }
                    endOfLine()
                    continue
                }
                val keys = key()
                spaces()
                if (s.getOrNull(i) != '=') fail("应为 '='")
                i++
                spaces()
                val v = value()
                val parent = descend(table, keys.dropLast(1))
                if (parent.containsKey(keys.last())) fail("键 ${keys.last()} 重复")
                parent[keys.last()] = v
                endOfLine()
            }
        }

        @Suppress("UNCHECKED_CAST")
        fun descend(from: MutableMap<String, Any?>, keys: List<String>): MutableMap<String, Any?> {
            var t = from
            for (k in keys) {
                t = when (val x = t.getOrPut(k) { LinkedHashMap<String, Any?>() }) {
                    is MutableMap<*, *> -> x as MutableMap<String, Any?>
                    is MutableList<*> -> (x.lastOrNull() as? MutableMap<String, Any?>) ?: fail("键 $k 不是表")
                    else -> fail("键 $k 不是表")
                }
            }
            return t
        }

        fun key(): List<String> {
            val keys = ArrayList<String>()
            while (true) {
                spaces()
                keys.add(
                    when (s.getOrNull(i)) {
                        '"' -> basic()
                        '\'' -> literal()
                        else -> {
                            val start = i
                            while (i < s.length && (s[i].isLetterOrDigit() || s[i] == '_' || s[i] == '-')) i++
                            if (start == i) fail("应为键名")
                            s.substring(start, i)
                        }
                    },
                )
                spaces()
                if (s.getOrNull(i) == '.') i++ else return keys
            }
        }

        fun value(): Any? {
            return when (val c = s.getOrNull(i) ?: fail("缺少值")) {
                '"' -> if (s.startsWith("\"\"\"", i)) multiBasic() else basic()
                '\'' -> if (s.startsWith("'''", i)) multiLiteral() else literal()
                '[' -> array()
                '{' -> inline()
                't' -> if (s.startsWith("true", i)) { i += 4; true } else fail("无法识别的值")
                'f' -> if (s.startsWith("false", i)) { i += 5; false } else fail("无法识别的值")
                else -> if (c.isDigit() || c == '+' || c == '-' || c == 'i' || c == 'n') bare() else fail("无法识别的值")
            }
        }

        fun bare(): Any {
            val start = i
            while (i < s.length && s[i] !in ",]}#\n\r" && !(s[i] == ' ' && !dateSpace(start))) i++
            val t = s.substring(start, i).trim()
            val n = t.replace("_", "")
            return when {
                n == "inf" || n == "+inf" -> Double.POSITIVE_INFINITY
                n == "-inf" -> Double.NEGATIVE_INFINITY
                n.endsWith("nan") -> Double.NaN
                n.startsWith("0x") -> n.drop(2).toLong(16)
                n.startsWith("0o") -> n.drop(2).toLong(8)
                n.startsWith("0b") -> n.drop(2).toLong(2)
                Regex("[+-]?\\d+").matches(n) -> n.toLongOrNull() ?: n
                Regex("[+-]?\\d+(\\.\\d+)?([eE][+-]?\\d+)?").matches(n) -> n.toDouble()
                Regex("\\d{4}-\\d{2}-\\d{2}.*|\\d{2}:\\d{2}.*").matches(t) -> t
                else -> fail("无法识别的值 $t")
            }
        }

        /** A space within a date-time, as "1979-05-27 07:32:00". */
        fun dateSpace(start: Int): Boolean =
            Regex("\\d{4}-\\d{2}-\\d{2}").matches(s.substring(start, i)) && s.getOrNull(i + 1)?.isDigit() == true

        fun basic(): String {
            i++
            val b = StringBuilder()
            while (true) {
                val c = s.getOrNull(i++) ?: fail("字符串未结束")
                when (c) {
                    '"' -> return b.toString()
                    '\\' -> escape(b)
                    '\n' -> fail("字符串未结束")
                    else -> b.append(c)
                }
            }
        }

        fun escape(b: StringBuilder) {
            when (val e = s.getOrNull(i++) ?: fail("转义不完整")) {
                'n' -> b.append('\n'); 't' -> b.append('\t'); 'r' -> b.append('\r')
                'b' -> b.append('\b'); 'f' -> b.append('\u000c'); '"' -> b.append('"'); '\\' -> b.append('\\')
                'u', 'U' -> {
                    val n = if (e == 'u') 4 else 8
                    b.appendCodePoint(s.substring(i, i + n).toInt(16)); i += n
                }
                else -> fail("无效的转义 \\$e")
            }
        }

        fun multiBasic(): String {
            i += 3
            if (s.getOrNull(i) == '\n') i++ else if (s.startsWith("\r\n", i)) i += 2
            val b = StringBuilder()
            while (true) {
                if (s.startsWith("\"\"\"", i)) {
                    i += 3
                    while (s.getOrNull(i) == '"') { b.append('"'); i++ }
                    return b.toString()
                }
                val c = s.getOrNull(i++) ?: fail("字符串未结束")
                if (c == '\\') {
                    if (s.getOrNull(i) == '\n' || s.getOrNull(i) == '\r' || s.getOrNull(i) == ' ') {
                        while (i < s.length && s[i].isWhitespace()) i++
                    } else escape(b)
                } else b.append(c)
            }
        }

        fun literal(): String {
            i++
            val end = s.indexOf('\'', i)
            if (end < 0 || s.substring(i, end).contains('\n')) fail("字符串未结束")
            return s.substring(i, end).also { i = end + 1 }
        }

        fun multiLiteral(): String {
            i += 3
            if (s.getOrNull(i) == '\n') i++ else if (s.startsWith("\r\n", i)) i += 2
            val end = s.indexOf("'''", i)
            if (end < 0) fail("字符串未结束")
            return s.substring(i, end).also { i = end + 3 }
        }

        fun array(): List<Any?> {
            i++
            val l = ArrayList<Any?>()
            while (true) {
                blank()
                if (s.getOrNull(i) == ']') { i++; return l }
                l.add(value())
                blank()
                when (s.getOrNull(i)) {
                    ',' -> i++
                    ']' -> { i++; return l }
                    else -> fail("应为 ',' 或 ']'")
                }
            }
        }

        fun inline(): Map<String, Any?> {
            i++
            val m = LinkedHashMap<String, Any?>()
            spaces()
            if (s.getOrNull(i) == '}') { i++; return m }
            while (true) {
                val keys = key()
                spaces()
                if (s.getOrNull(i) != '=') fail("应为 '='")
                i++
                spaces()
                descend(m, keys.dropLast(1))[keys.last()] = value()
                spaces()
                when (s.getOrNull(i)) {
                    ',' -> i++
                    '}' -> { i++; return m }
                    else -> fail("应为 ',' 或 '}'")
                }
            }
        }
    }

    /** Writes [v] as a TOML document. What is not a table goes under a key of its own. */
    fun write(v: Any?): String {
        val root: Map<*, *> = when (v) {
            is Map<*, *> -> v
            is List<*> -> linkedMapOf("items" to v)
            else -> linkedMapOf("value" to v)
        }
        val b = StringBuilder()
        table(b, root, emptyList())
        return b.toString().trimStart('\n')
    }

    private fun isTable(v: Any?) = v is Map<*, *>
    private fun isTables(v: Any?) = v is List<*> && v.isNotEmpty() && v.all { it is Map<*, *> }

    private fun table(b: StringBuilder, m: Map<*, *>, path: List<String>) {
        for ((k, x) in m) {
            if (x == null || isTable(x) || isTables(x)) continue
            b.append(key(k.toString())).append(" = ").append(value(x)).append('\n')
        }
        for ((k, x) in m) {
            val p = path + k.toString()
            when {
                isTable(x) -> {
                    val t = x as Map<*, *>
                    val hasScalars = t.values.any { it != null && !isTable(it) && !isTables(it) }
                    if (hasScalars || t.isEmpty()) b.append('\n').append('[').append(p.joinToString(".") { key(it) }).append("]\n")
                    table(b, t, p)
                }
                isTables(x) -> (x as List<*>).forEach {
                    b.append('\n').append("[[").append(p.joinToString(".") { key(it) }).append("]]\n")
                    table(b, it as Map<*, *>, p)
                }
            }
        }
    }

    private fun key(k: String) = if (k.isNotEmpty() && k.all { it.isLetterOrDigit() && it.code < 128 || it == '_' || it == '-' }) k else str(k)

    private fun str(s: String): String = StringBuilder().also { Json.string(it, s) }.toString()

    private fun value(v: Any?): String = when (v) {
        null -> "\"\""
        is String -> str(v)
        is Boolean, is Long, is Int -> v.toString()
        is Double -> when {
            v.isNaN() -> "nan"
            v.isInfinite() -> if (v > 0) "inf" else "-inf"
            v == Math.floor(v) && kotlin.math.abs(v) < 1e15 -> "${v.toLong()}.0"
            else -> v.toString()
        }
        is Map<*, *> -> v.entries.filter { it.value != null }.joinToString(", ", "{ ", " }") { "${key(it.key.toString())} = ${value(it.value)}" }
            .let { if (v.isEmpty()) "{}" else it }
        is List<*> -> v.filterNotNull().joinToString(", ", "[", "]") { value(it) }
        else -> str(v.toString())
    }
}
