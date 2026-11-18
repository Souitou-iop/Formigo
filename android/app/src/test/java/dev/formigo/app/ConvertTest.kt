package dev.formigo.app

import dev.formigo.app.convert.Archive
import dev.formigo.app.convert.ConvertException
import dev.formigo.app.convert.DataConvert
import dev.formigo.app.convert.DocConvert
import dev.formigo.app.convert.Encoders
import dev.formigo.app.convert.Engine
import dev.formigo.app.convert.Html
import dev.formigo.app.convert.Json
import dev.formigo.app.convert.Options
import dev.formigo.app.convert.Pixels
import dev.formigo.app.convert.Subtitles
import dev.formigo.app.convert.Toml
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.io.File
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream
import javax.imageio.ImageIO

class ConvertTest {
    @get:Rule
    val tmp = TemporaryFolder()

    private val sample = """
        {"name": "Formigo", "version": 2, "ratio": 1.5, "tags": ["go", "kotlin"],
         "owner": {"id": 7, "ok": true}, "items": [{"a": 1, "b": "x"}, {"a": 2, "b": "y,z"}]}
    """.trimIndent()

    @Test
    fun dataRoundTripsThroughEveryFormat() {
        val v = Json.parse(sample)
        for (f in listOf("json", "yaml", "toml", "xml")) {
            val text = DataConvert.write(f, v)
            val back = DataConvert.parse(f, text)
            if (f == "xml") {
                // XML holds one root; its values come back typed by their text.
                @Suppress("UNCHECKED_CAST")
                val root = (back as Map<String, Any?>)["root"] as Map<String, Any?>
                assertEquals("Formigo", root["name"])
                assertEquals(2L, root["version"])
                assertEquals(listOf("go", "kotlin"), root["tags"])
            } else {
                assertEquals("via $f", v, back)
            }
        }
    }

    @Test
    fun jsonKeepsOrderAndIndent() {
        val out = DataConvert.convert("""{"b":1,"a":[true,null,"é"]}""", "json", "json", Options(indent = 1))
        assertEquals("{\n    \"b\": 1,\n    \"a\": [\n        true,\n        null,\n        \"é\"\n    ]\n}\n", out)
        assertEquals("{\"b\":1,\"a\":[true,null,\"é\"]}\n", DataConvert.convert("""{"b":1,"a":[true,null,"é"]}""", "json", "json", Options(indent = 2)))
    }

    @Test
    fun badJsonSaysWhere() {
        try {
            Json.parse("{\n  \"a\": 1,\n  \"b\" 2\n}")
            throw AssertionError("parsed")
        } catch (e: ConvertException) {
            assertTrue(e.message!!, e.message!!.contains("第 3 行"))
        }
    }

    @Test
    fun tomlReadsTablesAndArraysOfTables() {
        val t = Toml.parse(
            """
            # a comment
            title = "TOML 示例"
            when = 1979-05-27T07:32:00Z
            [owner]
            name = 'Tom'
            dob.year = 1979
            [[products]]
            name = "Hammer"
            sku = 738_594_937
            [[products]]
            name = "Nail"
            colors = ["gray", "silver",]
            point = { x = 1, y = 2.5 }
            """.trimIndent(),
        )
        assertEquals("TOML 示例", t["title"])
        assertEquals("1979-05-27T07:32:00Z", t["when"])
        @Suppress("UNCHECKED_CAST")
        val owner = t["owner"] as Map<String, Any?>
        assertEquals("Tom", owner["name"])
        assertEquals(mapOf("year" to 1979L), owner["dob"])
        @Suppress("UNCHECKED_CAST")
        val products = t["products"] as List<Map<String, Any?>>
        assertEquals(738594937L, products[0]["sku"])
        assertEquals(listOf("gray", "silver"), products[1]["colors"])
        assertEquals(mapOf("x" to 1L, "y" to 2.5), products[1]["point"])
        // And back again.
        assertEquals(t, Toml.parse(Toml.write(t)))
    }

    @Test
    fun tablesBecomeRecordsAndBack() {
        val csv = "name,age,city\n\"Lee, Ann\",31,\"He said \"\"hi\"\"\"\nBo,,Paris\n"
        val v = DataConvert.parse("csv", csv)
        assertEquals(
            listOf(mapOf("name" to "Lee, Ann", "age" to 31L, "city" to "He said \"hi\""), mapOf("name" to "Bo", "age" to "", "city" to "Paris")),
            v,
        )
        assertEquals(csv, DataConvert.write("csv", v))
        val tsv = DataConvert.convert(csv, "csv", "tsv")
        assertEquals("name\tage\tcity\nLee, Ann\t31\tHe said \"hi\"\nBo\t\tParis\n", tsv)
        // Nested records flatten to dotted columns.
        val flat = DataConvert.write("csv", Json.parse(sample))
        assertEquals("a,b\n1,x\n2,\"y,z\"\n", DataConvert.write("csv", (Json.parse(sample) as Map<*, *>)["items"]))
        assertTrue(flat, flat.startsWith("key,value\nname,Formigo\n"))
    }

