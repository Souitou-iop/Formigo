// Adapted from Kyant0/AndroidLiquidGlass's LiquidBottomTabs, by way of
// KernelSU's FloatingBottomBar — Apache License 2.0.

package dev.formigo.app.ui.glass

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.EaseOut
import androidx.compose.animation.core.spring
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.requiredWidth
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.wrapContentWidth
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.LocalContentColor
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.onClick
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.util.fastCoerceIn
import androidx.compose.ui.util.lerp
import com.kyant.backdrop.Backdrop
import com.kyant.backdrop.backdrops.layerBackdrop
import com.kyant.backdrop.backdrops.rememberCombinedBackdrop
import com.kyant.backdrop.backdrops.rememberLayerBackdrop
import com.kyant.backdrop.drawBackdrop
import com.kyant.backdrop.effects.blur
import com.kyant.backdrop.effects.colorControls
import com.kyant.backdrop.effects.lens
import com.kyant.backdrop.highlight.Highlight
import com.kyant.backdrop.shadow.InnerShadow
import com.kyant.backdrop.shadow.Shadow
import kotlinx.coroutines.launch
import kotlin.math.abs
import kotlin.math.roundToInt
import kotlin.math.sign

/** How much a tab's content swells while the glass is pressed. */
val LocalFloatingBottomBarTabScale = staticCompositionLocalOf { { 1f } }

