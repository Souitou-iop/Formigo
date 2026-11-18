package dev.formigo.app

import android.Manifest
import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.ClipData
import android.content.ContentValues
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.provider.MediaStore
import android.provider.OpenableColumns
import android.webkit.MimeTypeMap
import android.widget.Toast
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import androidx.core.content.FileProvider
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import dev.formigo.app.convert.Archive
import dev.formigo.app.convert.ConvertException
import dev.formigo.app.convert.Engine
import dev.formigo.app.convert.Format
import dev.formigo.app.convert.Formats
import dev.formigo.app.convert.Job
import dev.formigo.app.convert.Kind
import dev.formigo.app.convert.Options
import dev.formigo.app.convert.Source
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withPermit
import kotlinx.coroutines.withContext
import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.concurrent.atomic.AtomicInteger

enum class Status { WAITING, WORKING, DONE, FAILED }

/** A file on the queue. What changes is state, so that only its row builds again. */
class Item(
    val id: Int,
    val name: String,
    val file: File,
    val source: Source,
    val size: Long,
    /** The order it came in within its batch, for its row's entrance. */
    val order: Int,
) {
    val kind: Kind get() = source.kind
    val ext: String get() = Formats.ext(name).uppercase()
    var meta by mutableStateOf<String?>(null)
    /** A target of its own, else it follows its group. */
    var own by mutableStateOf<String?>(null)
    var status by mutableStateOf(Status.WAITING)
    var progress by mutableFloatStateOf(0f)
    var outputs by mutableStateOf<List<File>>(emptyList())
    var already by mutableStateOf(false)
    var error by mutableStateOf<String?>(null)
    /** Bumped when it finishes or fails, which flashes or shakes its row. */
    var doneRun by mutableIntStateOf(0)
    var failRun by mutableIntStateOf(0)
    /** Whether its row has made its entrance, which plays once. */
    var entered = false
    val outSize: Long get() = outputs.sumOf { it.length() }
}

class AppModel(app: Application) : AndroidViewModel(app) {
    private val ctx get() = getApplication<Application>()
    private val store = PrefsStore(app)
    private val engine = Engine(app)
    private val ids = AtomicInteger()

    var prefs by mutableStateOf(Prefs())
        private set
    var loaded by mutableStateOf(false)
        private set

    val items = mutableStateListOf<Item>()
    /** Each kind's format, set from its strip. */
    val groupTargets = mutableStateMapOf<Kind, String>()

    var tab by mutableIntStateOf(0)
    var running by mutableStateOf(false)
        private set
    var importing by mutableStateOf(false)
        private set
    /** The files of the run going on, or of the last. */
    var runIds by mutableStateOf<Set<Int>>(emptySet())
        private set

    /** The kinds just added, shown in the "convert to" sheet. */
    var freshKinds by mutableStateOf<List<Kind>>(emptyList())
    var freshOpen by mutableStateOf(false)
    var optionsKind by mutableStateOf<Kind?>(null)
    var addMenu by mutableStateOf(false)

    /** Bumped each time a run is done, to cheer. */
    var cheers by mutableIntStateOf(0)

    init {
        viewModelScope.launch {
            prefs = store.prefs.first()
            loaded = true
        }
        viewModelScope.launch(Dispatchers.IO) {
            // What earlier runs left: nothing is kept but what was shared or saved.
            File(ctx.cacheDir, "out").deleteRecursively()
            File(ctx.cacheDir, "imports").deleteRecursively()
        }
    }

    fun update(f: (Prefs) -> Prefs) {
        val next = f(prefs)
        if (next == prefs) return
        val targetsChanged = next.targets != prefs.targets
        prefs = next
        if (targetsChanged) {
            // A kind's default changed: groups that were following the old one follow the new.
            next.targets.forEach { (k, t) -> if (items.none { it.kind == k }) groupTargets[k] = t }
        }
        viewModelScope.launch { store.save(next) }
    }

    fun setOptions(f: (Options) -> Options) {
        update { it.copy(options = f(it.options)) }
        // Files done in formats these options shape are converted again on the next run.
        items.filter { it.status == Status.DONE && !it.already }.forEach { reset(it) }
    }

    // The queue's groups.

    val kinds: List<Kind> get() = Kind.entries.filter { k -> items.any { it.kind == k } }

    fun count(k: Kind) = items.count { it.kind == k }

    fun exts(k: Kind): String {
        val e = items.filter { it.kind == k }.map { it.ext }.distinct()
        return if (e.size > 3) e.take(3).joinToString("、") + "…" else e.joinToString("、")
    }

