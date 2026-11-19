package dev.formigo.app

import android.graphics.BitmapFactory
import android.media.MediaExtractor
import android.os.Build
import android.util.Log
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import dev.formigo.app.convert.Engine
import dev.formigo.app.convert.Formats
import dev.formigo.app.convert.Job
import dev.formigo.app.convert.Kind
import dev.formigo.app.convert.Options
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

/**
 * Converts a sample of every format the app reads (androidTest/assets/seeds)
 * to every format it offers for it, with the device's own decoders and
 * encoders, and checks that what it wrote reads back.
 */
@RunWith(AndroidJUnit4::class)
class EngineDeviceTest {

    private val instrumentation = InstrumentationRegistry.getInstrumentation()
    private val context = instrumentation.targetContext
    private val engine = Engine(context)

    @Test
    fun everySourceToEveryTarget() {
        val dir = File(context.cacheDir, "engine-test").apply { deleteRecursively(); mkdirs() }
        val assets = instrumentation.context.assets
        val failures = mutableListOf<String>()
        var tried = 0
        val seeds = assets.list("seeds").orEmpty().sorted()
        for (name in seeds) {
            val src = File(dir, name)
            assets.open("seeds/$name").use { input -> src.outputStream().use { input.copyTo(it) } }
            val source = Formats.detect(name, Build.VERSION.SDK_INT)
            if (source == null) {
                Log.i(TAG, "skip $name: not read on API ${Build.VERSION.SDK_INT}")
                continue
            }
            for (target in Formats.targetsOf(source)) {
                tried++
                val out = File(dir, "out/${source.id}-to-${target.id}")
                val problem = try {
                    val result = runBlocking {
                        withTimeout(60_000) {
                            engine.convert(Job(src, source, target.id, Options(), out, "seed")) {}
                        }
                    }
                    check(result, target.id, target.kind)
                } catch (e: Throwable) {
                    "${e.javaClass.simpleName}: ${e.message}"
                }
                val line = "${source.id} → ${target.id}: ${problem ?: "ok"}"
                Log.i(TAG, line)
                if (problem != null) failures += line
            }
        }
        Log.i(TAG, "$tried conversions from ${seeds.size} samples, ${failures.size} failed")
        assertTrue("${failures.size} of $tried failed:\n" + failures.joinToString("\n"), failures.isEmpty())
    }

    /** Why what was written does not read back, or null when it does. */
    private fun check(result: dev.formigo.app.convert.Result, target: String, kind: Kind): String? {
        if (result.files.isEmpty()) return "nothing written"
        for (f in result.files) {
            if (!f.exists() || f.length() == 0L) return "${f.name} is empty"
            when {
                target == "pdf" -> if (!f.readBytes().copyOfRange(0, 5).contentEquals("%PDF-".toByteArray())) return "${f.name} is not a PDF"
                kind == Kind.IMAGE -> if (BitmapFactory.decodeFile(f.path) == null) return "${f.name} does not decode"
                kind == Kind.VIDEO || kind == Kind.AUDIO -> {
                    val x = MediaExtractor()
                    try {
                        x.setDataSource(f.path)
                        if (x.trackCount == 0) return "${f.name} has no tracks"
                    } catch (e: Exception) {
                        return "${f.name} does not open: ${e.message}"
                    } finally {
                        x.release()
                    }
                }
                target == "zip" -> if (!f.readBytes().copyOfRange(0, 2).contentEquals("PK".toByteArray())) return "${f.name} is not a ZIP"
                else -> if (f.readText().isBlank()) return "${f.name} is blank"
            }
        }
        return null
    }

    companion object {
        const val TAG = "FormigoEngineTest"
    }
}
