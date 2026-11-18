import Foundation
import ZIPFoundation

typealias ProgressHandler = @Sendable (Double) -> Void

/// Routes a file to the converter of its kind and target.
enum Engine {
    /// Converts `input` to the target `targetID`, writing into `directory` a
    /// file named `baseName` with the target's extension. Returns what was
    /// written (a file, or a folder for "unzip").
    static func convert(
        _ input: URL,
        to targetID: String,
        options: ConvertOptions = ConvertOptions(),
        into directory: URL,
        baseName: String,
        progress: @escaping ProgressHandler = { _ in }
    ) async throws -> URL {
        guard let source = Formats.source(for: input),
              let target = Formats.format(targetID),
              Formats.targets(for: source).contains(targetID)
        else { throw ConvertError.unsupported }

        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        let out = target.ext.isEmpty
            ? directory.appendingPathComponent(baseName, isDirectory: true)
            : directory.appendingPathComponent(baseName).appendingPathExtension(target.ext)
        try? FileManager.default.removeItem(at: out)

        if targetID == "zip" {
            try FileManager.default.zipItem(at: input, to: out, shouldKeepParent: false)
            progress(1)
            return out
        }

        switch source.kind {
        case .image:
            if targetID == "pdf" {
                try ImageEngine.pdf(from: input, options: options, to: out)
            } else {
                try ImageEngine.convert(input, to: targetID, options: options, to: out)
            }
        case .video:
            try await VideoEngine.convert(input, to: targetID, options: options, to: out, progress: progress)
        case .audio:
            try AudioEngine.convert(input, to: targetID, options: options, to: out, progress: progress)
        case .data:
            try DataEngine.convert(input, from: source.id, to: targetID, options: options, to: out)
        case .document:
            if source.id == "pdf" {
                return try ImageEngine.fromPDF(input, to: targetID, options: options, to: out, baseName: baseName)
            }
            try await DocEngine.convert(input, from: source.id, to: targetID, to: out)
        case .subtitle:
            try SubtitleEngine.convert(input, from: source.id, to: targetID, to: out)
        case .archive:
            try FileManager.default.createDirectory(at: out, withIntermediateDirectories: true)
            try FileManager.default.unzipItem(at: input, to: out)
        }
        progress(1)
        return out
    }
}

extension String {
    /// Reads a text file in UTF-8, falling back to the encodings files are
    /// commonly in.
    static func readText(_ url: URL) throws -> String {
        let data = try Data(contentsOf: url)
        if let s = String(data: data, encoding: .utf8) { return s.hasPrefix("\u{FEFF}") ? String(s.dropFirst()) : s }
        for enc: String.Encoding in [.utf16, .init(rawValue: CFStringConvertEncodingToNSStringEncoding(CFStringEncoding(CFStringEncodings.GB_18030_2000.rawValue))), .isoLatin1] {
            if let s = String(data: data, encoding: enc) { return s }
        }
        throw ConvertError.unreadable
    }
}
