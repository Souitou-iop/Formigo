package dev.formigo.app.convert

import java.io.ByteArrayOutputStream
import java.io.OutputStream

/** Pixels as ARGB ints, row by row. */
class Pixels(val width: Int, val height: Int, val argb: IntArray) {
    init {
        require(argb.size == width * height)
    }

    val hasAlpha: Boolean get() = argb.any { (it ushr 24) != 0xff }
}

/** Encoders Android's Bitmap does not have: GIF, BMP and ICO. */
object Encoders {
    // GIF: a median cut palette of up to 255 colors, and a transparent index
    // when the picture has any, LZW coded.

    fun gif(p: Pixels, out: OutputStream) {
        val transparent = p.argb.any { (it ushr 24) < 128 }
        val palette = medianCut(p.argb, if (transparent) 255 else 256)
        val lookup = IntArray(32768) { -1 }
        val index = ByteArray(p.argb.size)
        val tIndex = palette.size
        for (i in p.argb.indices) {
            val c = p.argb[i]
            if (transparent && (c ushr 24) < 128) { index[i] = tIndex.toByte(); continue }
            val key = ((c shr 9) and 0x7c00) or ((c shr 6) and 0x3e0) or ((c shr 3) and 0x1f)
            var n = lookup[key]
            if (n < 0) { n = nearest(palette, c); lookup[key] = n }
            index[i] = n.toByte()
        }
        val colors = palette.size + if (transparent) 1 else 0
        var bits = 1
        while ((1 shl bits) < colors) bits++
        val size = 1 shl bits

        val b = ByteArrayOutputStream()
        b.write("GIF89a".toByteArray())
        le16(b, p.width); le16(b, p.height)
        b.write(0x80 or ((bits - 1) shl 4) or (bits - 1))
        b.write(0); b.write(0)
        for (i in 0 until size) {
            val c = palette.getOrElse(i) { 0 }
            b.write((c shr 16) and 0xff); b.write((c shr 8) and 0xff); b.write(c and 0xff)
        }
        if (transparent) {
            b.write(byteArrayOf(0x21, 0xf9.toByte(), 4, 1, 0, 0, tIndex.toByte(), 0))
        }
        b.write(0x2c); le16(b, 0); le16(b, 0); le16(b, p.width); le16(b, p.height); b.write(0)
        lzw(index, maxOf(2, bits), b)
        b.write(0x3b)
        out.write(b.toByteArray())
    }

    private fun le16(b: OutputStream, v: Int) { b.write(v and 0xff); b.write((v shr 8) and 0xff) }

    private fun le32(b: OutputStream, v: Int) { le16(b, v and 0xffff); le16(b, (v ushr 16) and 0xffff) }

    private fun nearest(palette: IntArray, c: Int): Int {
        val r = (c shr 16) and 0xff; val g = (c shr 8) and 0xff; val bl = c and 0xff
        var best = 0
        var bestD = Int.MAX_VALUE
        for (i in palette.indices) {
            val p = palette[i]
            val dr = r - ((p shr 16) and 0xff); val dg = g - ((p shr 8) and 0xff); val db = bl - (p and 0xff)
            val d = 2 * dr * dr + 4 * dg * dg + 3 * db * db
            if (d < bestD) { bestD = d; best = i }
        }
        return best
    }

    /** A palette of up to [n] colors, by median cut over a 15-bit histogram. */
    fun medianCut(argb: IntArray, n: Int): IntArray {
        val hist = IntArray(32768)
        // The true colors of each bucket, summed, for the boxes' means.
        val sums = LongArray(32768 * 3)
        for (c in argb) {
            if ((c ushr 24) < 128) continue
            val k = ((c shr 9) and 0x7c00) or ((c shr 6) and 0x3e0) or ((c shr 3) and 0x1f)
            hist[k]++
            sums[k * 3] += ((c shr 16) and 0xff).toLong(); sums[k * 3 + 1] += ((c shr 8) and 0xff).toLong(); sums[k * 3 + 2] += (c and 0xff).toLong()
        }
        val keys = (0 until 32768).filter { hist[it] > 0 }.toIntArray()
        if (keys.isEmpty()) return intArrayOf(0)
        fun mean(box: IntArray): Int {
            var r = 0L; var g = 0L; var b = 0L; var w = 0L
            for (k in box) { r += sums[k * 3]; g += sums[k * 3 + 1]; b += sums[k * 3 + 2]; w += hist[k] }
            return (0xff shl 24) or ((r / w).toInt() shl 16) or ((g / w).toInt() shl 8) or (b / w).toInt()
        }
        if (keys.size <= n) return IntArray(keys.size) { mean(intArrayOf(keys[it])) }
        val boxes = ArrayList<IntArray>().apply { add(keys) }
        while (boxes.size < n) {
            // Split the box with the widest channel, weighted by its pixels.
            var pick = -1
            var pickScore = 0L
            var pickCh = 0
            for ((bi, box) in boxes.withIndex()) {
                if (box.size < 2) continue
                for (ch in 0..2) {
                    var lo = 31; var hi = 0; var count = 0L
                    for (k in box) { val v = (k shr (10 - 5 * ch)) and 31; if (v < lo) lo = v; if (v > hi) hi = v; count += hist[k] }
                    val score = (hi - lo).toLong() * count
                    if (score > pickScore) { pickScore = score; pick = bi; pickCh = ch }
                }
            }
            if (pick < 0) break
            val box = boxes.removeAt(pick)
            val shift = 10 - 5 * pickCh
            val sorted = box.sortedBy { (it shr shift) and 31 }
            val total = sorted.sumOf { hist[it].toLong() }
            var acc = 0L
            var cut = 1
            for ((i, k) in sorted.withIndex()) {
                acc += hist[k]
                if (acc * 2 >= total) { cut = (i + 1).coerceIn(1, sorted.size - 1); break }
            }
            boxes.add(sorted.subList(0, cut).toIntArray())
            boxes.add(sorted.subList(cut, sorted.size).toIntArray())
        }
        return IntArray(boxes.size) { mean(boxes[it]) }
    }

