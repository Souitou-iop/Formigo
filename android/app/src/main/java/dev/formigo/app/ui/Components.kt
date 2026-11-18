package dev.formigo.app.ui

import android.os.Build
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import android.view.HapticFeedbackConstants
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.PressInteraction
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.platform.LocalView
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.layout.layout
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.kyant.backdrop.Backdrop
import com.kyant.backdrop.drawBackdrop
import com.kyant.backdrop.effects.blur
import com.kyant.backdrop.effects.lens
import com.kyant.backdrop.effects.colorControls
import com.kyant.backdrop.highlight.Highlight
import com.kyant.backdrop.shadow.Shadow
import com.kyant.shapes.Capsule
import dev.formigo.app.convert.Kind
import kotlin.math.PI

/** Liquid Glass needs RuntimeShader (Android 13); below it, a frosted plate. */
val glassAvailable: Boolean = Build.VERSION.SDK_INT >= 33

/**
 * A capsule of Liquid Glass over [backdrop], tinted with [tint] when given.
 * Below Android 13 it is a translucent plate with an edge, the same shape.
 */
fun Modifier.glass(backdrop: Backdrop, p: Palette, tint: Color? = null, pressed: () -> Float = { 0f }): Modifier {
    if (!glassAvailable) {
        val fill = tint ?: p.panel.copy(alpha = 0.94f)
        return this
            .graphicsLayer {
                val s = 1f + 0.04f * pressed()
                scaleX = s; scaleY = s
                shadowElevation = 10.dp.toPx(); shape = Capsule(); clip = true
            }
            .background(fill)
            .border(1.dp, p.lineStrong, Capsule())
    }
    // As the tab bar: near clear, the page bent and saturated through it.
    val surface = p.panel.copy(alpha = 0.4f)
    return drawBackdrop(
        backdrop = backdrop,
        shape = { Capsule() },
        effects = {
            colorControls(saturation = 1.5f)
            blur(4.dp.toPx())
            lens(if (tint != null) 12.dp.toPx() else 24.dp.toPx(), 24.dp.toPx())
        },
        highlight = { Highlight.Default.copy(width = 1.dp, alpha = 0.75f) },
        shadow = { Shadow(radius = 10.dp, color = Color.Black, alpha = if (p.dark) 0.2f else 0.1f) },
        layerBlock = {
            val s = 1f + 4.dp.toPx() / size.height * pressed()
            scaleX = s; scaleY = s
        },
        onDrawSurface = { drawRect(tint?.copy(alpha = 0.86f) ?: surface) },
    )
}

/** A press that springs: how far down a pressable is, 0 to 1. */
@Composable
fun rememberPress(source: MutableInteractionSource): Float {
    val pressed by source.collectIsPressedAsState()
    val m = LocalMotion.current
    val v by animateFloatAsState(if (pressed) 1f else 0f, m.snap(), label = "press")
    return v
}

/**
 * A sticker button: a flat fill, a navy outline and a hard shadow below,
 * which it sinks onto as it is pressed.
 */