    @Test
    fun xmlAttributesTextAndRepeats() {
        val v = DataConvert.parse("xml", "<lib v=\"2\"><book id=\"1\">A &amp; B</book><book id=\"2\">C</book><empty/></lib>")
        assertEquals(
            mapOf("lib" to mapOf("@v" to 2L, "book" to listOf(mapOf("@id" to 1L, "#text" to "A & B"), mapOf("@id" to 2L, "#text" to "C")), "empty" to null)),
            v,
        )
        val xml = DataConvert.write("xml", v)
        assertTrue(xml, xml.contains("<book id=\"1\">A &amp; B</book>"))
        assertEquals(v, DataConvert.parse("xml", xml))
    }

    @Test
    fun yamlReadsDocuments() {
        val v = DataConvert.parse("yaml", "name: x\nlist:\n  - 1\n  - two\nnested: {a: true}\n")
        assertEquals(mapOf("name" to "x", "list" to listOf(1L, "two"), "nested" to mapOf("a" to true)), v)
    }

    @Test
    fun markdownToHtml() {
        val html = DocConvert.convert(
            "# 标题\n\nSome **bold** and *it* and `code` and [link](https://x.y).\n\n- one\n- [x] two\n\n```kotlin\nval a = 1 < 2\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n> quote\n",
            "md", "html", "标题",
        )
        assertTrue(html.contains("<title>标题</title>"))
        assertTrue(html.contains("<h1>标题</h1>"))
        assertTrue(html.contains("<strong>bold</strong>"))
        assertTrue(html.contains("<em>it</em>"))
        assertTrue(html.contains("<code>code</code>"))
        assertTrue(html.contains("<a href=\"https://x.y\">link</a>"))
        assertTrue(html.contains("<li>one</li>"))
        assertTrue(html.contains("checked"))
        assertTrue(html.contains("<pre><code class=\"language-kotlin\">val a = 1 &lt; 2</code></pre>"))
        assertTrue(html.contains("<td>1</td>"))
        assertTrue(html.contains("<blockquote>"))
    }

    @Test
    fun htmlToMarkdownAndText() {
        val src = "<html><head><title>t</title><style>p{}</style></head><body><h2>Hi</h2><p>A <b>b</b> <a href='/u'>l</a></p><ul><li>x<li>y</ul><script>no()</script></body></html>"
        val md = Html.toMarkdown(src)
        assertTrue(md, md.contains("## Hi"))
        assertTrue(md, md.contains("A **b** [l](/u)"))
        assertTrue(md, md.contains("- x\n- y"))
        assertFalse(md.contains("no()"))
        val text = Html.toText(src)
        assertTrue(text, text.contains("Hi\n\nA b l"))
        assertTrue(text, text.contains("• x"))
        // Markdown made from HTML renders as it.
        assertTrue(DocConvert.convert(md, "md", "html", "t").contains("<strong>b</strong>"))
    }

    @Test
    fun subtitlesConvertEachWay() {
        val srt = "1\n00:00:01,000 --> 00:00:02,500\n<i>Hello</i>\nworld\n\n2\n00:01:00,250 --> 00:01:03,000\n再见\n"
        val vtt = Subtitles.convert(srt, "srt", "vtt")
        assertTrue(vtt.startsWith("WEBVTT\n\n00:00:01.000 --> 00:00:02.500\nHello\nworld\n"))
        val ass = Subtitles.convert(vtt, "vtt", "ass")
        assertTrue(ass, ass.contains("Dialogue: 0,0:00:01.00,0:00:02.50,Default,,0,0,0,,Hello\\Nworld"))
        assertEquals(srt.replace("<i>", "").replace("</i>", ""), Subtitles.convert(ass, "ass", "srt"))
        val lrc = Subtitles.convert(srt, "srt", "lrc")
        assertEquals("[00:01.00]Hello world\n[01:00.25]再见\n", lrc)
        val back = Subtitles.parse("lrc", lrc)
        assertEquals(1000L, back[0].start)
        assertEquals(60250L, back[0].end) // a line shows until the next
    }

    private fun picture(alpha: Boolean): Pixels {
        val w = 37
        val h = 23
        return Pixels(w, h, IntArray(w * h) { i ->
            val x = i % w
            val y = i / w
            val a = if (alpha && x < 5) 0 else 255
            (a shl 24) or ((x * 255 / w) shl 16) or ((y * 255 / h) shl 8) or 0x40
        })
    }

