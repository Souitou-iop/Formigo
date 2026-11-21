package dev.formigo.app

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

/** The app's releases on GitHub, which the settings' 检查更新 reads. */
object Update {
    /** The page of the newest release, where its APK and its notes are. */
    const val releases = "https://github.com/Souitou-iop/Formigo/releases/latest"
    private const val latest = "https://api.github.com/repos/Souitou-iop/Formigo/releases/latest"

    /** What a check made of the newest release. */
    sealed class State {
        /** A newer version, by its number. */
        data class Available(val version: String) : State()
        data object UpToDate : State()
        /** GitHub could not be reached. */
        data object Offline : State()
    }

    /** Asks GitHub for the newest release, beside the version running. */
    suspend fun check(running: String): State = withContext(Dispatchers.IO) {
        val conn = URL(latest).openConnection() as HttpURLConnection
        try {
            conn.connectTimeout = 10_000
            conn.readTimeout = 10_000
            conn.setRequestProperty("Accept", "application/vnd.github+json")
            conn.setRequestProperty("User-Agent", "Formigo")
            if (conn.responseCode !in 200..299) return@withContext State.Offline
            val tag = JSONObject(conn.inputStream.bufferedReader().readText()).optString("tag_name")
            if (tag.isEmpty()) return@withContext State.Offline
            if (newer(tag, running)) State.Available(tag.removePrefix("v")) else State.UpToDate
        } catch (_: Exception) {
            State.Offline
        } finally {
            conn.disconnect()
        }
    }

    /** Whether release runs ahead of running: the first version number that differs. */
    fun newer(release: String, running: String): Boolean {
        val a = release.removePrefix("v").split('.').map { it.toIntOrNull() ?: 0 }
        val b = running.split('.').map { it.toIntOrNull() ?: 0 }
        for (i in 0 until maxOf(a.size, b.size)) {
            val x = a.getOrElse(i) { 0 }
            val y = b.getOrElse(i) { 0 }
            if (x != y) return x > y
        }
        return false
    }
}
