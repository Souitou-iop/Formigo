package dev.formigo.app.ui

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
import dev.formigo.app.AppModel
import dev.formigo.app.BuildConfigLite
import dev.formigo.app.convert.Formats
import dev.formigo.app.convert.Kind

private val appearances = listOf("system" to "跟随系统", "light" to "浅色", "dark" to "深色")
private val motions = listOf("full" to "完整", "calm" to "安静", "off" to "关闭")

@Composable
fun SettingsPage(model: AppModel) {
    val p = LocalPalette.current
    val prefs = model.prefs
    val context = LocalContext.current
    val askNotify = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { ok ->
        model.update { it.copy(notify = ok) }
    }
    Column(Modifier.fillMaxSize()) {
        PageHeader("设置")
        LazyColumn(contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = bottomRoom(false))) {
            item {
                Group("外观") {
                    Row("外观") {
                        val i = appearances.indexOfFirst { it.first == prefs.appearance }.coerceAtLeast(0)
                        PickerPill("外观", appearances.map { it.second }, i, { j -> model.update { it.copy(appearance = appearances[j].first) } })
                    }
                    Divider()
                    Row("OLED 纯黑") {
                        // Only the dark appearance turns black; in light it waits, dimmed.
                        Switch(
                            prefs.oled, { v -> model.update { it.copy(oled = v) } }, enabled = p.dark,
                            colors = switchColors(),
                        )
                    }
                    Divider()
                    Row("主题色") { Swatches(prefs.accent) { id -> model.update { it.copy(accent = id) } } }
                    Divider()
                    Row("动画") {
                        val i = motions.indexOfFirst { it.first == prefs.motion }.coerceAtLeast(0)
                        PickerPill("动画", motions.map { it.second }, i, { j -> model.update { it.copy(motion = motions[j].first) } })
                    }
                    Divider()
                    Row("小地鼠") {
                        Switch(prefs.mascot, { v -> model.update { it.copy(mascot = v) } }, colors = switchColors())
                    }
                }
            }
            item {
                Group("默认格式") {
                    Kind.entries.forEachIndexed { n, k ->
                        if (n > 0) Divider()
                        val choices = Formats.kindTargets(k)
                        val i = choices.indexOfFirst { it.id == prefs.targets[k] }.coerceAtLeast(0)
                        Row(k.label, badge = k) {
                            PickerPill(k.label, choices.map { it.label }, i, { j ->
                                model.update { it.copy(targets = it.targets + (k to choices[j].id)) }
                            })
                        }
                    }
                }
            }
            item {
                Group("转换") {
                    Row("同时转换数量") {
                        Segmented(listOf("1", "2", "3", "4"), prefs.workers - 1, { i -> model.update { it.copy(workers = i + 1) } })
                    }
                    Divider()
                    Row("文件名后缀") { SuffixField(prefs.suffix) { s -> model.update { it.copy(suffix = s) } } }
                    Divider()
                    Row("完成后通知") {
                        Switch(prefs.notify, { v ->
                            if (v && Build.VERSION.SDK_INT >= 33 &&
                                ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
                            ) askNotify.launch(Manifest.permission.POST_NOTIFICATIONS)
                            else model.update { it.copy(notify = v) }
                        }, colors = switchColors())
                    }
                }
            }
            item {
                Group("关于") {
                    Row("版本") { Text(BuildConfigLite.version(context), color = p.muted, fontSize = 15.sp, modifier = Modifier.padding(end = 12.dp)) }
                    Divider()
                    Row("检查更新") {
                        SoftButton(
                            onClick = { model.checkForUpdates() },
                            shape = RoundedCornerShape(10.dp),
                            label = "检查更新",
                            modifier = Modifier.height(38.dp),
                        ) {
                            Text("检查更新", color = p.muted, fontSize = 15.sp, modifier = Modifier.padding(horizontal = 12.dp))
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun Group(title: String, rows: @Composable () -> Unit) {
    val p = LocalPalette.current
    Column(Modifier.padding(top = 18.dp)) {
        Text(title, color = p.muted, fontSize = 13.sp, fontWeight = FontWeight.ExtraBold, modifier = Modifier.padding(start = 8.dp, bottom = 8.dp))
        Column(
            Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(18.dp))
                .background(p.surface)
                .border(1.5.dp, p.line, RoundedCornerShape(18.dp))
                .padding(start = 16.dp, end = 4.dp),
        ) { rows() }
    }
}

@Composable
private fun Divider() = HorizontalDivider(color = LocalPalette.current.line)

/** A setting: its name, and at the end the control, the row's only target. */
@Composable
private fun Row(label: String, badge: Kind? = null, control: @Composable () -> Unit) {
    val p = LocalPalette.current
    androidx.compose.foundation.layout.Row(
        Modifier.fillMaxWidth().heightIn(min = 56.dp).padding(vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (badge != null) {
            KindBadge(badge, 24.dp)
            Box(Modifier.width(12.dp))
        }
        Text(label, color = p.ink, fontSize = 16.sp, modifier = Modifier.weight(1f), maxLines = 1)
        Box(Modifier.padding(end = 8.dp)) { control() }
    }
}

/** The accents, the chosen one ringed. */
@Composable
private fun Swatches(chosen: String, onChoose: (String) -> Unit) {
    val p = LocalPalette.current
    val m = LocalMotion.current
    androidx.compose.foundation.layout.Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        for (a in accents) {
            val on = a.id == chosen
            val ring by animateFloatAsState(if (on) 1f else 0f, m.pop(), label = "ring")
            Box(
                Modifier
                    .size(30.dp)
                    .clip(RoundedCornerShape(50))
                    .clickable(role = Role.RadioButton) { onChoose(a.id) }
                    .semantics { selected = on; contentDescription = a.name },
                contentAlignment = Alignment.Center,
            ) {
                Box(Modifier.size(30.dp).graphicsLayer { scaleX = 0.7f + 0.3f * ring; scaleY = 0.7f + 0.3f * ring; alpha = ring }
                    .border(2.dp, if (p.dark) a.dark else a.light, RoundedCornerShape(50)))
                Box(Modifier.size(20.dp).background(if (p.dark) a.dark else a.light, RoundedCornerShape(50)).border(1.5.dp, p.outline, RoundedCornerShape(50)))
            }
        }
    }
}

@Composable
private fun SuffixField(value: String, onChange: (String) -> Unit) {
    val p = LocalPalette.current
    var text by remember { mutableStateOf(value) }
    BasicTextField(
        text,
        { t ->
            val clean = t.replace(Regex("[/\\\\:*?\"<>|\\n]"), "").take(24)
            text = clean
            onChange(clean)
        },
        singleLine = true,
        textStyle = TextStyle(color = p.ink, fontSize = 15.sp, textAlign = TextAlign.End),
        cursorBrush = SolidColor(p.accent),
        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
        modifier = Modifier.width(132.dp),
        decorationBox = { inner ->
            Box(
                Modifier.clip(RoundedCornerShape(10.dp)).background(p.bg).border(1.dp, p.line, RoundedCornerShape(10.dp))
                    .padding(horizontal = 10.dp, vertical = 8.dp),
                contentAlignment = Alignment.CenterEnd,
            ) {
                if (text.isEmpty()) Text("无", color = p.faint, fontSize = 15.sp)
                inner()
            }
        },
    )
}