@Composable
fun StickerButton(
    label: String,
    icon: ImageVector?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    primary: Boolean = true,
    enabled: Boolean = true,
    height: Dp = 52.dp,
    lift: Dp = 4.dp,
    fontSize: TextUnit = 16.sp,
    keyHaptics: Boolean = false,
) {
    val p = LocalPalette.current
    val source = remember { MutableInteractionSource() }
    val down = rememberPress(source)
    if (keyHaptics) {
        // A key's feel: a crisp tick as it goes down, a lighter one as it comes up and acts.
        val view = LocalView.current
        LaunchedEffect(source) {
            source.interactions.collect {
                when (it) {
                    is PressInteraction.Press -> view.performHapticFeedback(HapticFeedbackConstants.KEYBOARD_PRESS)
                    is PressInteraction.Release -> view.performHapticFeedback(HapticFeedbackConstants.VIRTUAL_KEY_RELEASE)
                }
            }
        }
    }
    val shape = RoundedCornerShape(16.dp)
    val bg = if (primary) p.accent else p.surface
    val ink = if (primary) p.accentInk else p.ink
    Box(
        modifier
            .height(height + lift)
            .graphicsLayer { alpha = if (enabled) 1f else 0.5f },
    ) {
        // The shadow stays; the face moves down onto it.
        Box(
            Modifier
                .matchParentSizeMinus(lift)
                .graphicsLayer { translationY = lift.toPx() }
                .background(p.outline, shape),
        )
        Row(
            Modifier
                .matchParentSizeMinus(lift)
                .graphicsLayer { translationY = lift.toPx() * down }
                .clip(shape)
                .background(bg)
                .border(2.dp, p.outline, shape)
                .clickable(source, null, enabled = enabled, role = Role.Button, onClick = onClick)
                .padding(horizontal = 18.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp, Alignment.CenterHorizontally),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (icon != null) Icon(icon, null, Modifier.size(19.dp), tint = ink)
            Text(label, color = ink, fontSize = fontSize, fontWeight = FontWeight.ExtraBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

/** Fills the parent's width and its height less [bottom]. */
private fun Modifier.matchParentSizeMinus(bottom: Dp): Modifier = layout { measurable, constraints ->
    val h = (constraints.maxHeight - bottom.roundToPx()).coerceAtLeast(0)
    val w = constraints.maxWidth
    val placeable = measurable.measure(androidx.compose.ui.unit.Constraints.fixed(w, h))
    layout(w, constraints.maxHeight) { placeable.place(0, 0) }
}

/** A quiet button that sinks a little as it is pressed. */
@Composable
fun SoftButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    shape: Shape = RoundedCornerShape(50),
    background: Color = Color.Transparent,
    border: Color? = null,
    enabled: Boolean = true,
    label: String? = null,
    content: @Composable RowScope.() -> Unit,
) {
    val source = remember { MutableInteractionSource() }
    val down = rememberPress(source)
    Row(
        modifier
            .graphicsLayer {
                val s = 1f - 0.06f * down
                scaleX = s; scaleY = s
                alpha = if (enabled) 1f else 0.45f
            }
            .clip(shape)
            .background(background)
            .then(if (border != null) Modifier.border(1.5.dp, border, shape) else Modifier)
            .clickable(source, null, enabled = enabled, role = Role.Button, onClickLabel = label, onClick = onClick),
        verticalAlignment = Alignment.CenterVertically,
        content = content,
    )
}

/** A kind's badge: its icon on a sticker of its tint. */
@Composable
fun KindBadge(k: Kind, size: Dp = 28.dp) {
    val p = LocalPalette.current
    val shape = RoundedCornerShape(size * 0.3f)
    Box(
        Modifier
            .size(size)
            .drawBehind {
                drawRoundRect(p.outline, Offset(0f, 2.dp.toPx()), this.size, CornerRadius((size * 0.3f).toPx()))
            }
            .background(p.tint(k).compositeOverSafe(p.surface), shape)
            .border(1.5.dp, p.outline, shape),
        contentAlignment = Alignment.Center,
    ) {
        Icon(Icons.kind(k), null, Modifier.size(size * 0.58f), tint = p.hueInk(k))
    }
}

/** A translucent color laid on an opaque one, so that shadows do not show through. */
fun Color.compositeOverSafe(under: Color): Color {
    if (alpha >= 1f) return this
    val a = alpha
    return Color(red * a + under.red * (1 - a), green * a + under.green * (1 - a), blue * a + under.blue * (1 - a), 1f)
}

/** A row of choices, the chosen one lifted on a sliding plate. */
@Composable
fun Segmented(options: List<String>, selected: Int, onSelect: (Int) -> Unit, modifier: Modifier = Modifier) {
    val p = LocalPalette.current
    val m = LocalMotion.current
    val shape = RoundedCornerShape(12.dp)
    Row(
        modifier
            .clip(shape)
            .background(if (p.dark) p.raised else p.bg)
            .border(1.dp, p.line, shape)
            .padding(3.dp),
        horizontalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        options.forEachIndexed { i, label ->
            val on = i == selected
            val bg by animateColorAsState(if (on) p.surface else Color.Transparent, m.snap(), label = "seg")
            val ink by animateColorAsState(if (on) p.ink else p.muted, m.snap(), label = "segInk")
            Box(
                Modifier
                    .clip(RoundedCornerShape(9.dp))
                    .background(bg)
                    .then(if (on) Modifier.border(1.dp, p.lineStrong, RoundedCornerShape(9.dp)) else Modifier)
                    .clickable(role = Role.RadioButton) { onSelect(i) }
                    .padding(horizontal = 12.dp, vertical = 7.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text(label, color = ink, fontSize = 13.sp, fontWeight = if (on) FontWeight.Bold else FontWeight.Medium, maxLines = 1)
            }
        }
    }
}

/**
 * A value as a pill which alone opens the menu of the choices: the row
 * around it takes no tap, so that scrolling a list sets nothing.
 */
@Composable
fun PickerPill(label: String, options: List<String>, selected: Int, onSelect: (Int) -> Unit, valueLabel: String = options.getOrElse(selected) { "" }) {
    val p = LocalPalette.current
    var open by remember { mutableStateOf(false) }
    Box {
        SoftButton(
            onClick = { open = true },
            shape = RoundedCornerShape(10.dp),
            background = if (open) p.skyTint else Color.Transparent,
            label = label,
            modifier = Modifier.height(38.dp),
        ) {
            Text(valueLabel, color = p.muted, fontSize = 15.sp, maxLines = 1, modifier = Modifier.padding(start = 12.dp))
            Icon(Icons.upDown, null, Modifier.padding(start = 4.dp, end = 8.dp).size(14.dp), tint = p.faint)
        }
        FormigoMenu(open, { open = false }) {
            options.forEachIndexed { i, o ->
                MenuItem(o, checked = i == selected) { open = false; if (i != selected) onSelect(i) }
            }
        }
    }
}

@Composable
fun FormigoMenu(expanded: Boolean, onDismiss: () -> Unit, content: @Composable () -> Unit) {
    val p = LocalPalette.current
    DropdownMenu(
        expanded = expanded,
        onDismissRequest = onDismiss,
        shape = RoundedCornerShape(16.dp),
        containerColor = p.panel,
        border = androidx.compose.foundation.BorderStroke(1.5.dp, p.lineStrong),
        shadowElevation = 8.dp,
    ) { content() }
}

@Composable
fun MenuItem(text: String, checked: Boolean = false, note: String? = null, onClick: () -> Unit) {
    val p = LocalPalette.current
    DropdownMenuItem(
        text = {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(text, color = p.ink, fontSize = 15.sp, fontWeight = if (checked) FontWeight.Bold else FontWeight.Normal)
                if (note != null) Text("  $note", color = p.faint, fontSize = 13.sp)
            }
        },
        trailingIcon = if (checked) ({ Icon(Icons.check, null, Modifier.size(18.dp), tint = p.accent) }) else null,
        onClick = onClick,
    )
}

/** A ring filling as a file converts, turning slowly so a long wait looks alive. */
@Composable
fun ProgressRing(progress: () -> Float, modifier: Modifier = Modifier) {
    val p = LocalPalette.current
    val m = LocalMotion.current
    val spin = if (m.ambient) {
        val t = rememberInfiniteTransition(label = "spin")
        t.animateFloat(0f, 1f, infiniteRepeatable(tween(2400, easing = LinearEasing), RepeatMode.Restart), label = "spin")
    } else null
    val shown = remember { Animatable(0f) }
    androidx.compose.runtime.LaunchedEffect(Unit) {
        androidx.compose.runtime.snapshotFlow { progress() }.collect { shown.animateTo(it, m.settle()) }
    }
    Canvas(modifier.size(28.dp)) {
        val w = 3.dp.toPx()
        val inset = w / 2 + 1.dp.toPx()
        val arcSize = Size(size.width - 2 * inset, size.height - 2 * inset)
        drawArc(p.line, 0f, 360f, false, Offset(inset, inset), arcSize, style = Stroke(w))
        val start = -90f + 360f * (spin?.value ?: 0f)
        val sweep = 360f * shown.value.coerceIn(0.06f, 1f)
        drawArc(p.accent, start, sweep, false, Offset(inset, inset), arcSize, style = Stroke(w, cap = androidx.compose.ui.graphics.StrokeCap.Round))
    }
}

@Suppress("unused")
private const val TAU = 2 * PI
