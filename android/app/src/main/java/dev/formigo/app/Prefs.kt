package dev.formigo.app

import android.content.Context
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dev.formigo.app.convert.Formats
import dev.formigo.app.convert.Kind
import dev.formigo.app.convert.Options
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

/** What Formigo remembers between runs. */
data class Prefs(
    val appearance: String = "system", // "system", "light" or "dark"
    val accent: String = dev.formigo.app.ui.DEFAULT_ACCENT,
    val motion: String = "full", // "full", "calm" or "off"
    val mascot: Boolean = true,
    /** Pure black in the dark appearance, for OLED screens. */
    val oled: Boolean = false,
    val targets: Map<Kind, String> = Kind.entries.associateWith { Formats.defaultTarget(it) },
    val workers: Int = 2,
    val suffix: String = "",
    val notify: Boolean = true,
    val options: Options = Options(),
)

private val Context.store by preferencesDataStore("settings")

class PrefsStore(private val context: Context) {
    private object K {
        val appearance = stringPreferencesKey("appearance")
        val accent = stringPreferencesKey("accent")
        val motion = stringPreferencesKey("motion")
        val mascot = booleanPreferencesKey("mascot")
        val oled = booleanPreferencesKey("oled")
        val workers = intPreferencesKey("workers")
        val suffix = stringPreferencesKey("suffix")
        val notify = booleanPreferencesKey("notify")
        val quality = intPreferencesKey("quality")
        val scale = intPreferencesKey("scale")
        val depth = intPreferencesKey("depth")
        val mono = booleanPreferencesKey("mono")
        val indent = intPreferencesKey("indent")
        val videoSize = intPreferencesKey("videoSize")
        val pdfScale = intPreferencesKey("pdfScale")
        fun target(k: Kind) = stringPreferencesKey("target.${k.id}")
    }

    val prefs: Flow<Prefs> = context.store.data.map { read(it) }

    private fun read(p: Preferences): Prefs {
        val d = Prefs()
        val o = d.options
        return Prefs(
            appearance = p[K.appearance]?.takeIf { it in setOf("system", "light", "dark") } ?: d.appearance,
            accent = p[K.accent]?.takeIf { id -> dev.formigo.app.ui.accents.any { it.id == id } } ?: d.accent,
            motion = p[K.motion]?.takeIf { it in setOf("full", "calm", "off") } ?: d.motion,
            mascot = p[K.mascot] ?: d.mascot,
            oled = p[K.oled] ?: d.oled,
            targets = Kind.entries.associateWith { k ->
                p[K.target(k)]?.takeIf { id -> Formats.kindTargets(k).any { it.id == id } } ?: Formats.defaultTarget(k)
            },
            workers = (p[K.workers] ?: d.workers).coerceIn(1, 4),
            suffix = p[K.suffix] ?: d.suffix,
            notify = p[K.notify] ?: d.notify,
            options = Options(
                quality = (p[K.quality] ?: o.quality).coerceIn(10, 100),
                scale = (p[K.scale] ?: o.scale).coerceIn(Options.SCALES.indices),
                depth = (p[K.depth] ?: o.depth).takeIf { it in Options.DEPTHS } ?: 16,
                mono = p[K.mono] ?: o.mono,
                indent = (p[K.indent] ?: o.indent).coerceIn(Options.INDENTS.indices),
                videoSize = (p[K.videoSize] ?: o.videoSize).coerceIn(Options.VIDEO_SIZES.indices),
                pdfScale = (p[K.pdfScale] ?: o.pdfScale).coerceIn(Options.PDF_SCALES.indices),
            ),
        )
    }

    suspend fun save(v: Prefs) {
        context.store.edit { p ->
            p[K.appearance] = v.appearance
            p[K.accent] = v.accent
            p[K.motion] = v.motion
            p[K.mascot] = v.mascot
            p[K.oled] = v.oled
            p[K.workers] = v.workers
            p[K.suffix] = v.suffix
            p[K.notify] = v.notify
            v.targets.forEach { (k, t) -> p[K.target(k)] = t }
            val o = v.options
            p[K.quality] = o.quality
            p[K.scale] = o.scale
            p[K.depth] = o.depth
            p[K.mono] = o.mono
            p[K.indent] = o.indent
            p[K.videoSize] = o.videoSize
            p[K.pdfScale] = o.pdfScale
        }
    }
}
