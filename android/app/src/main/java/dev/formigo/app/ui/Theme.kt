package dev.formigo.app.ui

import androidx.compose.animation.core.FiniteAnimationSpec
import androidx.compose.animation.core.snap
import androidx.compose.animation.core.spring
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.remember
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.graphics.luminance
import dev.formigo.app.convert.Kind
import kotlin.math.PI
import kotlin.math.max
import kotlin.math.min

/**
 * Formigo's look, from its icon: a sticker of a sky-blue gopher outlined in
 * navy, an action-blue orbit arrow, lavender and mint documents. Flat fills,
 * navy edges, and a hard shadow under what can be pressed.
 */
@Immutable
data class Palette(
    val dark: Boolean,
    /** Pure black: the dark appearance for OLED screens. */
    val oled: Boolean = false,
    val bg: Color,
    val panel: Color,
    val surface: Color,
    val raised: Color,
    val line: Color,
    val lineStrong: Color,
    val outline: Color,
    val ink: Color,
    val muted: Color,
    val faint: Color,
    val accent: Color,
    val accentPressed: Color,
    val accentInk: Color,
    val sky: Color,
    val skyTint: Color,
    val ok: Color,
    val bad: Color,
) {
    /** A kind's color, from the icon's documents. */
    fun hue(k: Kind): Color = when (k) {
        Kind.IMAGE -> if (dark) Color(0xFFA597FF) else Color(0xFF7A68F0)
        Kind.AUDIO -> if (dark) Color(0xFF3FD39F) else Color(0xFF1FA874)
        Kind.DATA -> if (dark) Color(0xFF5CB9F7) else Color(0xFF2492E0)
        Kind.VIDEO -> if (dark) Color(0xFFFF8FB1) else Color(0xFFE0567F)
        Kind.SUBTITLE -> if (dark) Color(0xFF7FD8E0) else Color(0xFF1E9AA6)
        Kind.ARCHIVE -> if (dark) Color(0xFFB8C2D9) else Color(0xFF6A7591)
        Kind.DOC -> if (dark) Color(0xFFFFBF55) else Color(0xFFE8930F)
    }

    /** A kind's color as a sticker's fill. */
    fun tint(k: Kind): Color = if (dark) hue(k).copy(alpha = 0.18f) else lerp(Color.White, hue(k), 0.14f)

    /** A kind's color as ink on its tint. */
    fun hueInk(k: Kind): Color = lerp(hue(k), outline, if (dark) 0.0f else 0.25f)
}

data class Accent(val id: String, val name: String, val light: Color, val dark: Color)

val accents = listOf(
    Accent("mint", "薄荷", Color(0xFF11996A), Color(0xFF3CCF9E)),
    Accent("orange", "暖橘", Color(0xFFE2681C), Color(0xFFFF9447)),
    Accent("sky", "地鼠天蓝", Color(0xFF1F7FE0), Color(0xFF5CB9F7)),
    Accent("neutral", "中性白", Color(0xFF1A1B1F), Color(0xFFECECEE)),
)

/** The accent a new install starts with, and what unknown or retired ids fall back to. */
const val DEFAULT_ACCENT = "neutral"

fun accentById(id: String) = accents.firstOrNull { it.id == id } ?: accents.first { it.id == DEFAULT_ACCENT }