    fun groupTarget(k: Kind): String = groupTargets[k] ?: prefs.targets[k] ?: Formats.defaultTarget(k)

    fun targetOf(it: Item): String = Formats.resolve(it.source, it.own, groupTarget(it.kind))

    /** What a group's strip offers: whatever any of its files may become. */
    fun groupChoices(k: Kind): List<Format> = Formats.union(items.filter { it.kind == k }.map { it.source }.toSet())

    /** The targets of a group's files, for its options. */
    fun groupTargetsInUse(k: Kind): List<String> = items.filter { it.kind == k }.map { targetOf(it) }.distinct()

    /** Common targets of a batch of several kinds. */
    fun commonChoices(): List<Format> =
        if (kinds.size < 2) emptyList() else Formats.common(items.map { it.source }.toSet())

    fun setGroupTarget(k: Kind, t: String) {
        if (groupTarget(k) == t) return
        groupTargets[k] = t
        items.filter { it.kind == k && it.own == null }.forEach { reset(it) }
    }

    fun setOwn(it: Item, t: String?) {
        val before = targetOf(it)
        it.own = if (t == groupTarget(it.kind)) null else t
        if (targetOf(it) != before) reset(it)
    }

    /** Every file, whatever its kind, to [t]. */
    fun setAll(t: String) {
        kinds.forEach { groupTargets[it] = t }
        items.forEach { it.own = null; reset(it) }
    }

    private fun reset(it: Item) {
        if (it.status == Status.WORKING) return
        it.status = Status.WAITING
        it.progress = 0f
        it.outputs = emptyList()
        it.already = false
        it.error = null
    }

    val pending: Int get() = items.count { it.status == Status.WAITING || it.status == Status.FAILED }

    fun remove(id: Int) {
        val it = items.firstOrNull { x -> x.id == id } ?: return
        if (it.status == Status.WORKING) return
        items.remove(it)
        if (items.none { x -> x.kind == it.kind }) groupTargets.remove(it.kind)
    }

    fun clear() {
        if (running) return
        items.clear()
        groupTargets.clear()
        freshOpen = false
    }

    // Bringing files in.

    fun add(uris: List<Uri>) {
        if (uris.isEmpty()) return
        importing = true
        viewModelScope.launch {
            val before = kinds.toSet()
            var skipped = 0
            val added = ArrayList<Item>()
            withContext(Dispatchers.IO) {
                for ((n, uri) in uris.withIndex()) {
                    val name = realName(uri, displayName(uri) ?: "file-${n + 1}")
                    val source = Formats.detect(name, Build.VERSION.SDK_INT) ?: sniff(uri, name)
                    android.util.Log.i("Formigo", "import $uri name=$name type=${ctx.contentResolver.getType(uri)} source=${source?.id}")
                    if (source == null) { skipped++; continue }
                    val id = ids.incrementAndGet()
                    val dir = File(ctx.cacheDir, "imports/$id").apply { mkdirs() }
                    val fixed = if (Formats.detect(name) == null) "$name.${source.id}" else name
                    val f = File(dir, fixed.replace('/', '_'))
                    try {
                        ctx.contentResolver.openInputStream(uri)?.use { i -> f.outputStream().use { i.copyTo(it) } } ?: run { skipped++; null }
                            ?: continue
                    } catch (e: Exception) {
                        skipped++
                        continue
                    }
                    added.add(Item(id, fixed, f, source, f.length(), added.size))
                }
            }
            importing = false
            if (skipped > 0) toast(if (added.isEmpty()) "这些文件的格式暂不支持" else "已跳过 $skipped 个不支持的文件")
            if (added.isEmpty()) return@launch
            for (it in added) if (it.kind !in groupTargets) groupTargets[it.kind] = prefs.targets[it.kind] ?: Formats.defaultTarget(it.kind)
            items.addAll(added)
            val fresh = added.map { it.kind }.distinct().filter { it !in before }
            if (fresh.isNotEmpty()) {
                freshKinds = Kind.entries.filter { it in fresh }
                freshOpen = true
            }
            added.forEach { item ->
                launch(Dispatchers.IO) { item.meta = engine.describe(item.file, item.source) }
            }
        }
    }

    private fun displayName(uri: Uri): String? = try {
        ctx.contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { c ->
            if (c.moveToFirst()) c.getString(0) else null
        } ?: uri.lastPathSegment
    } catch (_: Exception) {
        uri.lastPathSegment
    }

