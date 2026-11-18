package dev.formigo.app.convert

/** How files are written. Each format reads only what concerns it. */
data class Options(
    /** Lossy quality, 10–100. */
    val quality: Int = 85,
    /** Index into [SCALES]. */
    val scale: Int = 0,
    /** WAV bits per sample: 16 or 24. */
    val depth: Int = 16,
    /** Mix sound down to one channel. */
    val mono: Boolean = false,
    /** Index into [INDENTS]. */
    val indent: Int = 0,
    /** Index into [VIDEO_SIZES]. */
    val videoSize: Int = 0,
    /** Index into [PDF_SCALES]: how sharp pages render. */
    val pdfScale: Int = 1,
) {
    val scaleFactor: Float get() = SCALE_FACTORS[scale.coerceIn(SCALE_FACTORS.indices)]
    val indentText: String get() = INDENT_TEXT[indent.coerceIn(INDENT_TEXT.indices)]
    val videoHeight: Int get() = VIDEO_HEIGHTS[videoSize.coerceIn(VIDEO_HEIGHTS.indices)]
    val pdfFactor: Float get() = PDF_FACTORS[pdfScale.coerceIn(PDF_FACTORS.indices)]

    companion object {
        val SCALES = listOf("原始尺寸", "75%", "50%", "25%")
        private val SCALE_FACTORS = listOf(1f, 0.75f, 0.5f, 0.25f)
        val INDENTS = listOf("2 格", "4 格", "紧凑")
        private val INDENT_TEXT = listOf("  ", "    ", "")
        val VIDEO_SIZES = listOf("原始", "1080p", "720p", "480p")
        private val VIDEO_HEIGHTS = listOf(0, 1080, 720, 480)
        val PDF_SCALES = listOf("标准", "高清", "超清")
        private val PDF_FACTORS = listOf(2f, 3f, 4f)
        val DEPTHS = listOf(16, 24)
    }
}

/** An option the options sheet may show. */
enum class Opt { QUALITY, SCALE, DEPTH, MONO, INDENT, VIDEO_SIZE, PDF_SCALE }

/** The options that shape files of [kind] written as [target]. */
fun optionsFor(kind: Kind, target: String): List<Opt> = when {
    kind == Kind.DOC && (target == "png" || target == "jpg") ->
        if (target == "jpg") listOf(Opt.PDF_SCALE, Opt.QUALITY) else listOf(Opt.PDF_SCALE)
    kind == Kind.IMAGE && (target == "jpg" || target == "webp") -> listOf(Opt.QUALITY, Opt.SCALE)
    kind == Kind.IMAGE && target in setOf("png", "gif", "bmp", "ico", "pdf") -> listOf(Opt.SCALE)
    target == "wav" -> listOf(Opt.DEPTH, Opt.MONO)
    target == "m4a" || target == "aac" -> listOf(Opt.MONO)
    target == "mp4" || target == "hevc" -> listOf(Opt.VIDEO_SIZE)
    target == "json" || target == "xml" -> listOf(Opt.INDENT)
    else -> emptyList()
}