fun palette(dark: Boolean, accentId: String, oled: Boolean = false): Palette {
    val base = if (dark) Palette(
        dark = true,
        // Neutral graphite, so that any accent sits on it.
        bg = Color(0xFF111214), panel = Color(0xFF16171A), surface = Color(0xFF1A1B1F), raised = Color(0xFF222328),
        line = Color.White.copy(alpha = 0.09f), lineStrong = Color.White.copy(alpha = 0.18f),
        outline = Color(0xFF060607), ink = Color(0xFFECECEE), muted = Color(0xFF9A9CA3), faint = Color(0xFF686A72),
        accent = Color(0xFF4B8BFF), accentPressed = Color(0xFF3873E6), accentInk = Color.White,
        sky = Color(0xFF5CB9F7), skyTint = Color(0xFF24262B), ok = Color(0xFFA3D65C), bad = Color(0xFFFF6B74),
    ) else Palette(
        dark = false,
        bg = Color(0xFFF2F6FC), panel = Color.White, surface = Color.White, raised = Color(0xFFE9F3FE),
        line = Color(20, 33, 61).copy(alpha = 0.11f), lineStrong = Color(20, 33, 61).copy(alpha = 0.2f),
        outline = Color(0xFF14213D), ink = Color(0xFF14213D), muted = Color(0xFF56627E), faint = Color(0xFF8E99B2),
        accent = Color(0xFF1F6BFF), accentPressed = Color(0xFF1555DC), accentInk = Color.White,
        sky = Color(0xFF5CB9F7), skyTint = Color(0xFFE3F2FE), ok = Color(0xFF5F9E1F), bad = Color(0xFFEF4F5A),
    )
    // OLED: the page is black, cards a near black kept apart by thin edges.
    val base2 = if (dark && oled) base.copy(
        oled = true,
        bg = Color.Black, panel = Color(0xFF0B0B0D), surface = Color(0xFF0B0B0D), raised = Color(0xFF16161A),
        line = Color.White.copy(alpha = 0.10f), lineStrong = Color.White.copy(alpha = 0.20f),
        outline = Color.Black, skyTint = Color(0xFF18181C),
    ) else base
    val a = accentById(accentId)
    val accent = if (dark) a.dark else a.light
    // White or graphite on the accent, whichever reads better.
    val graphite = Color(0xFF101113)
    val ink = if (contrast(accent, graphite) > contrast(accent, Color.White)) graphite else Color.White
    return base2.copy(accent = accent, accentPressed = lerp(accent, base2.outline, 0.22f), accentInk = ink)
}

private fun contrast(a: Color, b: Color): Float {
    val l1 = max(a.luminance(), b.luminance())
    val l2 = min(a.luminance(), b.luminance())
    return (l1 + 0.05f) / (l2 + 0.05f)
}

/**
 * The motion vocabulary, the desktop's springs (a duration and a bounce)
 * as Compose springs: stiffness (2π/duration)², damping 1 − bounce.
 */
@Immutable
class Motion(val enabled: Boolean, val ambient: Boolean) {
    fun <T> spring(durationMs: Int, bounce: Float, visibility: T? = null): FiniteAnimationSpec<T> {
        if (!enabled) return snap()
        val w = 2 * PI / (durationMs / 1000.0)
        return spring(dampingRatio = 1f - bounce, stiffness = (w * w).toFloat(), visibilityThreshold = visibility)
    }

    /** For layout: panels arriving, rows moving into place. */
    fun <T> settle(): FiniteAnimationSpec<T> = spring(520, 0.15f)

    /** For direct responses to a finger. */
    fun <T> snap(): FiniteAnimationSpec<T> = spring(280, 0.25f)

    /** For small rewards, a check landing. */
    fun <T> pop(): FiniteAnimationSpec<T> = spring(480, 0.5f)

    /** The gopher's jump. */
    fun <T> hop(): FiniteAnimationSpec<T> = spring(560, 0.55f)

    companion object {
        val Default = Motion(enabled = true, ambient = true)
    }
}

val LocalPalette = staticCompositionLocalOf { palette(false, DEFAULT_ACCENT) }
val LocalMotion = staticCompositionLocalOf { Motion.Default }

@Composable
fun FormigoTheme(p: Palette, motion: Motion, content: @Composable () -> Unit) {
    val scheme = remember(p) {
        val s = if (p.dark) darkColorScheme() else lightColorScheme()
        s.copy(
            primary = p.accent, onPrimary = p.accentInk, primaryContainer = p.skyTint, onPrimaryContainer = p.ink,
            secondary = p.sky, background = p.bg, onBackground = p.ink, surface = p.panel, onSurface = p.ink,
            surfaceVariant = p.raised, onSurfaceVariant = p.muted, surfaceContainer = p.panel,
            surfaceContainerLow = p.panel, surfaceContainerHigh = p.surface, surfaceContainerHighest = p.raised,
            surfaceContainerLowest = p.panel, outline = p.lineStrong, outlineVariant = p.line, error = p.bad,
        )
    }
    CompositionLocalProvider(LocalPalette provides p, LocalMotion provides motion) {
        MaterialTheme(colorScheme = scheme, content = content)
    }
}
