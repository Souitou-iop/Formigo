package dev.formigo.app.ui

import android.content.Intent
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.scaleOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.togetherWith
import androidx.compose.animation.AnimatedContent
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.defaultMinSize
import dev.formigo.app.ui.glass.FloatingBottomBar
import dev.formigo.app.ui.glass.FloatingBottomBarItem
import androidx.compose.foundation.layout.windowInsetsBottomHeight
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.zIndex
import com.kyant.backdrop.backdrops.LayerBackdrop
import com.kyant.backdrop.backdrops.layerBackdrop
import com.kyant.backdrop.backdrops.rememberLayerBackdrop
import com.kyant.shapes.Capsule
import dev.formigo.app.AppModel
import kotlin.math.abs

private val TabBarHeight = 64.dp
private val BarGap = 8.dp
private val Floating = 14.dp

/** What the pickers open: the photo library, and the system's files. */
class Pickers(val photos: () -> Unit, val files: () -> Unit)

@Composable
fun FormigoApp(model: AppModel) {
    val p = LocalPalette.current
    val m = LocalMotion.current
    val context = LocalContext.current

    val photos = rememberLauncherForActivityResult(ActivityResultContracts.PickMultipleVisualMedia(100)) { model.add(it) }
    val files = rememberLauncherForActivityResult(ActivityResultContracts.OpenMultipleDocuments()) { model.add(it) }
    val pickers = remember {
        Pickers(
            photos = { photos.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageAndVideo)) },
            files = { files.launch(arrayOf("*/*")) },
        )
    }

    // The tabs' position, 0 to 1: pages and the bar's highlight follow it in
    // frames that only draw, so a switch builds nothing.
    val tab = remember { Animatable(model.tab.toFloat()) }
    LaunchedEffect(model.tab) { tab.animateTo(model.tab.toFloat(), m.spring(420, 0.08f)) }

    val backdrop = rememberLayerBackdrop {
        drawRect(p.bg)
        drawContent()
    }
    val share: (List<java.io.File>) -> Unit = { list ->
        model.shareIntent(list)?.let { context.startActivity(it.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
    }

    Box(Modifier.fillMaxSize().background(p.bg)) {
        Box(Modifier.fillMaxSize().layerBackdrop(backdrop)) {
            for (i in 0..1) {
                val on = model.tab == i
                Box(
                    Modifier
                        .fillMaxSize()
                        .zIndex(if (on) 1f else 0f)
                        .graphicsLayer {
                            val d = i - tab.value
                            alpha = (1f - abs(d) * 1.4f).coerceIn(0f, 1f)
                            translationX = d * size.width * 0.06f
                        }
                        .then(if (on) Modifier else Modifier.blockInput().clearAndSetSemantics { }),
                ) {
                    if (i == 0) ConvertPage(model, pickers, share) else SettingsPage(model)
                }
            }
        }

        RunCapsule(model, backdrop, tab, share, Modifier.align(Alignment.BottomCenter))
        GlassTabs(model, backdrop, Modifier.align(Alignment.BottomCenter))

        FreshSheet(model)
        OptionsSheet(model)
        UpdateSheet(model)
    }
}

/** A page's large title. */
@Composable
fun PageHeader(title: String, trailing: @Composable () -> Unit = {}) {
    val p = LocalPalette.current
    Row(
        Modifier.fillMaxWidth().statusBarsPadding().padding(start = 22.dp, end = 14.dp, top = 10.dp, bottom = 6.dp).height(52.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(title, color = p.ink, fontSize = 32.sp, fontWeight = FontWeight.ExtraBold, letterSpacing = (-0.5).sp, modifier = Modifier.weight(1f))
        trailing()
    }
}

/** How far a tab's content goes on under what floats over it. */
@Composable
fun bottomRoom(withCapsule: Boolean): androidx.compose.ui.unit.Dp {
    val nav = with(LocalDensity.current) { WindowInsets.navigationBars.getBottom(this).toDp() }
    return nav + TabBarHeight + BarGap + 24.dp + if (withCapsule) 70.dp else 0.dp
}

private fun Modifier.blockInput(): Modifier = pointerInput(Unit) {
    awaitPointerEventScope {
        while (true) awaitPointerEvent(PointerEventPass.Initial).changes.forEach { it.consume() }
    }
}

@Composable
private fun GlassTabs(model: AppModel, backdrop: LayerBackdrop, modifier: Modifier) {
    val p = LocalPalette.current
    val tabs = listOf("转换" to Icons.convert, "设置" to Icons.gear)
    Column(modifier.padding(horizontal = 28.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        FloatingBottomBar(
            selectedIndex = model.tab,
            onSelected = { model.tab = it },
            backdrop = backdrop,
            tabsCount = tabs.size,
            dark = p.dark,
            accent = p.accent,
            contentColor = p.ink,
            surfaceContainer = p.panel,
            glass = glassAvailable,
        ) { activate ->
            tabs.forEachIndexed { i, (label, icon) ->
                FloatingBottomBarItem(
                    selected = model.tab == i,
                    onClick = { activate(i) },
                    label = label,
                    modifier = Modifier.defaultMinSize(minWidth = 76.dp),
                ) {
                    Icon(icon, null, Modifier.size(24.dp))
                    Text(label, fontSize = 11.sp, lineHeight = 14.sp, fontWeight = FontWeight.SemiBold, maxLines = 1)
                }
            }
        }
        Spacer(Modifier.height(BarGap))
        Spacer(Modifier.windowInsetsBottomHeight(WindowInsets.navigationBars))
    }
}

/** The run's capsule over the tabs: converting, its progress, then sharing what it made. */
@Composable
private fun RunCapsule(
    model: AppModel, backdrop: LayerBackdrop, tab: Animatable<Float, *>,
    share: (List<java.io.File>) -> Unit, modifier: Modifier,
) {
    val p = LocalPalette.current
    val m = LocalMotion.current
    val nav = with(LocalDensity.current) { WindowInsets.navigationBars.getBottom(this).toDp() }
    AnimatedVisibility(
        visible = model.items.isNotEmpty(),
        modifier = modifier
            .padding(start = 28.dp, end = 28.dp, bottom = nav + BarGap + TabBarHeight + 12.dp)
            .graphicsLayer {
                // Leaving for the settings it slides down and fades.
                val away = tab.value.coerceIn(0f, 1f)
                alpha = 1f - away
                translationY = away * 28.dp.toPx()
                val s = 1f - 0.08f * away
                scaleX = s; scaleY = s
            }
            .then(if (model.tab == 0) Modifier else Modifier.blockInput()),
        enter = slideInVertically(m.pop()) { it } + scaleIn(m.pop(), 0.9f) + fadeIn(),
        exit = slideOutVertically(m.settle()) { it } + scaleOut(m.settle(), 0.9f) + fadeOut(),
    ) {
        val state = when {
            model.running -> 0
            model.pending > 0 -> 1
            else -> 2
        }
        AnimatedContent(state, transitionSpec = {
            (fadeIn(m.snap()) + scaleIn(m.pop(), 0.92f)) togetherWith (fadeOut(m.snap()) + scaleOut(m.snap(), 1.04f))
        }, label = "capsule") { s ->
            when (s) {
                0 -> RunningCapsule(model, backdrop)
                1 -> CapsuleButton(
                    "转换 ${model.pending} 个文件", Icons.convert, backdrop, tint = p.accent, ink = p.accentInk,
                    modifier = Modifier.fillMaxWidth(),
                ) { model.start() }
                else -> Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    CapsuleButton("全部分享", Icons.share, backdrop, tint = p.accent, ink = p.accentInk, modifier = Modifier.weight(1f)) {
                        share(model.doneFiles)
                    }
                    if (model.galleryFiles.isNotEmpty()) {
                        CapsuleButton("存到相册", Icons.save, backdrop, tint = null, ink = p.ink, modifier = Modifier.weight(1f)) {
                            model.saveToGallery()
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun CapsuleButton(
    label: String, icon: ImageVector, backdrop: LayerBackdrop, tint: Color?, ink: Color,
    modifier: Modifier = Modifier, onClick: () -> Unit,
) {
    val p = LocalPalette.current
    val source = remember { MutableInteractionSource() }
    val down = rememberPress(source)
    Row(
        modifier
            .height(56.dp)
            .glass(backdrop, p, tint = tint, pressed = { down })
            .clickable(source, null, role = Role.Button, onClick = onClick)
            .padding(horizontal = 20.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp, Alignment.CenterHorizontally),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icon, null, Modifier.size(19.dp), tint = ink)
        Text(label, color = ink, fontSize = 16.sp, fontWeight = FontWeight.ExtraBold, maxLines = 1)
    }
}

@Composable
private fun RunningCapsule(model: AppModel, backdrop: LayerBackdrop) {
    val p = LocalPalette.current
    val m = LocalMotion.current
    val todo = model.items.filter { it.id in model.runIds }
    val total = todo.size.coerceAtLeast(1)
    val doneN = todo.count { it.status == dev.formigo.app.Status.DONE || it.status == dev.formigo.app.Status.FAILED }
    val fraction = todo.sumOf { (if (it.status == dev.formigo.app.Status.DONE) 1f else it.progress).toDouble() }.toFloat() / total
    val shown = remember { Animatable(0f) }
    LaunchedEffect(fraction) { shown.animateTo(fraction, m.settle()) }
    Box(
        Modifier
            .fillMaxWidth()
            .height(56.dp)
            .glass(backdrop, p)
            .drawBehind {
                val inset = 6.dp.toPx()
                val h = size.height - 2 * inset
                val w = (size.width - 2 * inset) * shown.value.coerceIn(0f, 1f)
                if (w > 0) drawRoundRect(
                    p.accent.copy(alpha = 0.22f), androidx.compose.ui.geometry.Offset(inset, inset),
                    Size(w.coerceAtLeast(h), h), CornerRadius(h / 2),
                )
            },
        contentAlignment = Alignment.Center,
    ) {
        Text("正在转换 $doneN / $total", color = p.ink, fontSize = 16.sp, fontWeight = FontWeight.ExtraBold)
    }
}
