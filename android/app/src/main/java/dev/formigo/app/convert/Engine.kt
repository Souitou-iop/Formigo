package dev.formigo.app.convert

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.ImageDecoder
import android.graphics.Paint
import android.graphics.pdf.PdfDocument
import android.graphics.pdf.PdfRenderer
import android.media.MediaCodec
import android.media.MediaExtractor
import android.media.MediaFormat
import android.media.MediaMetadataRetriever
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.ParcelFileDescriptor
import android.text.Layout
import android.text.StaticLayout
import android.text.TextPaint
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.audio.AudioProcessor
import androidx.media3.common.audio.ChannelMixingAudioProcessor
import androidx.media3.common.audio.ChannelMixingMatrix
import androidx.media3.common.Effect
import androidx.media3.effect.Presentation
import androidx.media3.transformer.Composition
import androidx.media3.transformer.EditedMediaItem
import androidx.media3.transformer.Effects
import androidx.media3.transformer.ExportException
import androidx.media3.transformer.ExportResult
import androidx.media3.transformer.ProgressHolder
import androidx.media3.transformer.Transformer
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.RandomAccessFile
import java.nio.ByteBuffer
import java.nio.ByteOrder
import kotlin.coroutines.coroutineContext
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException
import kotlin.math.roundToInt

/** What a file becomes: the files written, or that it already was the format. */
data class Result(val files: List<File>, val already: Boolean = false)

/** One file to convert. */
data class Job(
    val src: File,
    val source: Source,
    val target: String,
    val options: Options,
    val outDir: File,
    /** The name written, without its extension, the suffix already in it. */
    val baseName: String,
)

/** Converts files on this device, with Android's own decoders and encoders. */
class Engine(private val context: Context) {

    suspend fun convert(job: Job, progress: (Float) -> Unit): Result {
        val to = Formats.format(job.target) ?: throw ConvertException("未知的目标格式")
        if (job.target == job.source.same) return Result(listOf(job.src), already = true)
        job.outDir.mkdirs()
        return when {
            job.target == "zip" -> withContext(Dispatchers.IO) {
                val out = unique(job.outDir, job.baseName, "zip")
                Archive.zip(listOf(job.src to job.src.name), out, progress)
                Result(listOf(out))
            }
            job.target == "unzip" -> withContext(Dispatchers.IO) {
                val dir = File(job.outDir, job.baseName).let { if (it.exists()) File(job.outDir, "${job.baseName}-${System.nanoTime()}") else it }
                Result(Archive.unzip(job.src, dir, progress))
            }
            job.source.kind == Kind.IMAGE -> withContext(Dispatchers.Default) { Result(listOf(image(job))) }
            job.source.id == "pdf" -> withContext(Dispatchers.Default) { Result(pdfPages(job, progress)) }
            job.source.kind == Kind.DOC && job.target == "pdf" -> withContext(Dispatchers.Default) { Result(listOf(textPdf(job))) }
            job.source.kind == Kind.VIDEO || job.source.kind == Kind.AUDIO -> Result(listOf(media(job, to, progress)))
            else -> withContext(Dispatchers.IO) {
                val text = readText(job.src)
                val out = when (job.source.kind) {
                    Kind.DATA -> DataConvert.convert(text, job.source.id, job.target, job.options)
                    Kind.DOC -> DocConvert.convert(text, job.source.id, job.target, titleOf(text, job.source.id, job.src))
                    Kind.SUBTITLE -> Subtitles.convert(text, job.source.id, job.target)
                    else -> throw ConvertException("不支持的转换")
                }
                val f = unique(job.outDir, job.baseName, to.ext)
                f.writeText(out)
                Result(listOf(f))
            }
        }
    }

    // Pictures.