    @Test
    fun gifDecodesBackClose() {
        for (alpha in listOf(false, true)) {
            val src = picture(alpha)
            val out = ByteArrayOutputStream().also { Encoders.gif(src, it) }.toByteArray()
            val img = ImageIO.read(ByteArrayInputStream(out))
            assertEquals(src.width, img.width)
            assertEquals(src.height, img.height)
            for (y in 0 until src.height step 5) for (x in 0 until src.width step 4) {
                val want = src.argb[y * src.width + x]
                val got = img.getRGB(x, y)
                if ((want ushr 24) == 0) { assertEquals(0, got ushr 24); continue }
                for (shift in listOf(16, 8, 0)) {
                    val d = Math.abs(((want shr shift) and 0xff) - ((got shr shift) and 0xff))
                    assertTrue("pixel $x,$y channel $shift off by $d", d <= 12)
                }
            }
        }
    }

    @Test
    fun gifOfManyColorsStillDecodes() {
        // Enough distinct colors and pixels to fill the LZW table and clear it.
        val w = 300
        val h = 200
        val src = Pixels(w, h, IntArray(w * h) { i -> (0xff shl 24) or ((i * 2654435761L).toInt() and 0xffffff) })
        val out = ByteArrayOutputStream().also { Encoders.gif(src, it) }.toByteArray()
        val img = ImageIO.read(ByteArrayInputStream(out))
        assertEquals(w, img.width)
        assertEquals(h, img.height)
    }

    @Test
    fun bmpDecodesBackExactly() {
        val src = picture(false)
        val out = ByteArrayOutputStream().also { Encoders.bmp(src, it) }.toByteArray()
        val img = ImageIO.read(ByteArrayInputStream(out))
        for (y in 0 until src.height) for (x in 0 until src.width) assertEquals(src.argb[y * src.width + x], img.getRGB(x, y))
        // With alpha: a 32-bit BMP with its masks.
        val a = ByteArrayOutputStream().also { Encoders.bmp(picture(true), it) }.toByteArray()
        assertEquals(32, a[28].toInt())
    }

    @Test
    fun icoDirectory() {
        val pngs = listOf(256 to ByteArray(10) { 1 }, 32 to ByteArray(4) { 2 })
        val b = ByteArrayOutputStream().also { Encoders.ico(pngs, it) }.toByteArray()
        assertEquals(listOf(0, 0, 1, 0, 2, 0), b.take(6).map { it.toInt() })
        assertEquals(0, b[6].toInt()) // 256 is written as 0
        assertEquals(32, b[22].toInt())
        assertEquals(6 + 32 + 14, b.size)
        assertEquals(listOf(256, 128, 64, 48, 32, 16), Encoders.icoSizes(1000))
        assertEquals(listOf(32, 16), Encoders.icoSizes(40))
    }

    @Test
    fun zipPacksAndUnpacks() {
        val a = tmp.newFile("a.txt").apply { writeText("alpha") }
        val dir = tmp.newFolder("sub")
        val b = File(dir, "a.txt").apply { writeText("beta") }
        val zip = File(tmp.root, "out.zip")
        Archive.zip(listOf(a to "a.txt", b to "a.txt"), zip)
        val files = Archive.unzip(zip, tmp.newFolder("x"))
        assertEquals(setOf("a.txt", "a (1).txt"), files.map { it.name }.toSet())
        assertEquals(setOf("alpha", "beta"), files.map { it.readText() }.toSet())
    }

    @Test
    fun unzipKeepsEntriesInside() {
        val zip = File(tmp.root, "evil.zip")
        ZipOutputStream(zip.outputStream()).use { z ->
            z.putNextEntry(ZipEntry("../escape.txt")); z.write("x".toByteArray()); z.closeEntry()
            z.putNextEntry(ZipEntry("ok/inner.txt")); z.write("y".toByteArray()); z.closeEntry()
            z.putNextEntry(ZipEntry("__MACOSX/._junk")); z.write("z".toByteArray()); z.closeEntry()
        }
        val out = tmp.newFolder("dest")
        val files = Archive.unzip(zip, out)
        assertEquals(listOf("inner.txt"), files.map { it.name })
        assertFalse(File(tmp.root, "escape.txt").exists())
    }

    @Test
    fun namesAndText() {
        val d = tmp.newFolder("n")
        File(d, "a.txt").writeText("")
        assertEquals("a (1).txt", Engine.unique(d, "a", "txt").name)
        assertEquals("a_b.txt", Engine.unique(d, "a/b", "txt").name)
        val gbk = tmp.newFile("g.txt").apply { writeBytes("中文字幕".toByteArray(charset("GB18030"))) }
        assertEquals("中文字幕", Engine.readText(gbk))
        assertEquals("1:05", Engine.duration(65_000))
        val h = Engine.wavHeader(44100, 2, 16, 1000)
        assertEquals("RIFF", String(h, 0, 4))
        assertEquals(44, h.size)
    }
}
