import AVFoundation
import ImageIO
import PDFKit
import SwiftUI
import UIKit
import XCTest
@testable import Formigo

final class FormatMappingTests: XCTestCase {
    func testSourcesAreRecognized() {
        XCTAssertEqual(Formats.source(for: URL(fileURLWithPath: "/a/IMG_1.HEIC"))?.kind, .image)
        XCTAssertEqual(Formats.source(for: URL(fileURLWithPath: "/a/b.jpeg"))?.id, "jpg")
        XCTAssertEqual(Formats.source(for: URL(fileURLWithPath: "/a/b.yml"))?.id, "yaml")
        XCTAssertEqual(Formats.source(for: URL(fileURLWithPath: "/a/b.MOV"))?.kind, .video)
        XCTAssertEqual(Formats.source(for: URL(fileURLWithPath: "/a/b.flac"))?.kind, .audio)
        XCTAssertEqual(Formats.source(for: URL(fileURLWithPath: "/a/b.ssa"))?.id, "ass")
        XCTAssertEqual(Formats.source(for: URL(fileURLWithPath: "/a/b.pdf"))?.kind, .document)
        XCTAssertNil(Formats.source(for: URL(fileURLWithPath: "/a/b.exe")))
    }

    func testTargetsLeaveOutTheSourceFormat() {
        XCTAssertFalse(Formats.targets(forExtension: "png").contains("png"))
        XCTAssertFalse(Formats.targets(forExtension: "jpeg").contains("jpg"))
        XCTAssertFalse(Formats.targets(forExtension: "yml").contains("yaml"))
        XCTAssertTrue(Formats.targets(forExtension: "heic").contains("jpg"))
        XCTAssertTrue(Formats.targets(forExtension: "heic").contains("pdf"))
        XCTAssertEqual(Formats.targets(forExtension: "pdf"), ["png", "jpg", "txt", "zip"])
        XCTAssertEqual(Formats.targets(forExtension: "zip"), ["unzip"])
        XCTAssertTrue(Formats.targets(forExtension: "mov").contains("mp4"))
        XCTAssertTrue(Formats.targets(forExtension: "mov").contains("gif"))
        XCTAssertTrue(Formats.targets(forExtension: "mp3").contains("alac"))
        XCTAssertTrue(Formats.targets(forExtension: "srt").contains("lrc"))
    }

    func testEveryTargetIsRegistered() {
        for ext in Formats.sources.keys {
            for t in Formats.targets(forExtension: ext) {
                XCTAssertNotNil(Formats.format(t), "\(ext) → \(t)")
            }
        }
    }

    func testImageWritersIncludeTheBasics() {
        for id in ["jpg", "png", "heic", "gif", "bmp", "tiff"] {
            XCTAssertTrue(Formats.writableImages.contains(id), id)
        }
    }

    func testRelevantOptions() {
        XCTAssertEqual(OptionSet.relevant(kind: .image, targets: ["jpg"]).quality, true)
        XCTAssertEqual(OptionSet.relevant(kind: .image, targets: ["png"]).quality, false)
        XCTAssertEqual(OptionSet.relevant(kind: .image, targets: ["png"]).scale, true)
        XCTAssertEqual(OptionSet.relevant(kind: .audio, targets: ["wav"]).bitDepth, true)
        XCTAssertEqual(OptionSet.relevant(kind: .data, targets: ["json"]).indent, true)
        XCTAssertFalse(OptionSet.relevant(kind: .data, targets: ["csv"]).any)
        XCTAssertFalse(OptionSet.relevant(kind: .subtitle, targets: ["vtt"]).any)
    }