    /**
     * The name a file goes by, its extension the one its type says for
     * pictures, video and sound: the photo picker names everything "<id>.jpg".
     */
    private fun realName(uri: Uri, name: String): String {
        val mime = ctx.contentResolver.getType(uri) ?: return name
        if (!(mime.startsWith("image/") || mime.startsWith("video/") || mime.startsWith("audio/"))) return name
        val ext = mediaExts[mime] ?: MimeTypeMap.getSingleton().getExtensionFromMimeType(mime) ?: return name
        val want = Formats.detect("x.$ext") ?: return name
        if (Formats.detect(name)?.id == want.id) return name
        val base = name.substringBeforeLast('.', name)
        return "$base.$ext"
    }

    private val mediaExts = mapOf(
        "image/jpeg" to "jpg", "image/png" to "png", "image/webp" to "webp", "image/gif" to "gif", "image/bmp" to "bmp",
        "image/heic" to "heic", "image/heif" to "heic", "image/avif" to "avif", "image/x-icon" to "ico",
        "video/mp4" to "mp4", "video/quicktime" to "mov", "video/3gpp" to "3gp", "video/webm" to "webm", "video/x-matroska" to "mkv",
        "audio/mpeg" to "mp3", "audio/mp4" to "m4a", "audio/aac" to "aac", "audio/x-wav" to "wav", "audio/wav" to "wav",
        "audio/flac" to "flac", "audio/ogg" to "ogg", "audio/amr" to "amr",
    )

    /** A file without a known extension, by the type its provider gives. */
    private fun sniff(uri: Uri, name: String): Source? {
        val mime = ctx.contentResolver.getType(uri) ?: return null
        val ext = MimeTypeMap.getSingleton().getExtensionFromMimeType(mime) ?: return null
        return Formats.detect("$name.$ext", Build.VERSION.SDK_INT)
    }

    // Converting.

    fun start() {
        if (running) return
        val todo = items.filter { it.status == Status.WAITING || it.status == Status.FAILED }
        if (todo.isEmpty()) return
        running = true
        runIds = todo.map { it.id }.toSet()
        freshOpen = false
        val outDir = File(ctx.cacheDir, "out/" + SimpleDateFormat("yyyyMMdd-HHmmss", Locale.US).format(Date()))
        val p = prefs
        viewModelScope.launch {
            todo.forEach { it.status = Status.WAITING; it.progress = 0f; it.error = null }
            val (zips, rest) = todo.partition { targetOf(it) == "zip" }
            val gate = Semaphore(p.workers)
            val jobs = rest.map { item ->
                async {
                    gate.withPermit { one(item, outDir, p) }
                }
            }.toMutableList()
            if (zips.isNotEmpty()) jobs.add(async { gate.withPermit { packed(zips, outDir, p) } })
            jobs.awaitAll()
            running = false
            cheers++
            val ok = todo.count { it.status == Status.DONE }
            val failed = todo.count { it.status == Status.FAILED }
            if (p.notify) notifyDone(ok, failed)
        }
    }

    private suspend fun one(item: Item, outDir: File, p: Prefs) {
        item.status = Status.WORKING
        val target = targetOf(item)
        val job = Job(item.file, item.source, target, p.options, outDir, item.file.nameWithoutExtension + p.suffix)
        try {
            val r = engine.convert(job) { v -> item.progress = v }
            item.outputs = r.files
            item.already = r.already
            item.progress = 1f
            item.status = Status.DONE
            item.doneRun++
        } catch (e: CancellationException) {
            throw e
        } catch (e: ConvertException) {
            fail(item, e.message ?: "转换失败")
        } catch (e: OutOfMemoryError) {
            fail(item, "文件太大，内存不足")
        } catch (e: Exception) {
            fail(item, e.message ?: "转换失败")
        }
    }

    private fun fail(item: Item, msg: String) {
        item.error = msg
        item.status = Status.FAILED
        item.failRun++
    }

    /** Files going to ZIP in one run go into one archive. */
    private suspend fun packed(zips: List<Item>, outDir: File, p: Prefs) {
        zips.forEach { it.status = Status.WORKING }
        try {
            val out = withContext(Dispatchers.IO) {
                outDir.mkdirs()
                val base = if (zips.size == 1) zips[0].file.nameWithoutExtension + p.suffix
                else "Formigo-" + SimpleDateFormat("yyyyMMdd-HHmm", Locale.US).format(Date()) + p.suffix
                val f = Engine.unique(outDir, base, "zip")
                Archive.zip(zips.map { it.file to it.name }, f) { v -> zips.forEach { z -> z.progress = v } }
                f
            }
            zips.forEach { it.outputs = listOf(out); it.status = Status.DONE; it.progress = 1f; it.doneRun++ }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            zips.forEach { fail(it, e.message ?: "打包失败") }
        }
    }

    // What became of them.

