package dev.formigo.app.convert

/** The family a file belongs to; the queue groups files by it. */
enum class Kind(val id: String, val label: String) {
    IMAGE("image", "图片"),
    VIDEO("video", "视频"),
    AUDIO("audio", "音频"),
    DATA("data", "数据"),
    DOC("doc", "文档"),
    SUBTITLE("subtitle", "字幕"),
    ARCHIVE("archive", "压缩包");

    companion object {
        fun byId(id: String?): Kind? = entries.firstOrNull { it.id == id }
    }
}

/**
 * A format a file can be written in. [id] is unique; [ext] is the extension
 * written ("" when the result is a set of files, as unpacking an archive).
 */
data class Format(
    val id: String,
    val label: String,
    val note: String,
    val ext: String,
    val kind: Kind,
)

/** A format Formigo reads, and the formats it becomes. */
data class Source(
    val id: String,
    val kind: Kind,
    /** The target it already is, when there is one: converting to it copies nothing. */
    val same: String?,
    val targets: List<String>,
    /** The lowest Android API that decodes it. */
    val minApi: Int = 0,
)

/**
 * The registry: every format read and written, and which sources become which
 * targets. The interface is built from it alone.
 */
object Formats {
    val all: List<Format> = listOf(
        Format("jpg", "JPG", "照片", "jpg", Kind.IMAGE),
        Format("png", "PNG", "无损", "png", Kind.IMAGE),
        Format("webp", "WEBP", "网页", "webp", Kind.IMAGE),
        Format("gif", "GIF", "兼容", "gif", Kind.IMAGE),
        Format("bmp", "BMP", "位图", "bmp", Kind.IMAGE),
        Format("ico", "ICO", "图标", "ico", Kind.IMAGE),
        Format("pdf", "PDF", "文档", "pdf", Kind.DOC),

        Format("mp4", "MP4", "H.264", "mp4", Kind.VIDEO),
        Format("hevc", "HEVC", "更小", "mp4", Kind.VIDEO),

        Format("m4a", "M4A", "AAC", "m4a", Kind.AUDIO),
        Format("aac", "AAC", "音轨", "aac", Kind.AUDIO),
        Format("wav", "WAV", "无压缩", "wav", Kind.AUDIO),

        Format("json", "JSON", "通用", "json", Kind.DATA),
        Format("yaml", "YAML", "配置", "yaml", Kind.DATA),
        Format("toml", "TOML", "配置", "toml", Kind.DATA),
        Format("csv", "CSV", "表格", "csv", Kind.DATA),
        Format("tsv", "TSV", "表格", "tsv", Kind.DATA),
        Format("xml", "XML", "标记", "xml", Kind.DATA),

        Format("html", "HTML", "网页", "html", Kind.DOC),
        Format("md", "MD", "Markdown", "md", Kind.DOC),
        Format("txt", "TXT", "纯文本", "txt", Kind.DOC),

        Format("srt", "SRT", "通用", "srt", Kind.SUBTITLE),
        Format("vtt", "VTT", "网页", "vtt", Kind.SUBTITLE),
        Format("ass", "ASS", "特效", "ass", Kind.SUBTITLE),
        Format("lrc", "LRC", "歌词", "lrc", Kind.SUBTITLE),

        Format("zip", "ZIP", "打包", "zip", Kind.ARCHIVE),
        Format("unzip", "解压", "取出文件", "", Kind.ARCHIVE),
    )

    private val byId = all.associateBy { it.id }

    fun format(id: String): Format? = byId[id]

    private val imageOut = listOf("jpg", "png", "webp", "gif", "bmp", "ico", "pdf")
    private val videoOut = listOf("mp4", "hevc", "m4a", "aac", "wav")
    private val audioOut = listOf("m4a", "aac", "wav")
    private val dataOut = listOf("json", "yaml", "toml", "csv", "tsv", "xml")
    private val textOut = listOf("html", "md", "txt", "pdf")
    private val subOut = listOf("srt", "vtt", "ass", "lrc")

    private fun src(id: String, kind: Kind, same: String?, targets: List<String>, minApi: Int = 0) =
        Source(id, kind, same, (targets + "zip").distinct(), minApi)

