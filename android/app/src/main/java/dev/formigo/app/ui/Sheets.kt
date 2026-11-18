package dev.formigo.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.formigo.app.AppModel
import dev.formigo.app.convert.Formats
import dev.formigo.app.convert.Opt
import dev.formigo.app.convert.Options
import dev.formigo.app.convert.optionsFor

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun FormigoSheet(onDismiss: () -> Unit, content: @Composable () -> Unit) {
    val p = LocalPalette.current
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
        containerColor = p.panel,
        contentColor = p.ink,
        shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
        dragHandle = {
            Box(Modifier.padding(top = 10.dp, bottom = 6.dp).size(40.dp, 5.dp).background(p.lineStrong, RoundedCornerShape(50)))
        },
    ) { content() }
}

/** What was recognized in the files just added, by kind, each with what it can become. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun FreshSheet(model: AppModel) {
    val p = LocalPalette.current
    val kinds = model.freshKinds.filter { model.count(it) > 0 }
    if (!model.freshOpen || kinds.isEmpty()) return
    FormigoSheet({ model.freshOpen = false }) {
        Column(Modifier.navigationBarsPadding()) {
            Text(
                "转换为", color = p.ink, fontSize = 24.sp, fontWeight = FontWeight.ExtraBold, letterSpacing = (-0.3).sp,
                modifier = Modifier.padding(start = 22.dp, end = 22.dp, top = 4.dp, bottom = 8.dp),
            )
            Column(
                Modifier.weight(1f, fill = false).verticalScroll(rememberScrollState()).padding(horizontal = 20.dp, vertical = 6.dp),
                verticalArrangement = Arrangement.spacedBy(22.dp),
            ) {
                val common = model.commonChoices()
                if (common.isNotEmpty()) {
                    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        Text("全部转为", color = p.ink, fontSize = 15.sp, fontWeight = FontWeight.ExtraBold)
                        val chosen = common.firstOrNull { f -> model.items.all { model.targetOf(it) == f.id } }?.id
                        TileGrid(common.size) { i, w -> val f = common[i]; FormatTile(f, chosen == f.id, null, width = w) { model.setAll(f.id) } }
                    }
                }
                for (k in kinds) {
                    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            KindBadge(k, 26.dp)
                            Text(k.label, color = p.ink, fontSize = 15.sp, fontWeight = FontWeight.ExtraBold, modifier = Modifier.padding(start = 10.dp))
                            Spacer(Modifier.weight(1f))
                            Text("${model.count(k)} 个 · ${model.exts(k)}", color = p.faint, fontSize = 12.sp, maxLines = 1)
                        }
                        val target = model.groupTarget(k)
                        val choices = model.groupChoices(k)
                        TileGrid(choices.size) { i, w -> val f = choices[i]; FormatTile(f, f.id == target, k, width = w) { model.setGroupTarget(k, f.id) } }
                    }
                }
            }
            Row(Modifier.fillMaxWidth().padding(start = 20.dp, end = 20.dp, top = 12.dp, bottom = 12.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                StickerButton("稍后", null, { model.freshOpen = false }, Modifier.width(104.dp), primary = false, lift = 3.dp, fontSize = 15.sp)
                StickerButton(
                    "转换 ${model.pending} 个文件", Icons.convert, { model.start() }, Modifier.weight(1f),
                    primary = true, enabled = !model.running, fontSize = 15.sp,
                )
            }
        }
    }
}

/** Tiles four to a row, as wide as the row allows. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun TileGrid(n: Int, tile: @Composable (Int, androidx.compose.ui.unit.Dp) -> Unit) {
    androidx.compose.foundation.layout.BoxWithConstraints(Modifier.fillMaxWidth()) {
        val w = (maxWidth - 8.dp * 3) / 4
        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(10.dp), maxItemsInEachRow = 4) {
            for (i in 0 until n) tile(i, w)
        }
    }
}

/** The options of the formats a kind's files go to, only those that shape them. */
@Composable
fun OptionsSheet(model: AppModel) {
    val p = LocalPalette.current
    val k = model.optionsKind ?: return
    val targets = model.groupTargetsInUse(k)
    val opts = Opt.entries.filter { o -> targets.any { o in optionsFor(k, it) } }
    if (opts.isEmpty()) { model.optionsKind = null; return }
    FormigoSheet({ model.optionsKind = null }) {
        Column(Modifier.navigationBarsPadding().padding(bottom = 16.dp)) {
            Row(Modifier.padding(start = 22.dp, end = 22.dp, top = 4.dp, bottom = 10.dp), verticalAlignment = Alignment.CenterVertically) {
                KindBadge(k, 28.dp)
                Text("${k.label}选项", color = p.ink, fontSize = 22.sp, fontWeight = FontWeight.ExtraBold, modifier = Modifier.padding(start = 10.dp))
                Spacer(Modifier.weight(1f))
                Text("→ " + targets.mapNotNull { Formats.format(it)?.label }.joinToString("、"), color = p.muted, fontSize = 13.sp, fontWeight = FontWeight.Bold, maxLines = 1)
            }
            Column(
                Modifier.padding(horizontal = 16.dp).background(p.surface, RoundedCornerShape(18.dp)).padding(horizontal = 16.dp),
            ) {
                val o = model.prefs.options
                opts.forEachIndexed { i, opt ->
                    if (i > 0) HorizontalDivider(color = p.line)
                    when (opt) {
                        Opt.QUALITY -> QualityRow(o.quality) { q -> model.setOptions { it.copy(quality = q) } }
                        Opt.SCALE -> OptionRow("尺寸") { PickerPill("尺寸", Options.SCALES, o.scale, { s -> model.setOptions { it.copy(scale = s) } }) }
                        Opt.DEPTH -> OptionRow("位深") {
                            Segmented(listOf("16 位", "24 位"), Options.DEPTHS.indexOf(o.depth), { i -> model.setOptions { it.copy(depth = Options.DEPTHS[i]) } })
                        }
                        Opt.MONO -> OptionRow("合并为单声道") {
                            Switch(o.mono, { v -> model.setOptions { it.copy(mono = v) } }, colors = switchColors())
                        }
                        Opt.INDENT -> OptionRow("缩进") { Segmented(Options.INDENTS, o.indent, { i -> model.setOptions { it.copy(indent = i) } }) }
                        Opt.VIDEO_SIZE -> OptionRow("分辨率") { PickerPill("分辨率", Options.VIDEO_SIZES, o.videoSize, { s -> model.setOptions { it.copy(videoSize = s) } }) }
                        Opt.PDF_SCALE -> OptionRow("清晰度") { Segmented(Options.PDF_SCALES, o.pdfScale, { i -> model.setOptions { it.copy(pdfScale = i) } }) }
                    }
                }
            }
        }
    }
}

