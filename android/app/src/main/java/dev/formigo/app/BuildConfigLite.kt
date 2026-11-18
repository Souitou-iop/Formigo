package dev.formigo.app

import android.content.Context

object BuildConfigLite {
    fun version(c: Context): String = try {
        c.packageManager.getPackageInfo(c.packageName, 0).versionName ?: ""
    } catch (_: Exception) {
        ""
    }
}