    private fun decode(src: File, scale: Float): Bitmap {
        try {
            val source = ImageDecoder.createSource(src)
            return ImageDecoder.decodeBitmap(source) { d, info, _ ->
                d.allocator = ImageDecoder.ALLOCATOR_SOFTWARE
                d.isMutableRequired = false
                val w = (info.size.width * scale).roundToInt().coerceAtLeast(1)
                val h = (info.size.height * scale).roundToInt().coerceAtLeast(1)
                if (scale != 1f) d.setTargetSize(w, h)
            }
        } catch (e: Exception) {
            // ICO and the odd file ImageDecoder refuses: BitmapFactory.
            val b = BitmapFactory.decodeFile(src.path) ?: throw ConvertException("无法读取这张图片")
            if (scale == 1f) return b
            return Bitmap.createScaledBitmap(b, (b.width * scale).roundToInt().coerceAtLeast(1), (b.height * scale).roundToInt().coerceAtLeast(1), true)
        }
    }

    private fun pixels(b: Bitmap): Pixels {
        val px = IntArray(b.width * b.height)
        b.getPixels(px, 0, b.width, 0, 0, b.width, b.height)
        return Pixels(b.width, b.height, px)
    }

    private fun flattened(b: Bitmap): Bitmap {
        if (!b.hasAlpha()) return b
        val out = Bitmap.createBitmap(b.width, b.height, Bitmap.Config.ARGB_8888)
        Canvas(out).apply { drawColor(Color.WHITE); drawBitmap(b, 0f, 0f, null) }
        return out
    }

    private fun image(job: Job): File {
        val o = job.options
        val to = Formats.format(job.target)!!
        val bmp = decode(job.src, o.scaleFactor)
        val out = unique(job.outDir, job.baseName, to.ext)
        out.outputStream().buffered().use { s ->
            when (job.target) {
                "jpg" -> flattened(bmp).compress(Bitmap.CompressFormat.JPEG, o.quality, s)
                "png" -> bmp.compress(Bitmap.CompressFormat.PNG, 100, s)
                "webp" -> {
                    @Suppress("DEPRECATION")
                    val f = when {
                        Build.VERSION.SDK_INT < 30 -> Bitmap.CompressFormat.WEBP
                        o.quality >= 100 -> Bitmap.CompressFormat.WEBP_LOSSLESS
                        else -> Bitmap.CompressFormat.WEBP_LOSSY
                    }
                    bmp.compress(f, o.quality, s)
                }
                "gif" -> Encoders.gif(pixels(bmp), s)
                "bmp" -> Encoders.bmp(pixels(bmp), s)
                "ico" -> {
                    val side = maxOf(bmp.width, bmp.height)
                    val pngs = Encoders.icoSizes(side).map { size ->
                        val sq = Bitmap.createBitmap(size, size, Bitmap.Config.ARGB_8888)
                        val k = size.toFloat() / side
                        val w = bmp.width * k
                        val h = bmp.height * k
                        Canvas(sq).drawBitmap(
                            bmp, null,
                            android.graphics.RectF((size - w) / 2, (size - h) / 2, (size + w) / 2, (size + h) / 2),
                            Paint(Paint.FILTER_BITMAP_FLAG or Paint.ANTI_ALIAS_FLAG),
                        )
                        size to ByteArrayOutputStream().also { sq.compress(Bitmap.CompressFormat.PNG, 100, it) }.toByteArray()
                    }
                    Encoders.ico(pngs, s)
                }
                "pdf" -> {
                    val doc = PdfDocument()
                    // At 144 dpi: a photo's page as large as a print of it.
                    val pw = (bmp.width / 2).coerceIn(1, 14400)
                    val ph = (bmp.height / 2).coerceIn(1, 14400)
                    val page = doc.startPage(PdfDocument.PageInfo.Builder(pw, ph, 1).create())
                    page.canvas.drawBitmap(bmp, null, android.graphics.RectF(0f, 0f, pw.toFloat(), ph.toFloat()), Paint(Paint.FILTER_BITMAP_FLAG))
                    doc.finishPage(page)
                    doc.writeTo(s)
                    doc.close()
                }
                else -> throw ConvertException("不支持的图片格式")
            }
        }
        return out
    }

    // PDF.