@Composable
fun switchColors() = LocalPalette.current.let { p ->
    SwitchDefaults.colors(
        checkedThumbColor = p.accentInk, checkedTrackColor = p.accent, checkedBorderColor = p.accent,
        uncheckedThumbColor = p.faint, uncheckedTrackColor = p.bg, uncheckedBorderColor = p.lineStrong,
    )
}

@Composable
private fun OptionRow(label: String, control: @Composable () -> Unit) {
    val p = LocalPalette.current
    Row(Modifier.fillMaxWidth().heightIn(min = 56.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(label, color = p.ink, fontSize = 16.sp, modifier = Modifier.weight(1f))
        control()
    }
}

@Composable
private fun QualityRow(quality: Int, onChange: (Int) -> Unit) {
    val p = LocalPalette.current
    // The slider moves freely; the setting takes its value as the finger lifts.
    var v by remember(quality) { mutableFloatStateOf(quality.toFloat()) }
    Row(Modifier.fillMaxWidth().height(60.dp), verticalAlignment = Alignment.CenterVertically) {
        Text("画质", color = p.ink, fontSize = 16.sp, modifier = Modifier.width(52.dp))
        Slider(
            v, { v = it }, Modifier.weight(1f), valueRange = 10f..100f, onValueChangeFinished = { onChange(v.toInt()) },
            colors = SliderDefaults.colors(thumbColor = p.accent, activeTrackColor = p.accent, inactiveTrackColor = p.line),
        )
        Text(if (v >= 100f) "无损" else "${v.toInt()}", color = p.muted, fontSize = 15.sp, modifier = Modifier.width(44.dp).padding(start = 10.dp))
    }
}
