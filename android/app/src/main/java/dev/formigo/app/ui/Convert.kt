package dev.formigo.app.ui

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.keyframes
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.scaleOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.formigo.app.AppModel
import dev.formigo.app.Item
import dev.formigo.app.R
import dev.formigo.app.Status
import dev.formigo.app.convert.Format
import dev.formigo.app.convert.Formats
import dev.formigo.app.convert.Kind
import dev.formigo.app.convert.optionsFor
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import java.io.File

@Composable
fun ConvertPage(model: AppModel, pickers: Pickers, share: (List<File>) -> Unit) {
    val m = LocalMotion.current
    Column(Modifier.fillMaxSize()) {
        PageHeader("转换")
        AnimatedContent(
            model.items.isEmpty(),
            transitionSpec = { (fadeIn(m.settle()) + scaleIn(m.settle(), 0.985f)) togetherWith (fadeOut(tween(200)) + scaleOut(tween(200), 1.015f)) },
            label = "stage",
            modifier = Modifier.weight(1f),
        ) { empty ->
            if (empty) Hero(model, pickers) else Queue(model, pickers, share)
        }
    }
}

// The empty page: the gopher, and where files come from.

@Composable
private fun Hero(model: AppModel, pickers: Pickers) {
    val p = LocalPalette.current
    Column(
        Modifier.fillMaxSize().padding(start = 24.dp, end = 24.dp, bottom = bottomRoom(false) - 20.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        OrbitStage(340.dp, model.prefs.mascot)
        Spacer(Modifier.height(4.dp))
        Text("把文件丢给 Formigo", color = p.ink, fontSize = 25.sp, fontWeight = FontWeight.ExtraBold, letterSpacing = (-0.3).sp)
        Spacer(Modifier.height(26.dp))
        Column(Modifier.width(280.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            StickerButton("从相册选择", Icons.photos, pickers.photos, Modifier.fillMaxWidth(), primary = true, enabled = !model.importing, keyHaptics = true)
            StickerButton("选择文件", Icons.folder, pickers.files, Modifier.fillMaxWidth(), primary = false, enabled = !model.importing, keyHaptics = true)
        }
    }
}

// The queue.

@Composable
private fun Queue(model: AppModel, pickers: Pickers, share: (List<File>) -> Unit) {
    val p = LocalPalette.current
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = bottomRoom(true)),
    ) {
        item(key = "head") { QueueHead(model, Modifier.animateItem()) }
        val common = model.commonChoices()
        if (common.isNotEmpty()) {
            item(key = "all") { AllTo(model, common, Modifier.animateItem()) }
        }
        for (k in model.kinds) {
            item(key = "group-$k") { GroupHead(model, k, Modifier.animateItem().padding(top = 18.dp)) }
            items(model.items.filter { it.kind == k }, key = { it.id }) { item ->
                FileRow(model, item, share, Modifier.animateItem(fadeInSpec = null, fadeOutSpec = tween(180)).padding(top = 8.dp))
            }
        }
        item(key = "more") { AddMore(model, pickers, Modifier.animateItem().padding(top = 18.dp)) }
    }
}

@Composable
private fun QueueHead(model: AppModel, modifier: Modifier) {
    val p = LocalPalette.current
    Row(modifier.fillMaxWidth().padding(start = 4.dp, end = 0.dp, bottom = 2.dp), verticalAlignment = Alignment.CenterVertically) {
        Text("文件", color = p.ink, fontSize = 15.sp, fontWeight = FontWeight.ExtraBold)
        Text("  ${model.items.size}", color = p.faint, fontSize = 13.sp)
        Spacer(Modifier.weight(1f))
        Text(bytes(model.items.sumOf { it.size }), color = p.faint, fontSize = 12.sp)
        Spacer(Modifier.width(6.dp))
        SoftButton({ model.clear() }, enabled = !model.running, shape = RoundedCornerShape(10.dp), label = "清空") {
            Row(Modifier.padding(horizontal = 10.dp, vertical = 7.dp), verticalAlignment = Alignment.CenterVertically) {
                Icon(Icons.sweep, null, Modifier.size(15.dp), tint = p.ink)
                Text(" 清空", color = p.ink, fontSize = 12.5.sp, fontWeight = FontWeight.SemiBold)
            }
        }
    }
}