    /** Sources by extension, lower case. */
    val sources: Map<String, Source> = buildMap {
        fun put(exts: List<String>, s: Source) = exts.forEach { put(it, s) }
        put(listOf("jpg", "jpeg", "jfif"), src("jpg", Kind.IMAGE, "jpg", imageOut))
        put(listOf("png"), src("png", Kind.IMAGE, "png", imageOut))
        put(listOf("webp"), src("webp", Kind.IMAGE, "webp", imageOut))
        put(listOf("gif"), src("gif", Kind.IMAGE, "gif", imageOut))
        put(listOf("bmp"), src("bmp", Kind.IMAGE, "bmp", imageOut))
        put(listOf("heic", "heif"), src("heic", Kind.IMAGE, null, imageOut, 28))
        put(listOf("avif"), src("avif", Kind.IMAGE, null, imageOut, 31))
        put(listOf("ico"), src("ico", Kind.IMAGE, "ico", imageOut))

        put(listOf("mp4", "m4v"), src("mp4", Kind.VIDEO, null, videoOut))
        put(listOf("mov"), src("mov", Kind.VIDEO, null, videoOut))
        put(listOf("3gp", "3gpp"), src("3gp", Kind.VIDEO, null, videoOut))
        put(listOf("webm"), src("webm", Kind.VIDEO, null, videoOut))
        put(listOf("mkv"), src("mkv", Kind.VIDEO, null, videoOut))

        put(listOf("mp3"), src("mp3", Kind.AUDIO, null, audioOut))
        put(listOf("m4a"), src("m4a", Kind.AUDIO, "m4a", audioOut))
        put(listOf("aac"), src("aac", Kind.AUDIO, "aac", audioOut))
        put(listOf("wav", "wave"), src("wav", Kind.AUDIO, "wav", audioOut))
        put(listOf("flac"), src("flac", Kind.AUDIO, null, audioOut))
        put(listOf("ogg", "oga", "opus"), src("ogg", Kind.AUDIO, null, audioOut))
        put(listOf("amr"), src("amr", Kind.AUDIO, null, audioOut))

        put(listOf("json"), src("json", Kind.DATA, "json", dataOut))
        put(listOf("yaml", "yml"), src("yaml", Kind.DATA, "yaml", dataOut))
        put(listOf("toml"), src("toml", Kind.DATA, "toml", dataOut))
        put(listOf("csv"), src("csv", Kind.DATA, "csv", dataOut))
        put(listOf("tsv"), src("tsv", Kind.DATA, "tsv", dataOut))
        put(listOf("xml"), src("xml", Kind.DATA, "xml", dataOut))

        put(listOf("md", "markdown"), src("md", Kind.DOC, "md", textOut))
        put(listOf("html", "htm"), src("html", Kind.DOC, "html", textOut))
        put(listOf("txt", "text"), src("txt", Kind.DOC, "txt", textOut))
        put(listOf("pdf"), src("pdf", Kind.DOC, "pdf", listOf("png", "jpg")))

        put(listOf("srt"), src("srt", Kind.SUBTITLE, "srt", subOut))
        put(listOf("vtt"), src("vtt", Kind.SUBTITLE, "vtt", subOut))
        put(listOf("ass", "ssa"), src("ass", Kind.SUBTITLE, "ass", subOut))
        put(listOf("lrc"), src("lrc", Kind.SUBTITLE, "lrc", subOut))

        put(listOf("zip"), Source("zip", Kind.ARCHIVE, "zip", listOf("unzip")))
    }

    fun ext(name: String): String = name.substringAfterLast('.', "").lowercase()

    /** The source of a file by its name, if Formigo reads it on this API level. */
    fun detect(name: String, api: Int = Int.MAX_VALUE): Source? =
        sources[ext(name)]?.takeIf { api >= it.minApi }

    /** The formats a source becomes, in the order shown, without what it already is. */
    fun targetsOf(source: Source): List<Format> =
        source.targets.filter { it != source.same }.mapNotNull { byId[it] }

    /** What a kind becomes by default. */
    fun defaultTarget(kind: Kind): String = when (kind) {
        Kind.IMAGE -> "jpg"
        Kind.VIDEO -> "mp4"
        Kind.AUDIO -> "m4a"
        Kind.DATA -> "json"
        Kind.DOC -> "html"
        Kind.SUBTITLE -> "srt"
        Kind.ARCHIVE -> "unzip"
    }

    /** The targets a kind's default may be set to in the settings. */
    fun kindTargets(kind: Kind): List<Format> {
        val ids = sources.values.filter { it.kind == kind }.flatMap { it.targets }.toSet() - "zip"
        return all.filter { it.id in ids }
    }

    /** What a file may be set to: what it becomes, and what it already is. */
    fun choicesOf(source: Source): List<Format> = source.targets.mapNotNull { byId[it] }

    /**
     * A file's target: its own if it has one, else its group's when it can
     * be that, else its kind's default, else the first it can become.
     */
    fun resolve(source: Source, own: String?, group: String?): String {
        val can = source.targets
        return when {
            own != null && own in can -> own
            group != null && group in can -> group
            defaultTarget(source.kind) in can -> defaultTarget(source.kind)
            else -> targetsOf(source).firstOrNull()?.id ?: can.first()
        }
    }

    /** Targets every one of the sources may be set to, for a mixed batch's "convert all to". */
    fun common(sources: Collection<Source>): List<Format> {
        if (sources.isEmpty()) return emptyList()
        val shared = sources.map { it.targets.toSet() }.reduce { a, b -> a intersect b }
        return all.filter { it.id in shared }
    }

    /** Targets any of the sources becomes, for a group's strip. */
    fun union(sources: Collection<Source>): List<Format> {
        val ids = sources.flatMap { it.targets }.toSet()
        return all.filter { it.id in ids }
    }
}