    private suspend fun pdfPages(job: Job, progress: (Float) -> Unit): List<File> {
        val to = Formats.format(job.target)!!
        val out = ArrayList<File>()
        ParcelFileDescriptor.open(job.src, ParcelFileDescriptor.MODE_READ_ONLY).use { fd ->
            val r = try {
                PdfRenderer(fd)
            } catch (e: SecurityException) {
                throw ConvertException("PDF 已加密")
            } catch (e: Exception) {
                throw ConvertException("无法读取这个 PDF")
            }
            r.use {
                val n = r.pageCount
                for (i in 0 until n) {
                    coroutineContext.ensureActive()
                    r.openPage(i).use { page ->
                        var k = job.options.pdfFactor
                        val longest = maxOf(page.width, page.height) * k
                        if (longest > 8000) k *= 8000 / longest
                        val w = (page.width * k).roundToInt().coerceAtLeast(1)
                        val h = (page.height * k).roundToInt().coerceAtLeast(1)
                        val bmp = Bitmap.createBitmap(w, h, Bitmap.Config.ARGB_8888)
                        bmp.eraseColor(Color.WHITE)
                        page.render(bmp, null, null, PdfRenderer.Page.RENDER_MODE_FOR_DISPLAY)
                        val name = if (n == 1) job.baseName else "${job.baseName}-${i + 1}"
                        val f = unique(job.outDir, name, to.ext)
                        f.outputStream().buffered().use { s ->
                            if (job.target == "jpg") bmp.compress(Bitmap.CompressFormat.JPEG, job.options.quality, s)
                            else bmp.compress(Bitmap.CompressFormat.PNG, 100, s)
                        }
                        bmp.recycle()
                        out.add(f)
                    }
                    progress((i + 1f) / n)
                }
            }
        }
        if (out.isEmpty()) throw ConvertException("PDF 没有页面")
        return out
    }

    /** A text document set on A4 pages. */
    private fun textPdf(job: Job): File {
        val text = DocConvert.plainText(readText(job.src), job.source.id)
        val doc = PdfDocument()
        val pw = 595
        val ph = 842
        val margin = 56f
        val paint = TextPaint(Paint.ANTI_ALIAS_FLAG).apply { textSize = 11f; color = Color.rgb(20, 33, 61) }
        val width = (pw - 2 * margin).toInt()
        val layout = StaticLayout.Builder.obtain(text, 0, text.length, paint, width)
            .setAlignment(Layout.Alignment.ALIGN_NORMAL).setLineSpacing(0f, 1.35f).build()
        val usable = ph - 2 * margin
        var line = 0
        var pageNo = 1
        while (line < layout.lineCount || pageNo == 1) {
            val top = if (line < layout.lineCount) layout.getLineTop(line) else 0
            var last = line
            while (last < layout.lineCount && layout.getLineBottom(last) - top <= usable) last++
            if (last == line && line < layout.lineCount) last = line + 1
            val page = doc.startPage(PdfDocument.PageInfo.Builder(pw, ph, pageNo).create())
            val c = page.canvas
            c.save()
            c.translate(margin, margin - top)
            val bottom = if (last > 0 && layout.lineCount > 0) layout.getLineBottom(last - 1) else 0
            c.clipRect(0f, top.toFloat(), width.toFloat(), bottom.toFloat())
            layout.draw(c)
            c.restore()
            doc.finishPage(page)
            pageNo++
            line = last
            if (layout.lineCount == 0) break
        }
        val out = unique(job.outDir, job.baseName, "pdf")
        out.outputStream().buffered().use { doc.writeTo(it) }
        doc.close()
        return out
    }

    // Sound and video.

