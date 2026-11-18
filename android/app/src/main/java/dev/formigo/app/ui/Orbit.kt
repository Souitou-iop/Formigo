package dev.formigo.app.ui

import android.provider.Settings
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.graphics.drawscope.withTransform
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.TextLayoutResult
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.drawText
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.formigo.app.R
import dev.formigo.app.convert.Kind
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlin.math.PI
import kotlin.math.cos
import kotlin.math.max
import kotlin.math.min
import kotlin.math.sin

/** A format riding the orbit around the gopher, and where it starts, in turns. */
private class Satellite(val label: String, val kind: Kind, val phase: Float)

private val satellites = listOf(
    Satellite("JPG", Kind.IMAGE, 0f), Satellite("MP3", Kind.AUDIO, 0.125f),
    Satellite("JSON", Kind.DATA, 0.25f), Satellite("MD", Kind.DOC, 0.375f),
    Satellite("PNG", Kind.IMAGE, 0.5f), Satellite("FLAC", Kind.AUDIO, 0.625f),
    Satellite("YAML", Kind.DATA, 0.75f), Satellite("HTML", Kind.DOC, 0.875f),
)

/** Seconds a turn of the orbit takes. */
private const val ORBIT_PERIOD = 38.0

/** How far, in radians, a tap on the gopher kicks the documents round. */
private const val BOOP_KICK = 1.4f

/** The tilted ellipse the documents ride, as the icon's arrow circles the gopher. */
private class Ellipse(val cx: Float, val cy: Float, val rx: Float, val ry: Float) {
    private val tilt = -9.0 * PI / 180
    private val s = sin(tilt).toFloat()
    private val c = cos(tilt).toFloat()

    /** The point at angle [a], and whether it is on the near side, before the gopher. */
    fun at(a: Float): Triple<Float, Float, Boolean> {
        val ex = rx * cos(a)
        val ey = ry * sin(a)
        return Triple(cx + ex * c - ey * s, cy + ex * s + ey * c, sin(a) > 0)
    }
}

/**
 * The empty page's stage: the gopher, the documents riding a tilted orbit
 * round it, the far half behind and the near half before. A tap makes the
 * gopher hop and kicks the documents on round for good. It all moves in
 * frames that only draw.
 */
