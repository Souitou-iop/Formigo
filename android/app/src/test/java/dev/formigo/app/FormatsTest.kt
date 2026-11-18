package dev.formigo.app

import dev.formigo.app.convert.Formats
import dev.formigo.app.convert.Kind
import dev.formigo.app.convert.Opt
import dev.formigo.app.convert.optionsFor
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class FormatsTest {
    private fun ids(name: String) = Formats.targetsOf(Formats.detect(name)!!).map { it.id }

    @Test
    fun detectsByExtension() {
        assertEquals(Kind.IMAGE, Formats.detect("IMG_0001.HEIC")!!.kind)
        assertEquals("jpg", Formats.detect("a.jpeg")!!.id)
        assertEquals(Kind.VIDEO, Formats.detect("clip.mov")!!.kind)
        assertEquals(Kind.AUDIO, Formats.detect("song.mp3")!!.kind)
        assertEquals(Kind.DATA, Formats.detect("conf.yml")!!.kind)
        assertEquals(Kind.SUBTITLE, Formats.detect("movie.ssa")!!.kind)
        assertEquals(Kind.ARCHIVE, Formats.detect("x.zip")!!.kind)
        assertEquals(Kind.DOC, Formats.detect("paper.pdf")!!.kind)
        assertNull(Formats.detect("thing.exe"))
        assertNull(Formats.detect("noext"))
    }

    @Test
    fun decodersNeedTheirApiLevel() {
        assertNull(Formats.detect("a.avif", api = 30))
        assertNotNull(Formats.detect("a.avif", api = 31))
        assertNotNull(Formats.detect("a.heic", api = 28))
    }

    @Test
    fun targetsLeaveOutWhatTheFileIs() {
        assertFalse("jpg" in ids("a.jpg"))
        assertTrue(ids("a.jpg").containsAll(listOf("png", "webp", "gif", "bmp", "ico", "pdf", "zip")))
        assertTrue("jpg" in ids("a.heic"))
        assertEquals(listOf("png", "jpg", "zip"), ids("a.pdf"))
        assertEquals(listOf("unzip"), ids("a.zip"))
        assertTrue(ids("a.mov").containsAll(listOf("mp4", "hevc", "m4a", "aac", "wav")))
        assertTrue(ids("a.mp3").containsAll(listOf("m4a", "wav", "aac")))
        assertFalse("mp4" in ids("a.mp3"))
        assertEquals(listOf("json", "toml", "csv", "tsv", "xml", "zip"), ids("a.yaml"))
        assertEquals(listOf("vtt", "ass", "lrc", "zip"), ids("a.srt"))
        assertTrue(ids("a.md").containsAll(listOf("html", "txt", "pdf")))
    }

    @Test
    fun everyTargetIsRegistered() {
        for (s in Formats.sources.values) for (t in s.targets) assertNotNull("${s.id} → $t", Formats.format(t))
    }

    @Test
    fun resolveFollowsOwnThenGroupThenDefault() {
        val heic = Formats.detect("a.heic")!!
        val pdf = Formats.detect("a.pdf")!!
        assertEquals("png", Formats.resolve(heic, "png", "jpg"))
        assertEquals("webp", Formats.resolve(heic, null, "webp"))
        assertEquals("jpg", Formats.resolve(heic, null, null))
        // A group target the file cannot become falls to what it can.
        assertEquals("png", Formats.resolve(pdf, null, "html"))
        // A file of the group's format already is it.
        assertEquals("jpg", Formats.resolve(Formats.detect("b.jpg")!!, null, "jpg"))
    }

    @Test
    fun mixedBatchesShareTargets() {
        val img = Formats.detect("a.png")!!
        val md = Formats.detect("a.md")!!
        val mov = Formats.detect("a.mov")!!
        assertEquals(listOf("pdf", "zip"), Formats.common(listOf(img, md)).map { it.id })
        assertEquals(listOf("zip"), Formats.common(listOf(img, md, mov)).map { it.id })
        // A group's strip offers whatever any file in it may become.
        val union = Formats.union(listOf(md, Formats.detect("a.pdf")!!)).map { it.id }
        assertTrue(union.containsAll(listOf("png", "jpg", "html", "md", "txt", "pdf", "zip")))
    }

    @Test
    fun kindDefaultsAreChoosable() {
        for (k in Kind.entries) {
            val choices = Formats.kindTargets(k).map { it.id }
            assertTrue("$k default", Formats.defaultTarget(k) in choices)
            assertFalse("zip is not a default", "zip" in choices)
        }
    }

    @Test
    fun optionsFollowTheTarget() {
        assertEquals(listOf(Opt.QUALITY, Opt.SCALE), optionsFor(Kind.IMAGE, "jpg"))
        assertEquals(listOf(Opt.SCALE), optionsFor(Kind.IMAGE, "png"))
        assertEquals(listOf(Opt.PDF_SCALE), optionsFor(Kind.DOC, "png"))
        assertEquals(listOf(Opt.DEPTH, Opt.MONO), optionsFor(Kind.AUDIO, "wav"))
        assertEquals(listOf(Opt.VIDEO_SIZE), optionsFor(Kind.VIDEO, "hevc"))
        assertEquals(listOf(Opt.INDENT), optionsFor(Kind.DATA, "json"))
        assertTrue(optionsFor(Kind.DATA, "yaml").isEmpty())
        assertTrue(optionsFor(Kind.SUBTITLE, "srt").isEmpty())
    }
}
