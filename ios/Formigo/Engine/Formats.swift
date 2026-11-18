import Foundation
import ImageIO
import UniformTypeIdentifiers

/// The family a file belongs to. The queue groups files by kind.
enum FileKind: String, CaseIterable, Codable, Identifiable, Sendable {
    case image, video, audio, data, document, subtitle, archive

    var id: String { rawValue }

    var title: String {
        switch self {
        case .image: "图片"
        case .video: "视频"
        case .audio: "音频"
        case .data: "数据"
        case .document: "文档"
        case .subtitle: "字幕"
        case .archive: "压缩包"
        }
    }

    var symbol: String {
        switch self {
        case .image: "photo"
        case .video: "film"
        case .audio: "waveform"
        case .data: "curlybraces"
        case .document: "doc.text"
        case .subtitle: "captions.bubble"
        case .archive: "archivebox"
        }
    }

    /// The target a kind's files go to unless the user chooses.
    var defaultTarget: String {
        switch self {
        case .image: "png"
        case .video: "mp4"
        case .audio: "wav"
        case .data: "json"
        case .document: "html"
        case .subtitle: "srt"
        case .archive: "unzip"
        }
    }
}

/// A format a file can be written in.
struct Format: Hashable, Identifiable, Sendable {
    /// Unique ID, usually the extension.
    let id: String
    let label: String
    let note: String
    /// The extension written; empty for a folder.
    let ext: String
}

/// Format registry: what Formigo reads, what it writes, and what each source
/// format can become. The interface is driven entirely by this table.
enum Formats {
    static let all: [Format] = [
        // Pictures
        Format(id: "jpg", label: "JPG", note: "照片", ext: "jpg"),
        Format(id: "png", label: "PNG", note: "无损", ext: "png"),
        Format(id: "heic", label: "HEIC", note: "省空间", ext: "heic"),
        Format(id: "webp", label: "WEBP", note: "网页", ext: "webp"),
        Format(id: "avif", label: "AVIF", note: "新一代", ext: "avif"),
        Format(id: "gif", label: "GIF", note: "动图", ext: "gif"),
        Format(id: "bmp", label: "BMP", note: "位图", ext: "bmp"),
        Format(id: "tiff", label: "TIFF", note: "印刷", ext: "tiff"),
        Format(id: "ico", label: "ICO", note: "图标", ext: "ico"),
        Format(id: "icns", label: "ICNS", note: "Mac 图标", ext: "icns"),
        Format(id: "pdf", label: "PDF", note: "文档", ext: "pdf"),
        // Video
        Format(id: "mp4", label: "MP4", note: "H.264", ext: "mp4"),
        Format(id: "hevc", label: "HEVC", note: "H.265", ext: "mp4"),
        Format(id: "mov", label: "MOV", note: "QuickTime", ext: "mov"),
        // Sound
        Format(id: "m4a", label: "M4A", note: "AAC", ext: "m4a"),
        Format(id: "alac", label: "ALAC", note: "无损", ext: "m4a"),
        Format(id: "wav", label: "WAV", note: "无压缩", ext: "wav"),
        Format(id: "aiff", label: "AIFF", note: "无压缩", ext: "aiff"),
        Format(id: "caf", label: "CAF", note: "Core Audio", ext: "caf"),
        // Data
        Format(id: "json", label: "JSON", note: "通用", ext: "json"),
        Format(id: "yaml", label: "YAML", note: "配置", ext: "yaml"),
        Format(id: "toml", label: "TOML", note: "配置", ext: "toml"),
        Format(id: "csv", label: "CSV", note: "表格", ext: "csv"),
        Format(id: "tsv", label: "TSV", note: "表格", ext: "tsv"),
        Format(id: "xml", label: "XML", note: "标记", ext: "xml"),
        Format(id: "plist", label: "PLIST", note: "Apple", ext: "plist"),
        // Documents
        Format(id: "html", label: "HTML", note: "网页", ext: "html"),
        Format(id: "md", label: "MD", note: "Markdown", ext: "md"),
        Format(id: "txt", label: "TXT", note: "纯文本", ext: "txt"),
        Format(id: "rtf", label: "RTF", note: "富文本", ext: "rtf"),
        // Subtitles
        Format(id: "srt", label: "SRT", note: "通用", ext: "srt"),
        Format(id: "vtt", label: "VTT", note: "网页", ext: "vtt"),
        Format(id: "ass", label: "ASS", note: "特效", ext: "ass"),
        Format(id: "lrc", label: "LRC", note: "歌词", ext: "lrc"),
        // Archives
        Format(id: "zip", label: "ZIP", note: "打包", ext: "zip"),
        Format(id: "unzip", label: "解压", note: "文件夹", ext: ""),
    ]

