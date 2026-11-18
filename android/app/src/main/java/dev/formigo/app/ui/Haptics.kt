package dev.formigo.app.ui

import android.content.Context
import android.os.Build
import android.os.VibrationEffect
import android.os.Vibrator
import android.os.VibratorManager

/** A felt tap from the vibrator itself, so it comes through whatever the system's touch feedback is set to. */
object Haptics {
    private fun vibrator(c: Context): Vibrator? =
        if (Build.VERSION.SDK_INT >= 31) c.getSystemService(VibratorManager::class.java)?.defaultVibrator
        else @Suppress("DEPRECATION") c.getSystemService(Vibrator::class.java)

    /** A crisp click, as a boop of the gopher. */
    fun click(c: Context) {
        val v = vibrator(c)?.takeIf { it.hasVibrator() } ?: return
        if (Build.VERSION.SDK_INT >= 29) v.vibrate(VibrationEffect.createPredefined(VibrationEffect.EFFECT_CLICK))
        else v.vibrate(VibrationEffect.createOneShot(18, VibrationEffect.DEFAULT_AMPLITUDE))
    }
}