    private fun uriOf(f: File): Uri = FileProvider.getUriForFile(ctx, "${ctx.packageName}.files", f)

    private fun mime(f: File): String =
        MimeTypeMap.getSingleton().getMimeTypeFromExtension(f.extension.lowercase()) ?: when (f.extension.lowercase()) {
            "md" -> "text/markdown"
            "toml", "yaml", "srt", "vtt", "ass", "lrc", "tsv" -> "text/plain"
            else -> "application/octet-stream"
        }

    /** An intent sharing [files], or null when there are none. */
    fun shareIntent(files: List<File>): Intent? {
        val list = files.distinct().filter { it.exists() }
        if (list.isEmpty()) return null
        val uris = ArrayList(list.map { uriOf(it) })
        val types = list.map { mime(it) }.distinct()
        val type = when {
            types.size == 1 -> types[0]
            types.all { it.startsWith("image/") } -> "image/*"
            types.all { it.startsWith("video/") } -> "video/*"
            else -> "*/*"
        }
        val send = if (uris.size == 1) {
            Intent(Intent.ACTION_SEND).putExtra(Intent.EXTRA_STREAM, uris[0])
        } else {
            Intent(Intent.ACTION_SEND_MULTIPLE).putParcelableArrayListExtra(Intent.EXTRA_STREAM, uris)
        }
        send.type = type
        send.clipData = ClipData.newRawUri(null, uris[0]).apply { uris.drop(1).forEach { addItem(ClipData.Item(it)) } }
        send.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        return Intent.createChooser(send, null)
    }

    val doneFiles: List<File> get() = items.filter { it.status == Status.DONE }.flatMap { it.outputs }.distinct()

    /** Pictures and videos among what was converted, which the library takes. */
    val galleryFiles: List<File>
        get() = doneFiles.filter { mime(it).startsWith("image/") && it.extension.lowercase() != "ico" || mime(it).startsWith("video/") }

    fun saveToGallery() {
        val files = galleryFiles
        if (files.isEmpty()) return
        viewModelScope.launch {
            val n = withContext(Dispatchers.IO) { files.count { saveOne(it) } }
            toast(if (n == files.size) "已存入相册 $n 个文件" else "已存入 $n 个，${files.size - n} 个未能保存")
        }
    }

    private fun saveOne(f: File): Boolean = try {
        val m = mime(f)
        val video = m.startsWith("video/")
        val collection = if (video) MediaStore.Video.Media.EXTERNAL_CONTENT_URI else MediaStore.Images.Media.EXTERNAL_CONTENT_URI
        val v = ContentValues().apply {
            put(MediaStore.MediaColumns.DISPLAY_NAME, f.name)
            put(MediaStore.MediaColumns.MIME_TYPE, m)
            if (Build.VERSION.SDK_INT >= 29) {
                put(MediaStore.MediaColumns.RELATIVE_PATH, (if (video) Environment.DIRECTORY_MOVIES else Environment.DIRECTORY_PICTURES) + "/Formigo")
                put(MediaStore.MediaColumns.IS_PENDING, 1)
            }
        }
        val r = ctx.contentResolver
        val uri = r.insert(collection, v) ?: throw ConvertException("无法写入相册")
        r.openOutputStream(uri)?.use { o -> f.inputStream().use { it.copyTo(o) } }
        if (Build.VERSION.SDK_INT >= 29) {
            v.clear()
            v.put(MediaStore.MediaColumns.IS_PENDING, 0)
            r.update(uri, v, null, null)
        }
        true
    } catch (_: Exception) {
        false
    }

    fun toast(s: String) = Toast.makeText(ctx, s, Toast.LENGTH_SHORT).show()

    private fun notifyDone(ok: Int, failed: Int) {
        if (Build.VERSION.SDK_INT >= 33 &&
            ContextCompat.checkSelfPermission(ctx, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) return
        val nm = ctx.getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(NotificationChannel("done", "转换完成", NotificationManager.IMPORTANCE_DEFAULT))
        val open = PendingIntent.getActivity(
            ctx, 0, Intent(ctx, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
            PendingIntent.FLAG_IMMUTABLE,
        )
        val text = if (failed == 0) "已转换 $ok 个文件" else "已转换 $ok 个，$failed 个失败"
        val n = NotificationCompat.Builder(ctx, "done")
            .setSmallIcon(R.drawable.ic_stat)
            .setContentTitle("转换完成")
            .setContentText(text)
            .setContentIntent(open)
            .setAutoCancel(true)
            .build()
        try {
            NotificationManagerCompat.from(ctx).notify(1, n)
        } catch (_: SecurityException) {
        }
    }
}
