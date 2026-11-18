package dev.formigo.app.convert

import java.io.File
import java.util.zip.ZipEntry
import java.util.zip.ZipInputStream
import java.util.zip.ZipOutputStream

/** A conversion that could not be done, its message for the row. */
class ConvertException(message: String) : Exception(message)

object Archive {
    /** Packs [files] (each with the name it takes in the archive) into [out]. */
    fun zip(files: List<Pair<File, String>>, out: File, progress: (Float) -> Unit = {}) {
        val total = files.sumOf { it.first.length() }.coerceAtLeast(1)
        var done = 0L
        val used = HashSet<String>()
        ZipOutputStream(out.outputStream().buffered()).use { z ->
            for ((f, name) in files) {
                var entry = name
                var n = 1
                while (!used.add(entry.lowercase())) {
                    val dot = name.lastIndexOf('.')
                    entry = if (dot > 0) "${name.substring(0, dot)} ($n)${name.substring(dot)}" else "$name ($n)"
                    n++
                }
                z.putNextEntry(ZipEntry(entry).apply { time = f.lastModified() })
                f.inputStream().use { input ->
                    val buf = ByteArray(64 * 1024)
                    while (true) {
                        val r = input.read(buf)
                        if (r < 0) break
                        z.write(buf, 0, r)
                        done += r
                        progress(done.toFloat() / total)
                    }
                }
                z.closeEntry()
            }
        }
    }

    /** Unpacks [zip] into [dir], keeping its folders; returns the files written. */
    fun unzip(zip: File, dir: File, progress: (Float) -> Unit = {}): List<File> {
        val out = ArrayList<File>()
        val root = dir.canonicalFile
        val total = zip.length().coerceAtLeast(1)
        var read = 0L
        ZipInputStream(zip.inputStream().buffered()).use { z ->
            while (true) {
                val e = z.nextEntry ?: break
                val name = e.name.replace('\\', '/')
                if (name.startsWith("__MACOSX/") || name.substringAfterLast('/') == ".DS_Store") continue
                val f = File(root, name).canonicalFile
                // Entries naming a place outside the folder are not written.
                if (!f.path.startsWith(root.path + File.separator)) continue
                if (e.isDirectory) { f.mkdirs(); continue }
                f.parentFile?.mkdirs()
                f.outputStream().use { o ->
                    val buf = ByteArray(64 * 1024)
                    while (true) {
                        val r = z.read(buf)
                        if (r < 0) break
                        o.write(buf, 0, r)
                        read += r
                    }
                }
                progress((e.compressedSize.coerceAtLeast(0) + read).toFloat() / (total + read))
                out.add(f)
            }
        }
        if (out.isEmpty()) throw ConvertException("压缩包里没有文件")
        return out
    }
}