    private suspend fun media(job: Job, to: Format, progress: (Float) -> Unit): File {
        val out = unique(job.outDir, job.baseName, to.ext)
        when (job.target) {
            "mp4", "hevc" -> transform(job, out, audioOnly = false, progress)
            "m4a" -> transform(job, out, audioOnly = true, progress)
            "aac" -> {
                val tmp = File(job.outDir, ".${System.nanoTime()}.m4a")
                try {
                    transform(job, tmp, audioOnly = true) { progress(it * 0.95f) }
                    withContext(Dispatchers.IO) { adts(tmp, out) }
                } finally {
                    tmp.delete()
                }
            }
            "wav" -> withContext(Dispatchers.IO) { wav(job.src, out, job.options, progress) }
            else -> throw ConvertException("不支持的转换")
        }
        return out
    }

    private suspend fun transform(job: Job, out: File, audioOnly: Boolean, progress: (Float) -> Unit) {
        val video = ArrayList<Effect>()
        val audio = ArrayList<AudioProcessor>()
        if (!audioOnly && job.options.videoHeight > 0) {
            val (w, h) = videoSize(job.src)
            if (minOf(w, h) > job.options.videoHeight) video.add(Presentation.createForShortSide(job.options.videoHeight))
        }
        if (job.options.mono) {
            audio.add(ChannelMixingAudioProcessor().apply {
                for (n in 1..8) putChannelMixingMatrix(ChannelMixingMatrix.createForConstantGain(n, 1))
            })
        }
        val item = EditedMediaItem.Builder(MediaItem.fromUri(Uri.fromFile(job.src)))
            .setRemoveVideo(audioOnly)
            .setEffects(Effects(audio, video))
            .build()
        withContext(Dispatchers.Main) {
            val handler = Handler(Looper.getMainLooper())
            suspendCancellableCoroutine { cont ->
                val transformer = Transformer.Builder(context)
                    .setVideoMimeType(if (job.target == "hevc") MimeTypes.VIDEO_H265 else MimeTypes.VIDEO_H264)
                    .setAudioMimeType(MimeTypes.AUDIO_AAC)
                    .addListener(object : Transformer.Listener {
                        override fun onCompleted(composition: Composition, exportResult: ExportResult) {
                            handler.removeCallbacksAndMessages(null)
                            if (cont.isActive) cont.resume(Unit)
                        }

                        override fun onError(composition: Composition, exportResult: ExportResult, exportException: ExportException) {
                            handler.removeCallbacksAndMessages(null)
                            out.delete()
                            if (cont.isActive) cont.resumeWithException(ConvertException(mediaError(exportException)))
                        }
                    })
                    .build()
                val holder = ProgressHolder()
                val poll = object : Runnable {
                    override fun run() {
                        if (transformer.getProgress(holder) == Transformer.PROGRESS_STATE_AVAILABLE) progress(holder.progress / 100f)
                        handler.postDelayed(this, 150)
                    }
                }
                try {
                    transformer.start(item, out.path)
                } catch (e: Exception) {
                    cont.resumeWithException(ConvertException("无法开始转换：${e.message}"))
                    return@suspendCancellableCoroutine
                }
                handler.postDelayed(poll, 150)
                cont.invokeOnCancellation {
                    handler.post {
                        handler.removeCallbacksAndMessages(null)
                        transformer.cancel()
                        out.delete()
                    }
                }
            }
        }
    }

    private fun mediaError(e: ExportException): String = when (e.errorCode) {
        ExportException.ERROR_CODE_DECODER_INIT_FAILED, ExportException.ERROR_CODE_DECODING_FORMAT_UNSUPPORTED -> "这台设备无法解码此文件"
        ExportException.ERROR_CODE_ENCODER_INIT_FAILED, ExportException.ERROR_CODE_ENCODING_FORMAT_UNSUPPORTED -> "这台设备不支持此编码"
        ExportException.ERROR_CODE_IO_FILE_NOT_FOUND, ExportException.ERROR_CODE_IO_UNSPECIFIED -> "无法读取文件"
        else -> "转换失败（${e.errorCodeName}）"
    }