@Composable
fun OrbitStage(size: Dp, mascot: Boolean) {
    val p = LocalPalette.current
    val m = LocalMotion.current
    val context = LocalContext.current
    val density = LocalDensity.current
    val scope = rememberCoroutineScope()
    val measurer = rememberTextMeasurer()

    // The system asking for less motion, or the app's quiet modes, stop the drift.
    val reduce = remember {
        Settings.Global.getFloat(context.contentResolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f) == 0f
    }
    val still = reduce || !m.ambient

    // A clock read only while drawing: a frame redraws the orbit and nothing else.
    var now by remember { mutableLongStateOf(System.nanoTime()) }
    var boops by remember { mutableIntStateOf(0) }
    var boopAt by remember { mutableLongStateOf(0L) }
    LaunchedEffect(still, boops) {
        // Drifting, always; still, only while a kick plays.
        val until = System.nanoTime() + 1_000_000_000L
        while (!still || System.nanoTime() < until) withFrameNanos { now = System.nanoTime() }
    }

    // The arrival: the gopher pops in, then the orbit spreads out round it.
    val gopherIn = remember { Animatable(0f) }
    val open = remember { Animatable(0f) }
    LaunchedEffect(Unit) {
        if (!m.enabled) { gopherIn.snapTo(1f); open.snapTo(1f); return@LaunchedEffect }
        launch { gopherIn.animateTo(1f, tween(700, easing = CubicBezierEasing(0.34f, 1.56f, 0.64f, 1f))) }
        delay(120)
        open.animateTo(1f, tween(900, easing = CubicBezierEasing(0.16f, 1f, 0.3f, 1f)))
    }

    // The hop: up for 170 ms, then down on a bouncing spring.
    val hopY = remember { Animatable(0f) }
    val hopTilt = remember { Animatable(0f) }
    val hopScale = remember { Animatable(1f) }
    val bob = if (!still) rememberInfiniteTransition(label = "bob").animateFloat(
        0f, 1f, infiniteRepeatable(tween(1300, easing = FastOutSlowInEasing), RepeatMode.Reverse), label = "bob",
    ) else null

    val labels = remember(p) {
        satellites.associate { s ->
            s.label to measurer.measure(
                s.label,
                TextStyle(fontSize = 10.5.sp, fontWeight = FontWeight.ExtraBold, color = if (p.dark) p.hue(s.kind) else lerp(p.hue(s.kind), p.outline, 0.35f)),
            )
        }
    }

    fun boop() {
        Haptics.click(context)
        boops++
        boopAt = System.nanoTime()
        if (!m.enabled) return
        scope.launch {
            val up = tween<Float>(170, easing = CubicBezierEasing(0.2f, 0f, 0f, 1f))
            val landing = m.spring<Float>(620, 0.55f)
            coroutineScope {
                launch { hopTilt.animateTo(-9f, up) }
                launch { hopScale.animateTo(1.08f, up) }
                hopY.animateTo(-30f, up)
            }
            launch { hopTilt.animateTo(0f, landing) }
            launch { hopScale.animateTo(1f, landing) }
            hopY.animateTo(0f, landing)
        }
    }

    val draw: DrawScope.(Boolean) -> Unit = { front ->
        val secs = if (still) 0.0 else (now % 1_000_000_000_000_000L) / 1e9
        // Each tap kicks the documents on round, for good: 1.4 rad over 0.9 s, easing out.
        val since = if (boopAt == 0L) Float.MAX_VALUE else ((now - boopAt) / 1e9f)
        val k = min(1f, since / 0.9f)
        val kick = 1f - (1f - k) * (1f - k) * (1f - k)
        val flash = max(0f, 1f - since / 0.7f)
        val turn = (secs / ORBIT_PERIOD * 2 * PI).toFloat() + if (boops > 0) BOOP_KICK * (boops - 1 + kick) else 0f
        drawOrbit(front, open.value, secs.toFloat(), turn, flash, p, labels, density.density)
    }

    Box(Modifier.size(size, size * 0.86f), contentAlignment = Alignment.Center) {
        Canvas(Modifier.fillMaxSize()) { draw(false) }
        val side = size * 0.74f
        val source = remember { MutableInteractionSource() }
        val gopher = Modifier
            .padding(bottom = size * 0.04f)
            .size(if (mascot) side else side * 0.46f)
            .graphicsLayer {
                transformOrigin = TransformOrigin(0.5f, 1f)
                val inn = gopherIn.value
                alpha = min(1f, inn * 1.6f)
                val s = (0.4f + 0.6f * inn) * hopScale.value
                scaleX = s; scaleY = s
                translationY = (hopY.value - 5f * (bob?.value ?: 0f)).dp.toPx()
                rotationZ = hopTilt.value
            }
            .clickable(source, null, role = Role.Button, onClickLabel = "小地鼠") { boop() }
        if (mascot) {
            Image(painterResource(R.drawable.mascot), "小地鼠", gopher)
        } else {
            // Without the gopher, the convert mark as a sticker of its own.
            Box(
                gopher.background(p.accent, RoundedCornerShape(side * 0.14f)).border(3.dp, p.outline, RoundedCornerShape(side * 0.14f)),
                contentAlignment = Alignment.Center,
            ) { Icon(Icons.convert, null, Modifier.size(side * 0.22f), tint = p.accentInk) }
        }
        Canvas(Modifier.fillMaxSize()) { draw(true) }
    }
}

