package dev.formigo.app

import android.content.Intent
import android.graphics.Color
import android.net.Uri
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.remember
import dev.formigo.app.ui.FormigoApp
import dev.formigo.app.ui.FormigoTheme
import dev.formigo.app.ui.Motion
import dev.formigo.app.ui.palette

class MainActivity : ComponentActivity() {
    private val model: AppModel by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (savedInstanceState == null) intent?.let { take(it) }
        setContent {
            val prefs = model.prefs
            val dark = when (prefs.appearance) {
                "light" -> false
                "dark" -> true
                else -> isSystemInDarkTheme()
            }
            DisposableEffect(dark) {
                val style = if (dark) SystemBarStyle.dark(Color.TRANSPARENT)
                else SystemBarStyle.light(Color.TRANSPARENT, Color.TRANSPARENT)
                enableEdgeToEdge(statusBarStyle = style, navigationBarStyle = style)
                if (Build.VERSION.SDK_INT >= 29) window.isNavigationBarContrastEnforced = false
                onDispose {}
            }
            val p = remember(dark, prefs.accent, prefs.oled) { palette(dark, prefs.accent, prefs.oled) }
            val motion = remember(prefs.motion) {
                Motion(enabled = prefs.motion != "off", ambient = prefs.motion == "full")
            }
            FormigoTheme(p, motion) { FormigoApp(model) }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        take(intent)
    }

    /** Files shared to Formigo from another app. */
    private fun take(intent: Intent) {
        val uris = when (intent.action) {
            Intent.ACTION_SEND -> listOfNotNull(
                if (Build.VERSION.SDK_INT >= 33) intent.getParcelableExtra(Intent.EXTRA_STREAM, Uri::class.java)
                else @Suppress("DEPRECATION") intent.getParcelableExtra(Intent.EXTRA_STREAM),
            )
            Intent.ACTION_SEND_MULTIPLE -> (
                if (Build.VERSION.SDK_INT >= 33) intent.getParcelableArrayListExtra(Intent.EXTRA_STREAM, Uri::class.java)
                else @Suppress("DEPRECATION") intent.getParcelableArrayListExtra(Intent.EXTRA_STREAM)
                ).orEmpty()
            else -> emptyList()
        }
        if (uris.isNotEmpty()) {
            model.tab = 0
            model.add(uris)
        }
    }
}