    @MainActor
    func testMixedQueueTargets() throws {
        let dir = try tempDir()
        let png = dir.appendingPathComponent("a.png")
        try Fixtures.png(width: 8, height: 8).write(to: png)
        let md = dir.appendingPathComponent("b.md")
        try "# hi".write(to: md, atomically: true, encoding: .utf8)
        let q = Queue(settings: AppSettings())
        q.add([png, md], copy: false)
        XCTAssertEqual(q.kinds, [.image, .document])
        XCTAssertTrue(q.isMixed)
        XCTAssertEqual(q.commonTargets, ["pdf", "zip"])
        XCTAssertTrue(q.showFresh)
        q.setAllTarget("pdf")
        XCTAssertEqual(q.items.map { q.target(of: $0) }, ["pdf", "pdf"])
        q.setOverride(q.items[0].id, "jpg")
        XCTAssertEqual(q.target(of: q.items[0]), "jpg")
        q.setGroupTarget(.document, "html")
        XCTAssertNil(q.allTarget)
        XCTAssertEqual(q.target(of: q.items[1]), "html")
        q.setOverride(q.items[0].id, nil)
        XCTAssertEqual(q.target(of: q.items[0]), "pdf", "the image group keeps the batch's choice")
    }
}

final class DataConversionTests: XCTestCase {
    let sample = #"{"name":"Formigo","version":3,"ratio":0.5,"ok":true,"none":null,"tags":["a","b"],"nested":{"x":1,"y":"2"}}"#