    private fun videoSize(f: File): Pair<Int, Int> {
        val r = MediaMetadataRetriever()
        return try {
            r.setDataSource(f.path)
            val w = r.extractMetadata(MediaMetadataRetriever.METADATA_KEY_VIDEO_WIDTH)?.toIntOrNull() ?: 0
            val h = r.extractMetadata(MediaMetadataRetriever.METADATA_KEY_VIDEO_HEIGHT)?.toIntOrNull() ?: 0
            w to h
        } catch (_: Exception) {
            0 to 0
        } finally {
            r.release()
        }
    }

    /** The AAC track of an MP4 as ADTS frames. */
    private fun adts(m4a: File, out: File) {
        val ex = MediaExtractor()
        try {
            ex.setDataSource(m4a.path)
            val track = (0 until ex.trackCount).firstOrNull { ex.getTrackFormat(it).getString(MediaFormat.KEY_MIME) == MimeTypes.AUDIO_AAC }
                ?: throw ConvertException("没有 AAC 音轨")
            val fmt = ex.getTrackFormat(track)
            ex.selectTrack(track)
            val rate = fmt.getInteger(MediaFormat.KEY_SAMPLE_RATE)
            val channels = fmt.getInteger(MediaFormat.KEY_CHANNEL_COUNT)
            val rates = intArrayOf(96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350)
            val freq = rates.indexOf(rate).let { if (it < 0) 4 else it }
            val profile = 1 // AAC LC, as written by the encoder
            val buf = ByteBuffer.allocate(1 shl 16)
            out.outputStream().buffered().use { s ->
                while (true) {
                    val n = ex.readSampleData(buf, 0)
                    if (n < 0) break
                    val len = n + 7
                    s.write(
                        byteArrayOf(
                            0xff.toByte(), 0xf1.toByte(),
                            ((profile shl 6) or (freq shl 2) or (channels shr 2)).toByte(),
                            (((channels and 3) shl 6) or (len shr 11)).toByte(),
                            ((len shr 3) and 0xff).toByte(),
                            (((len and 7) shl 5) or 0x1f).toByte(),
                            0xfc.toByte(),
                        ),
                    )
                    val bytes = ByteArray(n)
                    buf.position(0)
                    buf.get(bytes, 0, n)
                    s.write(bytes)
                    buf.clear()
                    ex.advance()
                }
            }
        } finally {
            ex.release()
        }
    }