/** For a batch of several kinds: one format they all go to. */
@Composable
private fun AllTo(model: AppModel, common: List<Format>, modifier: Modifier) {
    val p = LocalPalette.current
    val chosen = common.firstOrNull { f -> model.items.all { model.targetOf(it) == f.id } }?.id
    Column(
        modifier
            .fillMaxWidth()
            .padding(top = 12.dp)
            .clip(RoundedCornerShape(18.dp))
            .background(p.surface)
            .border(1.5.dp, p.line, RoundedCornerShape(18.dp))
            .padding(vertical = 12.dp),
    ) {
        Text("全部转为", color = p.ink, fontSize = 14.sp, fontWeight = FontWeight.ExtraBold, modifier = Modifier.padding(start = 14.dp, bottom = 8.dp))
        Row(
            Modifier.horizontalScroll(rememberScrollState()).padding(horizontal = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            for (f in common) FormatTile(f, chosen == f.id, null, enabled = !model.running) { model.setAll(f.id) }
        }
    }
}

@Composable
private fun GroupHead(model: AppModel, k: Kind, modifier: Modifier) {
    val p = LocalPalette.current
    val target = model.groupTarget(k)
    val hasOptions = model.groupTargetsInUse(k).any { optionsFor(k, it).isNotEmpty() }
    Column(modifier.fillMaxWidth()) {
        Row(Modifier.padding(horizontal = 2.dp), verticalAlignment = Alignment.CenterVertically) {
            KindBadge(k, 30.dp)
            Column(Modifier.weight(1f).padding(start = 10.dp)) {
                Text(k.label, color = p.ink, fontSize = 15.sp, fontWeight = FontWeight.ExtraBold)
                Text("${model.count(k)} 个 · ${model.exts(k)}", color = p.faint, fontSize = 11.5.sp, maxLines = 1)
            }
            if (hasOptions) {
                SoftButton(
                    { model.optionsKind = k }, background = p.surface, border = p.line, enabled = !model.running,
                    modifier = Modifier.height(34.dp), label = "${k.label}选项",
                ) {
                    Row(Modifier.padding(horizontal = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                        Icon(Icons.sliders, null, Modifier.size(14.dp), tint = p.ink)
                        Text(" 选项", color = p.ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                    }
                }
            }
        }
        Row(
            Modifier.fillMaxWidth().padding(top = 10.dp).horizontalScroll(rememberScrollState()),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            for (f in model.groupChoices(k)) {
                FormatTile(f, f.id == target, k, enabled = !model.running) { model.setGroupTarget(k, f.id) }
            }
        }
    }
}

/** A format to choose: its name and what it is for. The chosen one is a sticker in the accent. */
@Composable
fun FormatTile(f: Format, on: Boolean, kind: Kind?, enabled: Boolean = true, width: Dp = 78.dp, onClick: () -> Unit) {
    val p = LocalPalette.current
    val m = LocalMotion.current
    val source = remember { MutableInteractionSource() }
    val down = rememberPress(source)
    val shape = RoundedCornerShape(14.dp)
    val bg by animateColorAsState(if (on) p.accent else p.surface, m.snap(), label = "tile")
    val ink by animateColorAsState(if (on) p.accentInk else p.ink, m.snap(), label = "tileInk")
    val edge by animateColorAsState(if (on) p.outline else p.line, m.snap(), label = "tileEdge")
    val lift = remember { Animatable(if (on) 1f else 0f) }
    LaunchedEffect(on) { lift.animateTo(if (on) 1f else 0f, m.pop()) }
    Column(
        Modifier
            .width(width)
            .height(58.dp)
            .graphicsLayer {
                translationY = -2.dp.toPx() * lift.value + 2.dp.toPx() * down
                val s = 1f - 0.04f * down
                scaleX = s; scaleY = s
                alpha = if (enabled) 1f else 0.5f
            }
            .drawBehind {
                val l = lift.value
                if (l > 0.01f) drawRoundRect(p.outline.copy(alpha = l), Offset(0f, 3.dp.toPx() * l), size, CornerRadius(14.dp.toPx()))
            }
            .clip(shape)
            .background(bg)
            .border(if (on) 2.dp else 1.5.dp, edge, shape)
            .clickable(source, null, enabled = enabled, role = Role.RadioButton, onClick = onClick),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Text(f.label, color = ink, fontSize = 14.sp, fontWeight = FontWeight.ExtraBold, letterSpacing = 0.3.sp, maxLines = 1)
        Text(f.note, color = ink.copy(alpha = 0.7f), fontSize = 10.5.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

// A file's row.

@Composable
private fun FileRow(model: AppModel, it: Item, share: (List<File>) -> Unit, modifier: Modifier) {
    val p = LocalPalette.current
    val m = LocalMotion.current
    // It drops in as a sticker on the page, one after another.
    val enter = remember { Animatable(if (it.entered) 1f else 0f) }
    LaunchedEffect(Unit) {
        if (!it.entered) {
            kotlinx.coroutines.delay(minOf(it.order, 12) * 40L)
            enter.animateTo(1f, m.pop())
            it.entered = true
        }
    }
    // Finishing washes the row in green for a moment; failing shakes it.
    val flash = remember { Animatable(0f) }
    LaunchedEffect(it.doneRun) {
        if (it.doneRun > 0) {
            flash.snapTo(0f)
            flash.animateTo(1f, tween(90))
            flash.animateTo(0f, tween(1000))
        }
    }
    val shake = remember { Animatable(0f) }
    LaunchedEffect(it.failRun) {
        if (it.failRun > 0 && m.enabled) shake.animateTo(0f, keyframes {
            durationMillis = 380
            -7f at 60; 6f at 140; -4f at 220; 2f at 300
        })
    }
    val edge by animateColorAsState(if (it.status == Status.FAILED) p.bad else p.line, m.snap(), label = "edge")
    val shape = RoundedCornerShape(16.dp)
    Row(
        modifier
            .fillMaxWidth()
            .graphicsLayer {
                val e = enter.value
                translationY = (1f - e) * 22.dp.toPx()
                translationX = shake.value.dp.toPx()
                val s = 0.94f + 0.06f * e
                scaleX = s; scaleY = s
                rotationZ = -1.5f * (1f - e)
                alpha = e.coerceIn(0f, 1f)
            }
            .height(70.dp)
            .clip(shape)
            .drawBehind { drawRect(lerp(p.surface, p.ok, 0.18f * flash.value)) }
            .border(1.5.dp, edge, shape)
            .padding(horizontal = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // Its kind: a tile in its hue, with its extension.
        val tileShape = RoundedCornerShape(11.dp)
        Column(
            Modifier
                .size(42.dp, 44.dp)
                .drawBehind { drawRoundRect(p.outline, Offset(0f, 2.dp.toPx()), size, CornerRadius(11.dp.toPx())) }
                .background(p.tint(it.kind).compositeOverSafe(p.surface), tileShape)
                .border(1.5.dp, p.outline, tileShape),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center,
        ) {
            Icon(Icons.kind(it.kind), null, Modifier.size(15.dp), tint = p.hueInk(it.kind))
            Text(it.ext, color = p.hueInk(it.kind), fontSize = 8.5.sp, fontWeight = FontWeight.ExtraBold, maxLines = 1, letterSpacing = 0.3.sp)
        }
        Column(Modifier.weight(1f).padding(horizontal = 12.dp)) {
            Text(it.name, color = p.ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.MiddleEllipsis)
            Spacer(Modifier.height(3.dp))
            StatusLine(model, it)
        }
        TargetChip(model, it)
        Box(Modifier.padding(start = 8.dp).size(34.dp), contentAlignment = Alignment.Center) {
            RowState(model, it, share)
        }
    }
}

@Composable
private fun StatusLine(model: AppModel, it: Item) {
    val p = LocalPalette.current
    val base = bytes(it.size) + (it.meta?.let { m -> "  ·  $m" } ?: "")
    when {
        it.status == Status.FAILED -> Text(it.error ?: "转换失败", color = p.bad, fontSize = 11.5.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
        it.status == Status.DONE && it.already -> {
            val f = Formats.format(model.targetOf(it))
            Text("已是 ${f?.label} 格式", color = p.faint, fontSize = 11.5.sp, maxLines = 1)
        }
        it.status == Status.DONE -> {
            val out = it.outSize
            val compare = it.kind == Kind.IMAGE || it.kind == Kind.AUDIO || it.kind == Kind.VIDEO
            val r = if (it.size > 0) out.toDouble() / it.size - 1 else 0.0
            val n = it.outputs.size
            val text = buildString {
                append(bytes(it.size)).append(" → ").append(bytes(out))
                if (n > 1) append("  ·  $n 个文件")
                if (compare && r <= -0.05) append("  −${(-r * 100).toInt()}%")
                if (compare && r >= 0.05) append("  +${(r * 100).toInt()}%")
            }
            Text(text, color = if (compare && r <= -0.05) p.ok else p.muted, fontSize = 11.5.sp, maxLines = 1, fontWeight = FontWeight.Medium)
        }
        else -> Text(base, color = p.muted, fontSize = 11.5.sp, maxLines = 1)
    }
}

/** A file's format: a chip whose menu gives it one of its own, or has it follow its group's. */
@Composable
private fun TargetChip(model: AppModel, it: Item) {
    val p = LocalPalette.current
    var open by remember { mutableStateOf(false) }
    val target = Formats.format(model.targetOf(it))
    val own = it.own != null
    val hue = p.hue(it.kind)
    val ink = p.hueInk(it.kind)
    Box {
        SoftButton(
            { open = true }, shape = RoundedCornerShape(13.dp), background = p.tint(it.kind),
            border = if (own) p.outline else hue, enabled = !model.running && it.status != Status.WORKING,
            modifier = Modifier.height(28.dp), label = "${it.name} 的目标格式",
        ) {
            Text(target?.label ?: "", color = ink, fontSize = 11.5.sp, fontWeight = FontWeight.ExtraBold, letterSpacing = 0.3.sp, modifier = Modifier.padding(start = 10.dp))
            Icon(Icons.upDown, null, Modifier.padding(start = 2.dp, end = 7.dp).size(11.dp), tint = ink)
        }
        FormigoMenu(open, { open = false }) {
            val group = Formats.format(Formats.resolve(it.source, null, model.groupTarget(it.kind)))
            MenuItem("跟随分组", checked = !own, note = group?.label) { open = false; model.setOwn(it, null) }
            HorizontalDivider(color = p.line, modifier = Modifier.padding(vertical = 4.dp))
            for (f in Formats.choicesOf(it.source)) {
                MenuItem(f.label, checked = own && f.id == it.own, note = f.note) { open = false; model.setOwn(it, f.id) }
            }
        }
    }
}

@Composable
private fun RowState(model: AppModel, it: Item, share: (List<File>) -> Unit) {
    val p = LocalPalette.current
    val m = LocalMotion.current
    AnimatedContent(
        it.status,
        transitionSpec = { (scaleIn(m.pop(), 0.4f) + fadeIn(tween(120))) togetherWith (scaleOut(tween(160), 1.4f) + fadeOut(tween(160))) },
        label = "state",
    ) { s ->
        when (s) {
            Status.WORKING -> ProgressRing({ it.progress })
            Status.DONE -> {
                val source = remember { MutableInteractionSource() }
                val down = rememberPress(source)
                Box(
                    Modifier
                        .size(34.dp)
                        .drawBehind { drawCircle(p.outline, center = center.copy(y = center.y + 2.5.dp.toPx() * (1 - down))) }
                        .graphicsLayer { translationY = 2.5.dp.toPx() * down }
                        .clip(RoundedCornerShape(50))
                        .background(p.accent)
                        .border(2.dp, p.outline, RoundedCornerShape(50))
                        .clickable(source, null, role = Role.Button, onClickLabel = "分享") { share(it.outputs) },
                    contentAlignment = Alignment.Center,
                ) { Icon(Icons.share, "分享", Modifier.size(17.dp), tint = p.accentInk) }
            }
            Status.FAILED -> SoftButton(
                { model.start() }, background = p.bad.copy(alpha = 0.14f), border = p.bad, enabled = !model.running,
                modifier = Modifier.size(30.dp), label = "重试",
            ) { Icon(Icons.retry, "重试", Modifier.fillMaxSize().padding(6.dp), tint = p.bad) }
            Status.WAITING -> if (model.running) WaitingDots() else SoftButton(
                { model.remove(it.id) }, shape = RoundedCornerShape(9.dp), modifier = Modifier.size(30.dp), label = "移除",
            ) { Icon(Icons.close, "移除", Modifier.fillMaxSize().padding(7.dp), tint = p.muted) }
        }
    }
}

@Composable
private fun WaitingDots() {
    val p = LocalPalette.current
    val m = LocalMotion.current
    val t = if (m.ambient) rememberInfiniteTransition(label = "dots") else null
    Row(horizontalArrangement = Arrangement.spacedBy(3.dp)) {
        for (i in 0..2) {
            val a = t?.animateFloat(
                0.35f, 1f,
                infiniteRepeatable(keyframes { durationMillis = 900; 0.35f at 0; 1f at 300; 0.35f at 900 }, RepeatMode.Restart, androidx.compose.animation.core.StartOffset(i * 150)),
                label = "dot",
            )
            Box(Modifier.size(5.dp).graphicsLayer { alpha = a?.value ?: 0.6f }.background(p.sky, RoundedCornerShape(50)))
        }
    }
}

@Composable
private fun AddMore(model: AppModel, pickers: Pickers, modifier: Modifier) {
    val p = LocalPalette.current
    var open by remember { mutableStateOf(false) }
    Box(modifier.fillMaxWidth()) {
        SoftButton(
            { open = true }, shape = RoundedCornerShape(16.dp), enabled = !model.importing,
            modifier = Modifier
                .fillMaxWidth()
                .height(52.dp)
                .drawBehind {
                    drawRoundRect(
                        p.lineStrong, style = Stroke(2.dp.toPx(), pathEffect = PathEffect.dashPathEffect(floatArrayOf(8.dp.toPx(), 6.dp.toPx()))),
                        cornerRadius = CornerRadius(16.dp.toPx()),
                    )
                },
            label = "添加更多文件",
        ) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center, verticalAlignment = Alignment.CenterVertically) {
                Icon(Icons.plus, null, Modifier.size(16.dp), tint = p.muted)
                Text(" 添加更多文件", color = p.muted, fontSize = 13.5.sp, fontWeight = FontWeight.SemiBold)
            }
        }
        FormigoMenu(open, { open = false }) {
            MenuItem("从相册选择") { open = false; pickers.photos() }
            MenuItem("选择文件") { open = false; pickers.files() }
        }
    }
}

fun bytes(n: Long): String = when {
    n < 1024 -> "$n B"
    n < 1024 * 1024 -> "%.1f KB".format(n / 1024.0)
    n < 1024L * 1024 * 1024 -> "%.1f MB".format(n / 1024.0 / 1024)
    else -> "%.2f GB".format(n / 1024.0 / 1024 / 1024)
}