    func testJSONKeepsKeyOrder() throws {
        let v = try JSONCodec.decode(sample)
        guard case .object(let o) = v else { return XCTFail() }
        XCTAssertEqual(o.map(\.0), ["name", "version", "ratio", "ok", "none", "tags", "nested"])
        XCTAssertEqual(try JSONCodec.decode(JSONCodec.encode(v, indent: 2)), v)
        XCTAssertEqual(try JSONCodec.decode(JSONCodec.encode(v, indent: 0)), v)
        XCTAssertFalse(JSONCodec.encode(v, indent: 0).contains("\n"))
        XCTAssertEqual(try JSONCodec.decode(#""你😀""#), .string("你😀"))
    }

    func testYAMLRoundTrip() throws {
        let v = try JSONCodec.decode(sample)
        let yaml = try DataEngine.encode(v, as: "yaml")
        XCTAssertTrue(yaml.contains("name: Formigo"))
        XCTAssertFalse(yaml.contains("y: 2\n"), yaml)
        XCTAssertEqual(try DataEngine.decode(yaml, as: "yaml"), v)
    }

    func testTOMLRoundTrip() throws {
        let src = #"{"title":"x","owner":{"name":"Tom","age":30},"items":[{"id":1},{"id":2}],"list":[1,2,3]}"#
        let v = try JSONCodec.decode(src)
        let toml = try DataEngine.encode(v, as: "toml")
        XCTAssertTrue(toml.contains("[owner]"), toml)
        XCTAssertTrue(toml.contains("[[items]]"), toml)
        let back = try DataEngine.decode(toml, as: "toml")
        XCTAssertEqual(back["owner"]?["age"], .int(30))
        XCTAssertEqual(back["list"], .array([.int(1), .int(2), .int(3)]))
        guard case .array(let items) = back["items"] else { return XCTFail() }
        XCTAssertEqual(items.count, 2)
    }

    func testCSVAndTSV() throws {
        let csv = "name,age,note\nAda,36,\"likes, commas\"\nBob,41,\"say \"\"hi\"\"\"\n"
        let v = try DataEngine.decode(csv, as: "csv")
        guard case .array(let rows) = v else { return XCTFail() }
        XCTAssertEqual(rows.count, 2)
        XCTAssertEqual(rows[0]["age"], .int(36))
        XCTAssertEqual(rows[0]["note"], .string("likes, commas"))
        XCTAssertEqual(rows[1]["note"], .string("say \"hi\""))
        XCTAssertEqual(try DataEngine.encode(v, as: "csv"), csv)
        let tsv = try DataEngine.encode(v, as: "tsv")
        XCTAssertTrue(tsv.hasPrefix("name\tage\tnote\n"))
        XCTAssertEqual(try DataEngine.decode(tsv, as: "tsv"), v)
    }

    func testXMLAndPlist() throws {
        let xml = #"<library><book id="1"><title>A</title></book><book id="2"><title>B</title></book></library>"#
        let v = try DataEngine.decode(xml, as: "xml")
        guard case .array(let books) = v["library"]?["book"] else { return XCTFail("\(v)") }
        XCTAssertEqual(books.count, 2)
        XCTAssertEqual(books[1]["@id"], .string("2"))
        let again = try DataEngine.decode(DataEngine.encode(v, as: "xml"), as: "xml")
        XCTAssertEqual(again, v)

        let j = try JSONCodec.decode(#"{"a":1,"b":[true,"x"],"c":{"d":2.5}}"#)
        let plist = try PlistCodec.encode(j)
        XCTAssertEqual(try PlistCodec.decode(plist), j)
    }

    func testDataFileConversion() async throws {
        let dir = try tempDir()
        let src = dir.appendingPathComponent("conf.yaml")
        try "server:\n  port: 8080\n  hosts: [a, b]\n".write(to: src, atomically: true, encoding: .utf8)
        let out = try await Engine.convert(src, to: "json", into: dir.appendingPathComponent("out"), baseName: "conf")
        XCTAssertEqual(out.lastPathComponent, "conf.json")
        let v = try JSONCodec.decode(String(contentsOf: out, encoding: .utf8))
        XCTAssertEqual(v["server"]?["port"], .int(8080))
    }
}

final class TextConversionTests: XCTestCase {
    func testSubtitles() throws {
        let srt = "1\n00:00:01,000 --> 00:00:02,500\nHello\n\n2\n00:00:03,000 --> 00:00:04,000\nTwo\nlines\n"
        let cues = try SubtitleEngine.parse(srt, as: "srt")
        XCTAssertEqual(cues, [Cue(start: 1, end: 2.5, text: "Hello"), Cue(start: 3, end: 4, text: "Two\nlines")])
        let vtt = try SubtitleEngine.write(cues, as: "vtt")
        XCTAssertTrue(vtt.hasPrefix("WEBVTT"))
        XCTAssertTrue(vtt.contains("00:00:01.000 --> 00:00:02.500"))
        XCTAssertEqual(try SubtitleEngine.parse(vtt, as: "vtt"), cues)
        let ass = try SubtitleEngine.write(cues, as: "ass")
        XCTAssertEqual(try SubtitleEngine.parse(ass, as: "ass"), cues)
        XCTAssertEqual(try SubtitleEngine.write(cues, as: "srt"), srt)
        let lrc = try SubtitleEngine.write(cues, as: "lrc")
        XCTAssertTrue(lrc.hasPrefix("[00:01.00]Hello"), lrc)
        let back = try SubtitleEngine.parse(lrc, as: "lrc")
        XCTAssertEqual(back.map(\.start), [1, 3])
        XCTAssertEqual(back[0].end, 3)
    }

    func testMarkdownToHTML() {
        let html = Markdown.html("# Title\n\nSome **bold** and *it* with `code` and [a link](https://x.y).\n\n- one\n- two\n\n```swift\nlet a = 1 < 2\n```")
        XCTAssertTrue(html.contains("<h1>Title</h1>"))
        XCTAssertTrue(html.contains("<strong>bold</strong>"))
        XCTAssertTrue(html.contains("<em>it</em>"))
        XCTAssertTrue(html.contains("<code>code</code>"))
        XCTAssertTrue(html.contains("<a href=\"https://x.y\">a link</a>"))
        XCTAssertTrue(html.contains("<ul>\n<li>one</li>\n<li>two</li>\n</ul>"))
        XCTAssertTrue(html.contains("let a = 1 &lt; 2"))
        XCTAssertFalse(Markdown.html("snake_case_name").contains("<em>"))
    }

    func testDocumentConversions() async throws {
        let dir = try tempDir()
        let md = dir.appendingPathComponent("note.md")
        try "# Hello\n\nThis is **Formigo**.\n\n- a\n- b\n".write(to: md, atomically: true, encoding: .utf8)
        let out = dir.appendingPathComponent("out")

        let html = try await Engine.convert(md, to: "html", into: out, baseName: "note")
        XCTAssertTrue(try String(contentsOf: html, encoding: .utf8).contains("<h1>Hello</h1>"))

        let txt = try await Engine.convert(md, to: "txt", into: out, baseName: "note")
        let text = try String(contentsOf: txt, encoding: .utf8)
        XCTAssertTrue(text.contains("Hello") && text.contains("This is Formigo."), text)

        let rtf = try await Engine.convert(md, to: "rtf", into: out, baseName: "note")
        XCTAssertTrue(try String(contentsOf: rtf, encoding: .utf8).hasPrefix("{\\rtf"))

        let back = try await Engine.convert(rtf, to: "md", into: dir.appendingPathComponent("back"), baseName: "note")
        let mdBack = try String(contentsOf: back, encoding: .utf8)
        XCTAssertTrue(mdBack.contains("# Hello"), mdBack)
        XCTAssertTrue(mdBack.contains("**Formigo**"), mdBack)

        let pdf = try await Engine.convert(md, to: "pdf", into: out, baseName: "note")
        let doc = try XCTUnwrap(PDFDocument(url: pdf))
        XCTAssertGreaterThanOrEqual(doc.pageCount, 1)
        XCTAssertTrue(doc.string?.contains("Hello") ?? false, doc.string ?? "nil")
    }

    func testZipRoundTrip() async throws {
        let dir = try tempDir()
        let f = dir.appendingPathComponent("data.json")
        try "{\"a\":1}".write(to: f, atomically: true, encoding: .utf8)
        let zip = try await Engine.convert(f, to: "zip", into: dir.appendingPathComponent("z"), baseName: "data")
        XCTAssertEqual(zip.pathExtension, "zip")
        let folder = try await Engine.convert(zip, to: "unzip", into: dir.appendingPathComponent("u"), baseName: "data")
        XCTAssertEqual(try String(contentsOf: folder.appendingPathComponent("data.json"), encoding: .utf8), "{\"a\":1}")
    }
}

final class MediaConversionTests: XCTestCase {
    func testImageConversions() async throws {
        let dir = try tempDir()
        let png = dir.appendingPathComponent("pic.png")
        try Fixtures.png(width: 320, height: 200).write(to: png)
        for id in ["jpg", "heic", "gif", "bmp", "tiff", "ico"] {
            let out = try await Engine.convert(png, to: id, options: ConvertOptions(quality: 0.7), into: dir.appendingPathComponent(id), baseName: "pic")
            let src = try XCTUnwrap(CGImageSourceCreateWithURL(out as CFURL, nil), id)
            XCTAssertEqual(CGImageSourceGetType(src) as String?, Formats.imageUTIs[id], id)
            let props = CGImageSourceCopyPropertiesAtIndex(src, 0, nil) as? [CFString: Any]
            if id == "ico" {
                XCTAssertEqual(props?[kCGImagePropertyPixelWidth] as? Int, props?[kCGImagePropertyPixelHeight] as? Int)
            } else {
                XCTAssertEqual(props?[kCGImagePropertyPixelWidth] as? Int, 320, id)
            }
        }
        let half = try await Engine.convert(png, to: "jpg", options: ConvertOptions(scale: 0.5), into: dir.appendingPathComponent("half"), baseName: "pic")
        let hp = CGImageSourceCopyPropertiesAtIndex(CGImageSourceCreateWithURL(half as CFURL, nil)!, 0, nil) as? [CFString: Any]
        XCTAssertEqual(hp?[kCGImagePropertyPixelWidth] as? Int, 160)

        let pdf = try await Engine.convert(png, to: "pdf", into: dir.appendingPathComponent("pdf"), baseName: "pic")
        let doc = try XCTUnwrap(PDFDocument(url: pdf))
        XCTAssertEqual(doc.pageCount, 1)

        let back = try await Engine.convert(pdf, to: "png", into: dir.appendingPathComponent("back"), baseName: "pic")
        XCTAssertNotNil(UIImage(contentsOfFile: back.path))
    }

    func testAudioConversions() async throws {
        let dir = try tempDir()
        let wav = dir.appendingPathComponent("tone.wav")
        try Fixtures.wav(to: wav, seconds: 1, rate: 44100, channels: 2)
        for (id, ext) in [("m4a", "m4a"), ("alac", "m4a"), ("aiff", "aiff"), ("caf", "caf")] {
            let out = try await Engine.convert(wav, to: id, options: ConvertOptions(bitDepth: 24, mono: id == "aiff"),
                                               into: dir.appendingPathComponent(id), baseName: "tone")
            XCTAssertEqual(out.pathExtension, ext)
            let f = try AVAudioFile(forReading: out)
            XCTAssertEqual(Double(f.length) / f.fileFormat.sampleRate, 1, accuracy: 0.1, id)
            XCTAssertEqual(f.fileFormat.channelCount, id == "aiff" ? 1 : 2, id)
        }
    }

    func testVideoConversions() async throws {
        let dir = try tempDir()
        let mov = dir.appendingPathComponent("clip.mov")
        try await Fixtures.movie(to: mov, seconds: 1)
        let mp4 = try await Engine.convert(mov, to: "mp4", into: dir.appendingPathComponent("mp4"), baseName: "clip")
        let asset = AVURLAsset(url: mp4)
        let tracks = try await asset.loadTracks(withMediaType: .video)
        XCTAssertEqual(tracks.count, 1)
        let gif = try await Engine.convert(mov, to: "gif", into: dir.appendingPathComponent("gif"), baseName: "clip")
        let src = try XCTUnwrap(CGImageSourceCreateWithURL(gif as CFURL, nil))
        XCTAssertGreaterThan(CGImageSourceGetCount(src), 5)
        do {
            _ = try await Engine.convert(mov, to: "m4a", into: dir.appendingPathComponent("m4a"), baseName: "clip")
            XCTFail("a silent clip has no audio track")
        } catch ConvertError.noAudioTrack {}
    }
}

// MARK: - Fixtures

func tempDir() throws -> URL {
    let d = FileManager.default.temporaryDirectory.appendingPathComponent("FormigoTests-\(UUID().uuidString)", isDirectory: true)
    try FileManager.default.createDirectory(at: d, withIntermediateDirectories: true)
    return d
}

enum Fixtures {
    static func png(width: Int, height: Int) -> Data {
        let fmt = UIGraphicsImageRendererFormat()
        fmt.scale = 1
        return UIGraphicsImageRenderer(size: CGSize(width: width, height: height), format: fmt).pngData { ctx in
            UIColor(red: 0.36, green: 0.73, blue: 0.97, alpha: 1).setFill()
            ctx.fill(CGRect(x: 0, y: 0, width: width, height: height))
            UIColor(red: 0.08, green: 0.13, blue: 0.24, alpha: 1).setFill()
            ctx.cgContext.fillEllipse(in: CGRect(x: width / 4, y: height / 4, width: width / 2, height: height / 2))
        }
    }

    static func wav(to url: URL, seconds: Double, rate: Double, channels: AVAudioChannelCount) throws {
        let fmt = AVAudioFormat(standardFormatWithSampleRate: rate, channels: channels)!
        let file = try AVAudioFile(forWriting: url, settings: [
            AVFormatIDKey: kAudioFormatLinearPCM, AVSampleRateKey: rate, AVNumberOfChannelsKey: channels,
            AVLinearPCMBitDepthKey: 16, AVLinearPCMIsFloatKey: false,
        ])
        let frames = AVAudioFrameCount(seconds * rate)
        let buf = AVAudioPCMBuffer(pcmFormat: fmt, frameCapacity: frames)!
        buf.frameLength = frames
        for c in 0..<Int(channels) {
            for i in 0..<Int(frames) { buf.floatChannelData![c][i] = Float(sin(Double(i) * 2 * .pi * 440 / rate) * 0.3) }
        }
        try file.write(from: buf)
    }

    static func movie(to url: URL, seconds: Int) async throws {
        let writer = try AVAssetWriter(outputURL: url, fileType: .mov)
        let input = AVAssetWriterInput(mediaType: .video, outputSettings: [
            AVVideoCodecKey: AVVideoCodecType.h264, AVVideoWidthKey: 320, AVVideoHeightKey: 240,
        ])
        let adaptor = AVAssetWriterInputPixelBufferAdaptor(assetWriterInput: input, sourcePixelBufferAttributes: [
            kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA,
            kCVPixelBufferWidthKey as String: 320, kCVPixelBufferHeightKey as String: 240,
        ])
        writer.add(input)
        writer.startWriting()
        writer.startSession(atSourceTime: .zero)
        let fps = 30
        for f in 0..<(seconds * fps) {
            while !input.isReadyForMoreMediaData { try await Task.sleep(nanoseconds: 5_000_000) }
            var pb: CVPixelBuffer?
            CVPixelBufferPoolCreatePixelBuffer(nil, adaptor.pixelBufferPool!, &pb)
            let buffer = pb!
            CVPixelBufferLockBaseAddress(buffer, [])
            let base = CVPixelBufferGetBaseAddress(buffer)!.assumingMemoryBound(to: UInt8.self)
            let stride = CVPixelBufferGetBytesPerRow(buffer)
            for y in 0..<240 {
                for x in 0..<320 {
                    let p = base + y * stride + x * 4
                    p[0] = UInt8((x + f * 8) % 256); p[1] = UInt8(y % 256); p[2] = 200; p[3] = 255
                }
            }
            CVPixelBufferUnlockBaseAddress(buffer, [])
            adaptor.append(buffer, withPresentationTime: CMTime(value: CMTimeValue(f), timescale: CMTimeScale(fps)))
        }
        input.markAsFinished()
        await writer.finishWriting()
        if writer.status != .completed { throw writer.error ?? ConvertError.failed("fixture") }
    }
}

final class ThemeTests: XCTestCase {
    func testDefaultAndFallbackAreNeutral() {
        XCTAssertEqual(Accent.defaultID, "neutral")
        XCTAssertEqual(Accent.by("blue").id, "neutral")
        XCTAssertEqual(Accent.by("").id, "neutral")
        XCTAssertEqual(Accent.by("orange").id, "orange")
        XCTAssertEqual(Accent.all.map(\.id), ["mint", "orange", "sky", "neutral"])
        XCTAssertEqual(Accent.by("neutral").name, "中性黑白")
    }

    /// A theme tints the whole app, not only its accent: in the light, the
    /// page and the ink of text and sticker edges; in the dark, the surfaces.
    func testThemesTintTheWholeApp() {
        for a in Accent.all {
            let light = Palette.make(dark: false, oled: false, accent: a)
            XCTAssertEqual(light.bg, Color(hex: a.day.bg), a.id)
            XCTAssertEqual(light.outline, Color(hex: a.day.ink), a.id)
            XCTAssertEqual(light.ink, Color(hex: a.day.ink), a.id)
            let dark = Palette.make(dark: true, oled: false, accent: a)
            XCTAssertEqual(dark.bg, Color(hex: a.night.bg), a.id)
            XCTAssertEqual(dark.surface, Color(hex: a.night.surface), a.id)
            XCTAssertEqual(Palette.make(dark: true, oled: true, accent: a).bg, Color.black, a.id)
        }
    }
}

final class UpdateCheckTests: XCTestCase {
    func testAReleaseRunsAheadOfTheRunningVersion() {
        XCTAssertTrue(UpdateCheck.newer("v0.4.2", running: "0.4.1"))
        XCTAssertTrue(UpdateCheck.newer("v1.0", running: "0.9.9"))
        XCTAssertTrue(UpdateCheck.newer("v0.5", running: "0.4.10"))
    }

    func testTheRunningVersionAndOlderOnesStay() {
        XCTAssertFalse(UpdateCheck.newer("v0.4.1", running: "0.4.1"))
        XCTAssertFalse(UpdateCheck.newer("v0.4.0", running: "0.4.1"))
        XCTAssertFalse(UpdateCheck.newer("v0.3.9", running: "0.4.0"))
    }
}

// MARK: - Every source to every target

/// Converts a sample of every format the app reads (testdata/seeds, shared
/// with Android's device test) to every format it offers for it, on this
/// device's own frameworks, and checks that what it wrote reads back.
final class MatrixTests: XCTestCase {
    func testEverySourceToEveryTarget() async throws {
        let seeds = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("testdata/seeds")
        guard let names = try? FileManager.default.contentsOfDirectory(atPath: seeds.path) else {
            throw XCTSkip("no samples at \(seeds.path)")
        }
        let dir = try tempDir()
        var failures: [String] = []
        var tried = 0
        for name in names.sorted() {
            let src = dir.appendingPathComponent(name)
            try FileManager.default.copyItem(at: seeds.appendingPathComponent(name), to: src)
            guard let source = Formats.source(for: src) else { continue }
            for target in Formats.targets(for: source) {
                tried += 1
                do {
                    let out = try await Engine.convert(src, to: target, into: dir.appendingPathComponent("\(source.id)-to-\(target)"), baseName: "seed")
                    if let problem = await readBack(out, target) {
                        failures.append("\(source.id) → \(target): \(problem)")
                    }
                } catch ConvertError.noAudioTrack where source.kind == .video {
                    failures.append("\(source.id) → \(target): no audio track, though the sample has one")
                } catch {
                    failures.append("\(source.id) → \(target): \(error.localizedDescription)")
                }
            }
        }
        print("MATRIX: \(tried) conversions, \(failures.count) failed")
        for f in failures { print("MATRIX FAIL: \(f)") }
        XCTAssertTrue(failures.isEmpty, "\(failures.count) of \(tried) failed:\n" + failures.joined(separator: "\n"))
    }

    /// Why what was written does not read back, or nil when it does.
    private func readBack(_ out: URL, _ target: String) async -> String? {
        var isDir: ObjCBool = false
        guard FileManager.default.fileExists(atPath: out.path, isDirectory: &isDir) else { return "nothing written" }
        if isDir.boolValue {
            let items = (try? FileManager.default.contentsOfDirectory(atPath: out.path)) ?? []
            return items.isEmpty ? "empty folder" : nil
        }
        guard let data = try? Data(contentsOf: out), !data.isEmpty else { return "empty file" }
        let media: Set<String> = ["mp4", "hevc", "mov", "m4a", "alac", "wav", "aiff", "caf"]
        if Formats.imageUTIs[target] != nil {
            guard let s = CGImageSourceCreateWithURL(out as CFURL, nil), CGImageSourceGetCount(s) > 0,
                  CGImageSourceCreateImageAtIndex(s, 0, nil) != nil else { return "does not decode" }
        } else if media.contains(target) {
            let tracks = (try? await AVURLAsset(url: out).load(.tracks)) ?? []
            if tracks.isEmpty { return "no tracks" }
        } else if target == "pdf", PDFDocument(url: out) == nil {
            return "not a PDF"
        } else if target == "zip", data.prefix(2) != Data("PK".utf8) {
            return "not a ZIP"
        }
        return nil
    }
}