    private static let byID = Dictionary(uniqueKeysWithValues: all.map { ($0.id, $0) })

    static func format(_ id: String) -> Format? { byID[id] }

    private static let orderIndex = Dictionary(uniqueKeysWithValues: all.enumerated().map { ($1.id, $0) })

    /// A format's place in display order.
    static func order(_ id: String) -> Int { orderIndex[id] ?? Int.max }

    /// A source extension's kind and its canonical format ID.
    struct Source: Sendable, Hashable { let kind: FileKind; let id: String }

    static let sources: [String: Source] = {
        var m: [String: Source] = [:]
        func add(_ kind: FileKind, _ id: String, _ exts: [String]) {
            for e in exts { m[e] = Source(kind: kind, id: id) }
        }
        add(.image, "jpg", ["jpg", "jpeg", "jpe", "jfif"])
        add(.image, "png", ["png"])
        add(.image, "heic", ["heic", "heics"])
        add(.image, "heif", ["heif", "hif"])
        add(.image, "avif", ["avif"])
        add(.image, "webp", ["webp"])
        add(.image, "gif", ["gif"])
        add(.image, "bmp", ["bmp", "dib"])
        add(.image, "tiff", ["tif", "tiff"])
        add(.image, "ico", ["ico", "cur"])
        add(.image, "icns", ["icns"])
        add(.image, "jxl", ["jxl"])
        add(.image, "dng", ["dng"])
        add(.image, "psd", ["psd"])
        add(.image, "tga", ["tga"])
        add(.document, "pdf", ["pdf"])
        add(.video, "mov", ["mov", "qt"])
        add(.video, "mp4", ["mp4"])
        add(.video, "m4v", ["m4v"])
        add(.video, "3gp", ["3gp", "3g2"])
        add(.audio, "m4a", ["m4a"])
        add(.audio, "aac", ["aac"])
        add(.audio, "wav", ["wav", "wave"])
        add(.audio, "aiff", ["aif", "aiff", "aifc"])
        add(.audio, "caf", ["caf"])
        add(.audio, "mp3", ["mp3"])
        add(.audio, "flac", ["flac"])
        add(.data, "json", ["json", "geojson"])
        add(.data, "yaml", ["yaml", "yml"])
        add(.data, "toml", ["toml"])
        add(.data, "csv", ["csv"])
        add(.data, "tsv", ["tsv", "tab"])
        add(.data, "xml", ["xml"])
        add(.data, "plist", ["plist"])
        add(.document, "md", ["md", "markdown", "mdown", "mkd"])
        add(.document, "html", ["html", "htm", "xhtml"])
        add(.document, "txt", ["txt", "text", "log"])
        add(.document, "rtf", ["rtf"])
        add(.subtitle, "srt", ["srt"])
        add(.subtitle, "vtt", ["vtt"])
        add(.subtitle, "ass", ["ass", "ssa"])
        add(.subtitle, "lrc", ["lrc"])
        add(.archive, "zip", ["zip"])
        return m
    }()

    static func source(for url: URL) -> Source? {
        sources[url.pathExtension.lowercased()]
    }

    /// The UTI ImageIO writes each picture format as.
    static let imageUTIs: [String: String] = [
        "jpg": "public.jpeg", "png": "public.png", "heic": "public.heic",
        "webp": "org.webmproject.webp", "avif": "public.avif", "gif": "com.compuserve.gif",
        "bmp": "com.microsoft.bmp", "tiff": "public.tiff", "ico": "com.microsoft.ico",
        "icns": "com.apple.icns",
    ]

    /// Picture formats this device's ImageIO can write.
    static let writableImages: Set<String> = {
        let types = Set((CGImageDestinationCopyTypeIdentifiers() as? [String]) ?? [])
        return Set(imageUTIs.filter { types.contains($0.value) }.keys)
    }()

