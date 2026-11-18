package dev.formigo.app.convert

/** A line of subtitles: when it shows, in milliseconds, and its text (lines joined by \n). */
data class Cue(val start: Long, val end: Long, val text: String)

/** SRT, WebVTT, ASS/SSA and LRC, each to the others. */
object Subtitles {
    fun convert(text: String, from: String, to: String): String = write(to, parse(from, text))

    fun parse(from: String, text: String): List<Cue> {
        val s = text.removePrefix("\uFEFF").replace("\r\n", "\n").replace('\r', '\n')
        val cues = when (from) {
            "srt", "vtt" -> blocks(s)
            "ass" -> ass(s)
            "lrc" -> lrc(s)
            else -> throw ConvertException("不支持的字幕格式")
        }
        if (cues.isEmpty()) throw ConvertException("没有找到字幕")
        return cues.sortedBy { it.start }
    }

    private val timing = Regex("((?:\\d+:)?\\d{1,2}:\\d{2}[.,]\\d{1,3})\\s*-->\\s*((?:\\d+:)?\\d{1,2}:\\d{2}[.,]\\d{1,3})")

    /** SRT and VTT: blocks of an optional id, a timing line and the text. */
    private fun blocks(s: String): List<Cue> {
        val out = ArrayList<Cue>()
        for (block in s.split(Regex("\n\\s*\n"))) {
            val lines = block.lines()
            val at = lines.indexOfFirst { timing.containsMatchIn(it) }
            if (at < 0) continue
            val m = timing.find(lines[at])!!
            val body = lines.drop(at + 1).joinToString("\n").trim()
            out.add(Cue(clock(m.groupValues[1]), clock(m.groupValues[2]), stripTags(body)))
        }
        return out
    }

    private fun stripTags(s: String) = s.replace(Regex("</?(b|i|u|c|v|lang|ruby|rt|font)[^>]*>"), "")

    /** "01:02:03,456", "02:03.4" or "1:02:03.45" in milliseconds. */
    fun clock(t: String): Long {
        val parts = t.replace(',', '.').split(':')
        var ms = 0.0
        for (p in parts) ms = ms * 60 + p.toDouble()
        return Math.round(ms * 1000)
    }

    private fun ass(s: String): List<Cue> {
        var fields = listOf("Layer", "Start", "End", "Style", "Name", "MarginL", "MarginR", "MarginV", "Effect", "Text")
        var events = false
        val out = ArrayList<Cue>()
        for (line in s.lines()) {
            val l = line.trim()
            if (l.startsWith("[")) { events = l.equals("[Events]", true); continue }
            if (!events) continue
            if (l.startsWith("Format:", true)) {
                fields = l.substringAfter(':').split(',').map { it.trim() }
                continue
            }
            if (!l.startsWith("Dialogue:", true)) continue
            val values = l.substringAfter(':').split(',', limit = fields.size).map { it.trim() }
            fun f(name: String) = values.getOrNull(fields.indexOfFirst { it.equals(name, true) }) ?: ""
            val text = f("Text").replace(Regex("\\{[^}]*\\}"), "").replace("\\N", "\n").replace("\\n", "\n").replace("\\h", " ")
            out.add(Cue(clock(f("Start")), clock(f("End")), text.trim()))
        }
        return out
    }

    private fun lrc(s: String): List<Cue> {
        val stamp = Regex("\\[(\\d+):(\\d{1,2}(?:[.:]\\d{1,3})?)]")
        var offset = 0L
        val lines = ArrayList<Pair<Long, String>>()
        for (line in s.lines()) {
            Regex("^\\[offset:\\s*([+-]?\\d+)\\s*]", RegexOption.IGNORE_CASE).find(line.trim())?.let { offset = it.groupValues[1].toLong() }
            val stamps = stamp.findAll(line).toList()
            if (stamps.isEmpty()) continue
            val text = line.substring(stamps.last().range.last + 1).replace(Regex("<\\d+:\\d+(?:\\.\\d+)?>"), "").trim()
            for (m in stamps) {
                val sec = m.groupValues[2].replace(':', '.').toDouble()
                lines.add((m.groupValues[1].toLong() * 60000 + Math.round(sec * 1000) - offset).coerceAtLeast(0) to text)
            }
        }
        lines.sortBy { it.first }
        // A line shows until the next starts; the last for a few seconds.
        return lines.mapIndexedNotNull { i, (t, text) ->
            if (text.isEmpty()) null
            else Cue(t, lines.getOrNull(i + 1)?.first?.takeIf { it > t } ?: (t + 4000), text)
        }
    }

    fun write(to: String, cues: List<Cue>): String = when (to) {
        "srt" -> cues.mapIndexed { i, c -> "${i + 1}\n${stamp(c.start, ',')} --> ${stamp(c.end, ',')}\n${c.text}\n" }.joinToString("\n")
        "vtt" -> "WEBVTT\n\n" + cues.joinToString("\n") { c -> "${stamp(c.start, '.')} --> ${stamp(c.end, '.')}\n${c.text.replace("-->", "->")}\n" }
        "ass" -> assHeader + cues.joinToString("") { c ->
            "Dialogue: 0,${assStamp(c.start)},${assStamp(c.end)},Default,,0,0,0,,${c.text.replace("\n", "\\N")}\n"
        }
        "lrc" -> cues.joinToString("") { c -> "[${lrcStamp(c.start)}]${c.text.replace('\n', ' ')}\n" }
        else -> throw ConvertException("不支持的字幕格式")
    }

    private fun stamp(ms: Long, sep: Char) =
        "%02d:%02d:%02d%c%03d".format(ms / 3600000, ms / 60000 % 60, ms / 1000 % 60, sep, ms % 1000)

    private fun assStamp(ms: Long) = "%d:%02d:%02d.%02d".format(ms / 3600000, ms / 60000 % 60, ms / 1000 % 60, ms % 1000 / 10)

    private fun lrcStamp(ms: Long) = "%02d:%02d.%02d".format(ms / 60000, ms / 1000 % 60, ms % 1000 / 10)

    private val assHeader = """[Script Info]
ScriptType: v4.00+
PlayResX: 1920
PlayResY: 1080
WrapStyle: 0

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Noto Sans CJK SC,64,&H00FFFFFF,&H000000FF,&H00141E3D,&H64000000,0,0,0,0,100,100,0,0,1,3,1,2,60,60,50,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
"""
}