/** Half the orbit: the far side, under the gopher, or the near side, over it. */
private fun DrawScope.drawOrbit(
    front: Boolean, open: Float, secs: Float, turn: Float, flash: Float,
    p: Palette, labels: Map<String, TextLayoutResult>, dp: Float,
) {
    val w = size.width
    val h = size.height
    val spread = 0.6f + 0.4f * open
    val o = Ellipse(w / 2, h * 0.6f, w * 0.48f * spread, w * 0.14f * spread)

    // The track, faint all round, dashed and drifting.
    val path = Path()
    var started = false
    val steps = 96
    for (i in 0..steps) {
        val (x, y, near) = o.at(i.toFloat() / steps * 2 * PI.toFloat())
        if (near != front) { started = false; continue }
        if (!started) { path.moveTo(x, y); started = true } else path.lineTo(x, y)
    }
    drawPath(
        path, p.lineStrong.copy(alpha = p.lineStrong.alpha * open),
        style = Stroke(2 * dp, cap = StrokeCap.Round, pathEffect = PathEffect.dashPathEffect(floatArrayOf(2 * dp, 7 * dp), secs * 14 * dp)),
    )

    // The documents, smaller and paler on the far side.
    for (s in satellites) {
        val a = s.phase * 2 * PI.toFloat() + turn
        val (x, y, near) = o.at(a)
        if (near != front) continue
        val depth = (sin(a) + 1) / 2
        val scale = (0.62f + 0.44f * depth) * (0.5f + 0.5f * open)
        drawDoc(x, y, scale * dp, s, p, (0.7f + 0.3f * depth) * open, labels.getValue(s.label))
    }

    // The icon's three strokes of surprise, at a boop.
    if (front && flash > 0.02f) {
        val cx = w * 0.73f
        val cy = h * 0.14f
        for (deg in listOf(-62f, -30f, 2f)) {
            val r = deg * PI.toFloat() / 180
            val l0 = 10 * dp
            val l1 = (10 + 14 * flash) * dp
            drawLine(p.accent.copy(alpha = flash), Offset(cx + cos(r) * l0, cy + sin(r) * l0), Offset(cx + cos(r) * l1, cy + sin(r) * l1), 5 * dp, StrokeCap.Round)
        }
    }
}

/** A document sticker: a page with a folded corner in its kind's tint, outlined, its format on it. */
private fun DrawScope.drawDoc(x: Float, y: Float, scale: Float, s: Satellite, p: Palette, alpha: Float, label: TextLayoutResult) {
    val w = 46 * scale
    val h = 54 * scale
    val fold = 13 * scale
    val l = x - w / 2
    val t = y - h / 2
    val k = p.hue(s.kind)
    val rr = 7 * scale
    val page = Path().apply {
        moveTo(l + rr, t); lineTo(l + w - fold, t); lineTo(l + w, t + fold); lineTo(l + w, t + h - rr)
        quadraticTo(l + w, t + h, l + w - rr, t + h); lineTo(l + rr, t + h); quadraticTo(l, t + h, l, t + h - rr)
        lineTo(l, t + rr); quadraticTo(l, t, l + rr, t); close()
    }
    // The hard shadow, then the page, its outline and its fold.
    translate(0f, 3 * scale) { drawPath(page, p.outline.copy(alpha = alpha)) }
    val face = if (p.dark) lerp(p.surface, k, 0.25f) else lerp(Color.White, k, 0.16f)
    drawPath(page, face.copy(alpha = alpha))
    drawPath(page, p.outline.copy(alpha = alpha), style = Stroke(2 * scale + 0.4f, join = StrokeJoin.Round))
    val f = Path().apply {
        moveTo(l + w - fold, t); lineTo(l + w - fold, t + fold - 2 * scale)
        quadraticTo(l + w - fold, t + fold, l + w - fold + 2 * scale, t + fold); lineTo(l + w, t + fold); close()
    }
    drawPath(f, k.copy(alpha = 0.55f * alpha))
    drawPath(f, p.outline.copy(alpha = alpha), style = Stroke(1.6f * scale + 0.3f, join = StrokeJoin.Round))
    // The label, measured once at 1 dp a unit, scaled with the page.
    val k2 = scale / density
    withTransform({ translate(x, y + 3 * scale); scale(k2, k2, Offset.Zero) }) {
        drawText(label, topLeft = Offset(-label.size.width / 2f, -label.size.height / 2f), alpha = alpha)
    }
}