    private static let imageOrder = ["jpg", "png", "heic", "webp", "avif", "gif", "bmp", "tiff", "ico", "icns"]

    /// The targets a source format can become, in display order. The source's
    /// own format is left out.
    static func targets(for source: Source) -> [String] {
        let list: [String]
        switch source.kind {
        case .image:
            list = imageOrder.filter { writableImages.contains($0) } + ["pdf", "zip"]
        case .video:
            list = ["mp4", "hevc", "mov", "m4a", "gif", "zip"]
        case .audio:
            list = ["m4a", "alac", "wav", "aiff", "caf", "zip"]
        case .data:
            list = ["json", "yaml", "toml", "csv", "tsv", "xml", "plist", "zip"]
        case .document:
            if source.id == "pdf" {
                list = ["png", "jpg", "txt", "zip"]
            } else {
                list = ["html", "md", "txt", "rtf", "pdf", "zip"]
            }
        case .subtitle:
            list = ["srt", "vtt", "ass", "lrc", "zip"]
        case .archive:
            list = ["unzip"]
        }
        return list.filter { $0 != source.id }
    }

    static func targets(forExtension ext: String) -> [String] {
        guard let s = sources[ext.lowercased()] else { return [] }
        return targets(for: s)
    }

    /// Every extension Formigo reads, for the file importer.
    static var importTypes: [UTType] {
        var types: [UTType] = []
        for ext in sources.keys.sorted() {
            if let t = UTType(filenameExtension: ext), !types.contains(t) { types.append(t) }
        }
        return types + [.item]
    }

    /// Targets whose output the Photos library takes.
    static func isPhotoLibraryImage(_ ext: String) -> Bool {
        ["jpg", "jpeg", "png", "heic", "gif", "tiff", "bmp", "webp", "avif"].contains(ext.lowercased())
    }

    static func isPhotoLibraryVideo(_ ext: String) -> Bool {
        ["mp4", "mov", "m4v"].contains(ext.lowercased())
    }
}

/// The tuning of conversions. Every field has a sensible default.
struct ConvertOptions: Sendable, Equatable {
    /// Quality of lossy pictures, 0.1 … 1.
    var quality: Double = 0.85
    /// Resize factor of pictures and GIFs, 1 keeps size.
    var scale: Double = 1
    /// Bits per sample of PCM sound: 16 or 24.
    var bitDepth: Int = 16
    var mono: Bool = false
    /// Indentation of JSON and YAML; 0 is compact.
    var indent: Int = 2
    /// Video resolution: "original", "1080", "720".
    var videoResolution: String = "original"
}

/// Which options matter for a set of target IDs.
struct OptionSet: Equatable {
    var quality = false
    var scale = false
    var resolution = false
    var bitDepth = false
    var mono = false
    var indent = false

    var any: Bool { quality || scale || resolution || bitDepth || mono || indent }

    /// The options relevant to files of `kind` going to `targets`.
    static func relevant(kind: FileKind, targets: Set<String>) -> OptionSet {
        var o = OptionSet()
        for t in targets {
            switch (kind, t) {
            case (.image, "jpg"), (.image, "heic"), (.image, "webp"), (.image, "avif"), (.image, "pdf"),
                 (.document, "jpg"):
                o.quality = true
                if kind == .image { o.scale = true }
            case (.image, "png"), (.image, "gif"), (.image, "bmp"), (.image, "tiff"),
                 (.document, "png"), (.video, "gif"):
                o.scale = true
            case (.video, "mp4"), (.video, "hevc"), (.video, "mov"):
                o.resolution = true
            case (.video, "m4a"), (.audio, "m4a"):
                o.mono = true
            case (.audio, "wav"), (.audio, "aiff"), (.audio, "caf"), (.audio, "alac"):
                o.bitDepth = true
                o.mono = true
            case (.data, "json"), (.data, "yaml"):
                o.indent = true
            default:
                break
            }
        }
        return o
    }
}

enum ConvertError: LocalizedError {
    case unsupported
    case unreadable
    case noAudioTrack
    case failed(String)

    var errorDescription: String? {
        switch self {
        case .unsupported: "不支持这种转换"
        case .unreadable: "无法读取文件"
        case .noAudioTrack: "没有音轨"
        case .failed(let s): s
        }
    }
}