    /** Decodes the sound of [src] with the system's codecs into a WAV file. */
    private suspend fun wav(src: File, out: File, o: Options, progress: (Float) -> Unit) {
        val ex = MediaExtractor()
        var codec: MediaCodec? = null
        try {
            ex.setDataSource(src.path)
            val track = (0 until ex.trackCount).firstOrNull { ex.getTrackFormat(it).getString(MediaFormat.KEY_MIME)?.startsWith("audio/") == true }
                ?: throw ConvertException("文件里没有声音")
            val fmt = ex.getTrackFormat(track)
            ex.selectTrack(track)
            val duration = if (fmt.containsKey(MediaFormat.KEY_DURATION)) fmt.getLong(MediaFormat.KEY_DURATION) else 0L
            val c = MediaCodec.createDecoderByType(fmt.getString(MediaFormat.KEY_MIME)!!)
            codec = c
            c.configure(fmt, null, null, 0)
            c.start()
            var rate = fmt.getInteger(MediaFormat.KEY_SAMPLE_RATE)
            var channels = fmt.getInteger(MediaFormat.KEY_CHANNEL_COUNT)
            var encoding = 2 // AudioFormat.ENCODING_PCM_16BIT
            val outChannels = { if (o.mono) 1 else channels }
            val bytesOut = o.depth / 8
            RandomAccessFile(out, "rw").use { raf ->
                raf.setLength(0)
                raf.write(ByteArray(44))
                var dataLen = 0L
                val info = MediaCodec.BufferInfo()
                var inputDone = false
                var buf = ByteArray(0)
                while (true) {
                    coroutineContext.ensureActive()
                    if (!inputDone) {
                        val i = c.dequeueInputBuffer(10_000)
                        if (i >= 0) {
                            val ib = c.getInputBuffer(i)!!
                            val n = ex.readSampleData(ib, 0)
                            if (n < 0) {
                                c.queueInputBuffer(i, 0, 0, 0, MediaCodec.BUFFER_FLAG_END_OF_STREAM)
                                inputDone = true
                            } else {
                                c.queueInputBuffer(i, 0, n, ex.sampleTime, 0)
                                if (duration > 0) progress((ex.sampleTime.toFloat() / duration).coerceIn(0f, 0.99f))
                                ex.advance()
                            }
                        }
                    }
                    val oi = c.dequeueOutputBuffer(info, 10_000)
                    if (oi == MediaCodec.INFO_OUTPUT_FORMAT_CHANGED) {
                        val f = c.outputFormat
                        rate = f.getInteger(MediaFormat.KEY_SAMPLE_RATE)
                        channels = f.getInteger(MediaFormat.KEY_CHANNEL_COUNT)
                        if (f.containsKey(MediaFormat.KEY_PCM_ENCODING)) encoding = f.getInteger(MediaFormat.KEY_PCM_ENCODING)
                        continue
                    }
                    if (oi < 0) continue
                    val ob = c.getOutputBuffer(oi)!!
                    ob.position(info.offset).limit(info.offset + info.size)
                    val pcm = ob.slice().order(ByteOrder.LITTLE_ENDIAN)
                    val inBytes = if (encoding == 4) 4 else 2 // ENCODING_PCM_FLOAT, else 16-bit
                    val frames = info.size / (inBytes * channels)
                    val oc = outChannels()
                    val need = frames * oc * bytesOut
                    if (buf.size < need) buf = ByteArray(need)
                    var w = 0
                    for (fr in 0 until frames) {
                        var mix = 0.0
                        for (ch in 0 until channels) {
                            val v = if (encoding == 4) pcm.getFloat((fr * channels + ch) * 4).toDouble()
                            else pcm.getShort((fr * channels + ch) * 2) / 32768.0
                            if (oc == 1) mix += v else w = putSample(buf, w, v, bytesOut)
                        }
                        if (oc == 1) w = putSample(buf, w, mix / channels, bytesOut)
                    }
                    raf.write(buf, 0, w)
                    dataLen += w
                    c.releaseOutputBuffer(oi, false)
                    if (info.flags and MediaCodec.BUFFER_FLAG_END_OF_STREAM != 0) break
                }
                val oc = outChannels()
                raf.seek(0)
                raf.write(wavHeader(rate, oc, o.depth, dataLen))
            }
            progress(1f)
        } catch (e: ConvertException) {
            out.delete(); throw e
        } catch (e: kotlinx.coroutines.CancellationException) {
            out.delete(); throw e
        } catch (e: Exception) {
            out.delete(); throw ConvertException("这台设备无法解码此文件")
        } finally {
            try { codec?.stop() } catch (_: Exception) {}
            codec?.release()
            ex.release()
        }
    }

    private fun putSample(b: ByteArray, at: Int, v: Double, bytes: Int): Int {
        val x = v.coerceIn(-1.0, 1.0)
        return if (bytes == 2) {
            val s = (x * 32767).roundToInt()
            b[at] = s.toByte(); b[at + 1] = (s shr 8).toByte(); at + 2
        } else {
            val s = (x * 8388607).roundToInt()
            b[at] = s.toByte(); b[at + 1] = (s shr 8).toByte(); b[at + 2] = (s shr 16).toByte(); at + 3
        }
    }

    // Reading and naming.

