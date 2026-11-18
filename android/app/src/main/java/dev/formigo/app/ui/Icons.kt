package dev.formigo.app.ui

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.unit.dp
import dev.formigo.app.convert.Kind

/** The desktop's icons: a 24-unit grid, round strokes a little heavy, to sit beside the icon's outlines. */
object Icons {
    private fun circle(cx: Float, cy: Float, r: Float) = "M${cx - r},${cy}a$r,$r 0 1,0 ${2 * r},0a$r,$r 0 1,0 ${-2 * r},0"

    private fun rect(x: Float, y: Float, w: Float, h: Float, r: Float) =
        "M${x + r},${y}h${w - 2 * r}a$r,$r 0 0 1 $r,$r" + "v${h - 2 * r}a$r,$r 0 0 1 ${-r},$r" +
            "h${-(w - 2 * r)}a$r,$r 0 0 1 ${-r},${-r}" + "v${-(h - 2 * r)}a$r,$r 0 0 1 $r,${-r}z"

    private fun icon(name: String, vararg paths: String, width: Float = 2.1f): ImageVector {
        val b = ImageVector.Builder(name, 24.dp, 24.dp, 24f, 24f)
        for (d in paths) {
            b.addPath(
                pathData = PathParser().parsePathString(d).toNodes(),
                fill = null,
                stroke = SolidColor(Color.Black),
                strokeLineWidth = width,
                strokeLineCap = StrokeCap.Round,
                strokeLineJoin = StrokeJoin.Round,
            )
        }
        return b.build()
    }

    val plus = icon("plus", "M12 5v14M5 12h14")
    val close = icon("close", "M6 6l12 12M18 6L6 18")
    val check = icon("check", "M5 12.5l4.5 4.5L19 7.5")
    val alert = icon("alert", "M12 8v5", "M12 16.5v.01", circle(12f, 12f, 9f))
    val arrow = icon("arrow", "M5 12h13", "M13 6l6 6-6 6")
    val folder = icon("folder", "M3.5 7.5a2 2 0 0 1 2-2h4l2 2.5h7a2 2 0 0 1 2 2v7.5a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2z")
    val sweep = icon("sweep", "M4 7h16", "M9 7V4.5h6V7", "M6.5 7l1 12.5h9l1-12.5")
    val convert = icon(
        "convert", "M4 9a8 8 0 0 1 14.3-3.3L20 7.5", "M20 3.5v4h-4", "M20 15a8 8 0 0 1-14.3 3.3L4 16.5", "M4 20.5v-4h4",
    )
    val save = icon("save", "M12 4v11", "M7.5 10.5L12 15l4.5-4.5", "M5 19.5h14")
    val photos = icon(
        "photos", rect(3.5f, 5.5f, 13f, 13f, 2.5f), "M7.5 3.5h10a3 3 0 0 1 3 3v10", circle(8f, 10f, 1.4f), "M16.5 15.5l-3.5-3.5-6 6",
    )
    val gear = icon(
        "gear", circle(12f, 12f, 3f),
        "M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z",
        width = 1.9f,
    )
    val share = icon(
        "share", "M12 3.5v11", "M8 7.5l4-4 4 4",
        "M6.5 11H6a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-6a2 2 0 0 0-2-2h-.5",
    )
    val sliders = icon(
        "sliders", "M5 6h9M18 6h1M5 12h3M12 12h7M5 18h11M20 18h-1",
        circle(16f, 6f, 2f), circle(10f, 12f, 2f), circle(18f, 18f, 2f),
    )
    val upDown = icon("updown", "M8 9.5l4-4 4 4", "M8 14.5l4 4 4-4")
    val retry = icon("retry", "M4.5 12a7.5 7.5 0 1 0 2.2-5.3L4.5 9", "M4.5 4.5V9H9")
    val doc = icon("doc", "M14 3.5H7a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-10z", "M14 3.5v5h5", "M8.5 13h7M8.5 16.5h5")

    private val image = icon("image", rect(3.5f, 4.5f, 17f, 15f, 2.5f), circle(9f, 10f, 1.6f), "M20.5 15.5l-4.5-4.5-8.5 8.5")
    private val audio = icon("audio", "M9 18.5V6l10-2v12.5", circle(6.5f, 18.5f, 2.5f), circle(16.5f, 16.5f, 2.5f))
    private val video = icon("video", rect(3f, 6f, 13f, 12f, 2.5f), "M16 10.5l5-3v9l-5-3")
    private val data = icon(
        "data", "M8.5 4.5c-2 0-3 1-3 3v2c0 1.5-.8 2.5-2 2.5 1.2 0 2 1 2 2.5v2c0 2 1 3 3 3",
        "M15.5 4.5c2 0 3 1 3 3v2c0 1.5.8 2.5 2 2.5-1.2 0-2 1-2 2.5v2c0 2-1 3-3 3",
    )
    private val subtitle = icon("subtitle", rect(3f, 5f, 18f, 14f, 2.5f), "M7 12.5h4M13 12.5h4M7 15.5h7")
    private val archive = icon("archive", rect(4f, 3.5f, 16f, 17f, 2.5f), "M12 3.5v2M12 7.5v2M12 11.5v2", rect(10.5f, 14.5f, 3f, 3f, 1f))

    fun kind(k: Kind): ImageVector = when (k) {
        Kind.IMAGE -> image
        Kind.AUDIO -> audio
        Kind.VIDEO -> video
        Kind.DATA -> data
        Kind.DOC -> doc
        Kind.SUBTITLE -> subtitle
        Kind.ARCHIVE -> archive
    }
}
