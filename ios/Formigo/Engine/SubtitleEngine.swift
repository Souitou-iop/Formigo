import Foundation

/// A timed line of a subtitle or lyric file.
struct Cue: Equatable {
    var start: Double
    var end: Double
    var text: String
}

/// SRT, WebVTT, ASS/SSA and LRC, through a list of cues.
enum SubtitleEngine {
    static func convert(_ input: URL, from src: String, to dst: String, to out: URL) throws {
        let cues = try parse(String.readText(input), as: src)
        guard !cues.isEmpty else { throw ConvertError.failed("没有找到字幕") }
        try write(cues, as: dst).write(to: out, atomically: true, encoding: .utf8)
    }

    static func parse(_ text: String, as format: String) throws -> [Cue] {
        let text = text.replacingOccurrences(of: "\r\n", with: "\n").replacingOccurrences(of: "\r", with: "\n")
        switch format {
        case "srt", "vtt": return blocks(text)
        case "ass": return ass(text)
        case "lrc": return lrc(text)
        default: throw ConvertError.unsupported
        }
    }

    static func write(_ cues: [Cue], as format: String) throws -> String {
        switch format {
        case "srt":
            return cues.enumerated().map { i, c in
                "\(i + 1)\n\(stamp(c.start, ",")) --> \(stamp(c.end, ","))\n\(c.text)\n"
            }.joined(separator: "\n")
        case "vtt":
            return "WEBVTT\n\n" + cues.map { c in "\(stamp(c.start, ".")) --> \(stamp(c.end, "."))\n\(c.text)\n" }.joined(separator: "\n")
        case "ass":
            let header = """
            [Script Info]
            ScriptType: v4.00+
            PlayResX: 1920
            PlayResY: 1080

            [V4+ Styles]
            Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
            Style: Default,PingFang SC,64,&H00FFFFFF,&H000000FF,&H00000000,&H64000000,0,0,0,0,100,100,0,0,1,3,1,2,40,40,48,1

            [Events]
            Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text

            """
            return header + cues.map { c in
                "Dialogue: 0,\(assStamp(c.start)),\(assStamp(c.end)),Default,,0,0,0,,\(c.text.replacingOccurrences(of: "\n", with: "\\N"))"
            }.joined(separator: "\n") + "\n"
        case "lrc":
            return cues.map { c in
                let m = Int(c.start) / 60, s = c.start - Double(m * 60)
                return String(format: "[%02d:%05.2f]", m, s) + c.text.replacingOccurrences(of: "\n", with: " ")
            }.joined(separator: "\n") + "\n"
        default:
            throw ConvertError.unsupported
        }
    }

    /// "01:02:03,456", "02:03.456" or "1:02:03.45" in seconds.
    static func seconds(_ s: String) -> Double? {
        let parts = s.trimmingCharacters(in: .whitespaces).replacingOccurrences(of: ",", with: ".").split(separator: ":")
        guard (2...3).contains(parts.count) else { return nil }
        var total = 0.0
        for p in parts {
            guard let v = Double(p) else { return nil }
            total = total * 60 + v
        }
        return total
    }

    static func stamp(_ t: Double, _ sep: String) -> String {
        let ms = Int((t * 1000).rounded())
        return String(format: "%02d:%02d:%02d%@%03d", ms / 3_600_000, ms / 60000 % 60, ms / 1000 % 60, sep, ms % 1000)
    }

    static func assStamp(_ t: Double) -> String {
        let cs = Int((t * 100).rounded())
        return String(format: "%d:%02d:%02d.%02d", cs / 360_000, cs / 6000 % 60, cs / 100 % 60, cs % 100)
    }

    /// SRT and WebVTT: blocks with a "start --> end" line.
    static func blocks(_ text: String) -> [Cue] {
        var cues: [Cue] = []
        for block in text.components(separatedBy: "\n\n") {
            let lines = block.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
            guard let ti = lines.firstIndex(where: { $0.contains("-->") }) else { continue }
            let halves = lines[ti].components(separatedBy: "-->")
            guard halves.count == 2,
                  let a = seconds(halves[0]),
                  let b = seconds(halves[1].trimmingCharacters(in: .whitespaces).split(separator: " ").first.map(String.init) ?? "")
            else { continue }
            let body = lines[(ti + 1)...].joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines)
            if !body.isEmpty { cues.append(Cue(start: a, end: b, text: body)) }
        }
        return cues
    }

    static func ass(_ text: String) -> [Cue] {
        var fields = ["Layer", "Start", "End", "Style", "Name", "MarginL", "MarginR", "MarginV", "Effect", "Text"]
        var cues: [Cue] = []
        var inEvents = false
        for line in text.components(separatedBy: "\n") {
            let t = line.trimmingCharacters(in: .whitespaces)
            if t.hasPrefix("[") { inEvents = t.lowercased() == "[events]"; continue }
            guard inEvents else { continue }
            if t.hasPrefix("Format:") {
                fields = t.dropFirst(7).split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }
            } else if t.hasPrefix("Dialogue:") {
                let values = t.dropFirst(9).split(separator: ",", maxSplits: fields.count - 1, omittingEmptySubsequences: false)
                    .map { $0.trimmingCharacters(in: .whitespaces) }
                guard values.count == fields.count,
                      let si = fields.firstIndex(of: "Start"), let ei = fields.firstIndex(of: "End"),
                      let xi = fields.firstIndex(of: "Text"),
                      let a = seconds(values[si]), let b = seconds(values[ei]) else { continue }
                let body = values[xi].replacing(/\{[^}]*\}/, with: "")
                    .replacingOccurrences(of: "\\N", with: "\n").replacingOccurrences(of: "\\n", with: "\n")
                cues.append(Cue(start: a, end: b, text: body))
            }
        }
        return cues.sorted { $0.start < $1.start }
    }

    static func lrc(_ text: String) -> [Cue] {
        var timed: [(Double, String)] = []
        for line in text.components(separatedBy: "\n") {
            var rest = Substring(line)
            var stamps: [Double] = []
            while let m = rest.prefixMatch(of: /\[(\d+:\d+(?:[.:]\d+)?)\]/) {
                if let s = seconds(String(m.1).replacingOccurrences(of: ":", with: ".", options: [], range: nil).countColons()) {
                    stamps.append(s)
                }
                rest = rest[m.range.upperBound...]
            }
            let body = rest.trimmingCharacters(in: .whitespaces)
            for s in stamps where !body.isEmpty { timed.append((s, body)) }
        }
        timed.sort { $0.0 < $1.0 }
        return timed.enumerated().map { i, t in
            Cue(start: t.0, end: i + 1 < timed.count ? timed[i + 1].0 : t.0 + 4, text: t.1)
        }
    }
}

private extension String {
    /// "01.23.45" from an LRC stamp whose separators were all dots, back to
    /// "01:23.45": minutes, seconds and hundredths.
    func countColons() -> String {
        let parts = split(separator: ".")
        guard parts.count >= 2 else { return self }
        return parts[0] + ":" + parts[1] + (parts.count > 2 ? "." + parts[2] : "")
    }
}