    /** A short description of a file for its row: its size in pixels, its length, what it holds. */
    fun describe(src: File, source: Source): String? = try {
        when (source.kind) {
            Kind.IMAGE -> {
                val o = BitmapFactory.Options().apply { inJustDecodeBounds = true }
                BitmapFactory.decodeFile(src.path, o)
                if (o.outWidth > 0) "${o.outWidth}×${o.outHeight}" else imageSize(src)
            }
            Kind.VIDEO, Kind.AUDIO -> {
                val r = MediaMetadataRetriever()
                try {
                    r.setDataSource(src.path)
                    val ms = r.extractMetadata(MediaMetadataRetriever.METADATA_KEY_DURATION)?.toLongOrNull() ?: 0
                    val w = r.extractMetadata(MediaMetadataRetriever.METADATA_KEY_VIDEO_WIDTH)
                    val h = r.extractMetadata(MediaMetadataRetriever.METADATA_KEY_VIDEO_HEIGHT)
                    listOfNotNull(if (w != null && h != null && source.kind == Kind.VIDEO) "$w×$h" else null, duration(ms)).joinToString(" · ")
                } finally {
                    r.release()
                }
            }
            Kind.DATA -> DataConvert.summary(DataConvert.parse(source.id, readText(src)))
            Kind.SUBTITLE -> "${Subtitles.parse(source.id, readText(src)).size} 条"
            Kind.DOC -> if (source.id == "pdf") ParcelFileDescriptor.open(src, ParcelFileDescriptor.MODE_READ_ONLY).use { fd ->
                PdfRenderer(fd).use { "${it.pageCount} 页" }
            } else "${readText(src).length} 字"
            Kind.ARCHIVE -> java.util.zip.ZipFile(src).use { z -> "${z.entries().asSequence().count { !it.isDirectory }} 个文件" }
        }
    } catch (_: Exception) {
        null
    }

    private fun imageSize(src: File): String? = try {
        var s: String? = null
        ImageDecoder.decodeBitmap(ImageDecoder.createSource(src)) { d, info, _ ->
            s = "${info.size.width}×${info.size.height}"
            d.setTargetSize(1, 1)
        }
        s
    } catch (_: Exception) {
        null
    }

    companion object {
        fun duration(ms: Long): String {
            val s = ms / 1000
            return if (s >= 3600) "%d:%02d:%02d".format(s / 3600, s / 60 % 60, s % 60) else "%d:%02d".format(s / 60, s % 60)
        }

        fun wavHeader(rate: Int, channels: Int, bits: Int, dataLen: Long): ByteArray {
            val b = ByteBuffer.allocate(44).order(ByteOrder.LITTLE_ENDIAN)
            val block = channels * bits / 8
            b.put("RIFF".toByteArray()).putInt((36 + dataLen).toInt()).put("WAVE".toByteArray())
            b.put("fmt ".toByteArray()).putInt(16).putShort(1).putShort(channels.toShort()).putInt(rate)
                .putInt(rate * block).putShort(block.toShort()).putShort(bits.toShort())
            b.put("data".toByteArray()).putInt(dataLen.toInt())
            return b.array()
        }

        /** Text in UTF-8, or in GB18030 when it is not valid UTF-8, as Chinese files often are. */
        fun readText(f: File): String {
            val bytes = f.readBytes()
            val dec = Charsets.UTF_8.newDecoder()
            return try {
                dec.decode(ByteBuffer.wrap(bytes)).toString()
            } catch (_: Exception) {
                String(bytes, charset("GB18030"))
            }
        }

        fun titleOf(text: String, from: String, f: File): String {
            val t = when (from) {
                "md" -> Regex("^#\\s+(.+)$", RegexOption.MULTILINE).find(text)?.groupValues?.get(1)
                "html" -> Regex("<title[^>]*>(.*?)</title>", setOf(RegexOption.IGNORE_CASE, RegexOption.DOT_MATCHES_ALL)).find(text)?.groupValues?.get(1)
                else -> null
            }
            return t?.trim()?.takeIf { it.isNotEmpty() } ?: f.nameWithoutExtension
        }

        /** A path in [dir] named [base].[ext], numbered when taken. */
        fun unique(dir: File, base: String, ext: String): File {
            val clean = base.replace(Regex("[/\\\\:*?\"<>|]"), "_").ifBlank { "file" }
            fun name(n: Int) = (if (n == 0) clean else "$clean ($n)") + if (ext.isEmpty()) "" else ".$ext"
            var n = 0
            while (File(dir, name(n)).exists()) n++
            return File(dir, name(n))
        }
    }
}