/** A tab: its icon over its label, centered, swelling with the press. Taps reach it through the bar's drag. */
@Composable
fun RowScope.FloatingBottomBarItem(
    selected: Boolean,
    onClick: () -> Unit,
    label: String,
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
    val scale = LocalFloatingBottomBarTabScale.current
    Column(
        modifier
            .semantics(mergeDescendants = true) {
                this.selected = selected
                role = Role.Tab
                onClick(label) { onClick(); true }
            }
            .fillMaxHeight()
            .weight(1f)
            .graphicsLayer {
                val s = scale()
                scaleX = s
                scaleY = s
            },
        verticalArrangement = Arrangement.spacedBy(1.dp, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
        content = content,
    )
}

/**
 * A capsule of near-clear Liquid Glass holding the tabs, and a glass
 * indicator a finger can drag across them: pressed, it swells and bends what
 * is under it, stretching with its speed, and lets go onto the nearest tab.
 * The indicator shows the tabs redrawn in the accent.
 */
@Composable
fun FloatingBottomBar(
    selectedIndex: Int,
    onSelected: (Int) -> Unit,
    backdrop: Backdrop,
    tabsCount: Int,
    dark: Boolean,
    accent: Color,
    contentColor: Color,
    surfaceContainer: Color,
    modifier: Modifier = Modifier,
    glass: Boolean = true,
    content: @Composable RowScope.(activate: (Int) -> Unit) -> Unit,
) {
    val pill = CircleShape
    val containerColor = if (glass) surfaceContainer.copy(alpha = surfaceContainer.alpha * 0.4f) else surfaceContainer
    val tabsBackdrop = rememberLayerBackdrop()
    val density = LocalDensity.current
    val scope = rememberCoroutineScope()

    var tabWidthPx by remember { mutableFloatStateOf(0f) }
    var totalWidthPx by remember { mutableFloatStateOf(0f) }

    // The whole bar gives a little as the indicator is dragged, a rubber band.
    val offsetAnimation = remember { Animatable(0f) }
    val rubberBandPx = with(density) { 4.dp.toPx() }
    val panelOffset by remember(rubberBandPx) {
        derivedStateOf {
            if (totalWidthPx == 0f) 0f
            else {
                val fraction = (offsetAnimation.value / totalWidthPx).fastCoerceIn(-1f, 1f)
                rubberBandPx * fraction.sign * EaseOut.transform(abs(fraction))
            }
        }
    }

    var currentIndex by remember { mutableIntStateOf(selectedIndex) }
    val onSelectedUpdated by rememberUpdatedState(onSelected)

    fun indexAt(x: Float): Int {
        if (tabWidthPx == 0f) return currentIndex
        val padding = with(density) { 4.dp.toPx() }
        return ((x - padding) / tabWidthPx).toInt().coerceIn(0, tabsCount - 1)
    }

    val drag = remember(scope, tabsCount, density) {
        DampedDragAnimation(
            animationScope = scope,
            initialValue = selectedIndex.toFloat(),
            valueRange = 0f..(tabsCount - 1).toFloat(),
            visibilityThreshold = 0.001f,
            initialScale = 1f,
            pressedScale = 78f / 56f,
            canDrag = { it.x in 0f..totalWidthPx },
            onDragStarted = { position -> updateValue(indexAt(position.x).toFloat()) },
            onDragStopped = {
                val target = targetValue.roundToInt().coerceIn(0, tabsCount - 1)
                if (currentIndex != target) {
                    currentIndex = target
                    onSelectedUpdated(target)
                }
                updateValue(target.toFloat())
                scope.launch { offsetAnimation.animateTo(0f, spring(1f, 300f, 0.5f)) }
            },
            onDragCancelled = {
                updateValue(currentIndex.toFloat())
                scope.launch { offsetAnimation.animateTo(0f, spring(1f, 300f, 0.5f)) }
            },
            onDrag = { _, amount ->
                if (tabWidthPx > 0f && amount.x != 0f) {
                    updateValue((targetValue + amount.x / tabWidthPx).coerceIn(0f, (tabsCount - 1).toFloat()))
                    scope.launch { offsetAnimation.snapTo(offsetAnimation.value + amount.x) }
                }
            },
        )
    }

    LaunchedEffect(selectedIndex) {
        if (currentIndex != selectedIndex) {
            currentIndex = selectedIndex
            drag.animateToValue(selectedIndex.toFloat())
        }
    }

    fun activate(index: Int) {
        if (index !in 0 until tabsCount) return
        if (currentIndex != index) {
            currentIndex = index
            onSelectedUpdated(index)
        }
        drag.animateToValue(index.toFloat())
    }

    val highlight = remember(scope, drag) {
        InteractiveHighlight(scope) { size, _ ->
            Offset((drag.value + 0.5f) * tabWidthPx + panelOffset, size.height / 2f)
        }
    }
    val combined = rememberCombinedBackdrop(backdrop, tabsBackdrop)
    val specular = Highlight.Default.copy(width = 1.dp)

    Box(modifier.width(IntrinsicSize.Min), contentAlignment = Alignment.CenterStart) {
        Row(
            Modifier
                .onGloballyPositioned { c ->
                    totalWidthPx = c.size.width.toFloat()
                    tabWidthPx = ((totalWidthPx - with(density) { 8.dp.toPx() }) / tabsCount).coerceAtLeast(0f)
                }
                .selectableGroup()
                .graphicsLayer { translationX = panelOffset }
                .then(
                    if (glass) Modifier.drawBackdrop(
                        backdrop = backdrop,
                        shape = { pill },
                        effects = {
                            padding = maxOf(padding, 40.dp.toPx())
                            colorControls(saturation = 1.5f)
                            blur(4.dp.toPx())
                            lens(24.dp.toPx(), 24.dp.toPx())
                        },
                        highlight = { specular.copy(alpha = 0.75f) },
                        shadow = { Shadow(radius = 10.dp, color = Color.Black, alpha = if (dark) 0.2f else 0.1f) },
                        layerBlock = {
                            val s = lerp(1f, 1f + 16.dp.toPx() / size.width.coerceAtLeast(1f), drag.pressProgress)
                            scaleX = s
                            scaleY = s
                        },
                        onDrawSurface = { drawRect(containerColor) },
                    ).then(highlight.modifier).then(highlight.gestureModifier)
                    else Modifier.background(containerColor, pill),
                )
                .then(drag.modifier)
                .height(64.dp)
                .padding(4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            CompositionLocalProvider(LocalContentColor provides contentColor) { content(::activate) }
        }

        if (glass) {
            // The tabs again, in the accent, recorded for the indicator to show through.
            CompositionLocalProvider(
                LocalFloatingBottomBarTabScale provides { lerp(1f, 1.2f, drag.pressProgress) },
                LocalContentColor provides accent,
            ) {
                Row(
                    Modifier
                        .clearAndSetSemantics {}
                        .alpha(0f)
                        .layerBackdrop(tabsBackdrop)
                        .graphicsLayer { translationX = panelOffset }
                        .drawBackdrop(
                            backdrop = backdrop,
                            shape = { pill },
                            effects = {
                                colorControls(saturation = 1.5f)
                                blur(4.dp.toPx())
                                lens(24.dp.toPx(), 24.dp.toPx())
                            },
                            onDrawSurface = { drawRect(containerColor) },
                        )
                        .then(highlight.modifier)
                        .height(56.dp)
                        .padding(horizontal = 4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) { content(::activate) }
            }
        }

        if (tabWidthPx > 0f) {
            val tabWidth = with(density) { tabWidthPx.toDp() }
            if (glass) {
                Box(
                    Modifier
                        .padding(horizontal = 4.dp)
                        .graphicsLayer { translationX = drag.value * tabWidthPx + panelOffset }
                        .drawBackdrop(
                            backdrop = combined,
                            shape = { pill },
                            effects = {
                                val p = drag.pressProgress
                                lens(10.dp.toPx() * p, 14.dp.toPx() * p, depthEffect = true, chromaticAberration = true)
                            },
                            highlight = { specular.copy(alpha = drag.pressProgress) },
                            shadow = { Shadow(alpha = drag.pressProgress) },
                            innerShadow = { InnerShadow(radius = 8.dp * drag.pressProgress, color = Color.Black.copy(alpha = 0.15f), alpha = drag.pressProgress) },
                            layerBlock = {
                                scaleX = drag.scaleX
                                scaleY = drag.scaleY
                                val v = drag.velocity / 10f
                                scaleX /= 1f - (v * 0.75f).fastCoerceIn(-0.2f, 0.2f)
                                scaleY *= 1f - (v * 0.25f).fastCoerceIn(-0.2f, 0.2f)
                            },
                            onDrawSurface = {
                                val p = drag.pressProgress
                                drawRect(if (dark) Color.White.copy(alpha = 0.1f) else Color.Black.copy(alpha = 0.1f), alpha = 1f - p)
                                drawRect(Color.Black.copy(alpha = 0.03f * p))
                            },
                        )
                        .height(56.dp)
                        .width(tabWidth),
                )
            } else {
                // Without glass: a tinted pill, the accent tabs clipped inside it.
                Box(
                    Modifier
                        .padding(horizontal = 4.dp)
                        .graphicsLayer { translationX = drag.value * tabWidthPx + panelOffset }
                        .clip(pill)
                        .background(accent.copy(alpha = 0.15f), pill)
                        .height(56.dp)
                        .width(tabWidth),
                    contentAlignment = Alignment.CenterStart,
                ) {
                    CompositionLocalProvider(LocalContentColor provides accent) {
                        Row(
                            Modifier
                                .clearAndSetSemantics {}
                                .wrapContentWidth(Alignment.Start, unbounded = true)
                                .requiredWidth(with(density) { (totalWidthPx - 8.dp.toPx()).toDp() })
                                .height(56.dp)
                                .graphicsLayer { translationX = -drag.value * tabWidthPx },
                            verticalAlignment = Alignment.CenterVertically,
                        ) { content(::activate) }
                    }
                }
            }
        }
    }
}