    private fun lzw(data: ByteArray, minCode: Int, out: OutputStream) {
        out.write(minCode)
        val clear = 1 shl minCode
        val eoi = clear + 1
        val block = ByteArrayOutputStream()
        var acc = 0
        var nbits = 0
        var codeSize = minCode + 1
        fun emit(code: Int) {
            acc = acc or (code shl nbits)
            nbits += codeSize
            while (nbits >= 8) {
                block.write(acc and 0xff)
                acc = acc ushr 8
                nbits -= 8
            }
        }
        val dict = HashMap<Int, Int>(8192)
        var next = eoi + 1
        emit(clear)
        if (data.isNotEmpty()) {
            var prefix = data[0].toInt() and 0xff
            for (i in 1 until data.size) {
                val c = data[i].toInt() and 0xff
                val key = (prefix shl 8) or c
                val found = dict[key]
                if (found != null) { prefix = found; continue }
                emit(prefix)
                if (next < 4096) {
                    dict[key] = next++
                    if (next > (1 shl codeSize) && codeSize < 12) codeSize++
                } else {
                    emit(clear)
                    dict.clear()
                    next = eoi + 1
                    codeSize = minCode + 1
                }
                prefix = c
            }
            emit(prefix)
        }
        emit(eoi)
        if (nbits > 0) block.write(acc and 0xff)
        val bytes = block.toByteArray()
        var i = 0
        while (i < bytes.size) {
            val n = minOf(255, bytes.size - i)
            out.write(n)
            out.write(bytes, i, n)
            i += n
        }
        out.write(0)
    }

    /** A 24-bit BMP, or 32-bit with its alpha when the picture has any. */
    fun bmp(p: Pixels, out: OutputStream) {
        val alpha = p.hasAlpha
        val bpp = if (alpha) 4 else 3
        val stride = (p.width * bpp + 3) and 3.inv()
        val header = if (alpha) 108 else 40
        val dataSize = stride * p.height
        val b = ByteArrayOutputStream(14 + header + dataSize)
        b.write('B'.code); b.write('M'.code)
        le32(b, 14 + header + dataSize); le32(b, 0); le32(b, 14 + header)
        le32(b, header); le32(b, p.width); le32(b, p.height); le16(b, 1); le16(b, bpp * 8)
        le32(b, if (alpha) 3 else 0); le32(b, dataSize); le32(b, 2835); le32(b, 2835); le32(b, 0); le32(b, 0)
        if (alpha) {
            le32(b, 0x00ff0000); le32(b, 0x0000ff00); le32(b, 0x000000ff); le32(b, 0xff000000.toInt())
            le32(b, 0x57696e20) // "Win " color space
            repeat(12) { le32(b, 0) }
        }
        val row = ByteArray(stride)
        for (y in p.height - 1 downTo 0) {
            var o = 0
            for (x in 0 until p.width) {
                val c = p.argb[y * p.width + x]
                row[o++] = c.toByte(); row[o++] = (c shr 8).toByte(); row[o++] = (c shr 16).toByte()
                if (alpha) row[o++] = (c ushr 24).toByte()
            }
            b.write(row)
        }
        out.write(b.toByteArray())
    }

    /** An ICO holding PNG images, each a size of [pngs], 256 at most. */
    fun ico(pngs: List<Pair<Int, ByteArray>>, out: OutputStream) {
        val b = ByteArrayOutputStream()
        le16(b, 0); le16(b, 1); le16(b, pngs.size)
        var offset = 6 + 16 * pngs.size
        for ((size, data) in pngs) {
            b.write(if (size >= 256) 0 else size); b.write(if (size >= 256) 0 else size)
            b.write(0); b.write(0); le16(b, 1); le16(b, 32); le32(b, data.size); le32(b, offset)
            offset += data.size
        }
        for ((_, data) in pngs) b.write(data)
        out.write(b.toByteArray())
    }

    /** The sizes an icon is written at for a picture of the given side. */
    fun icoSizes(side: Int): List<Int> = listOf(256, 128, 64, 48, 32, 16).filter { it <= maxOf(16, side) }
}
