import Foundation
import UIKit

/// Documents: Markdown, HTML, plain text and RTF, through NSAttributedString,
/// and PDF through the print system.
enum DocEngine {
    static func convert(_ input: URL, from src: String, to dst: String, to out: URL) async throws {
        switch (src, dst) {
        case ("md", "html"):
            try html(document: Markdown.html(try String.readText(input)), title: input.deletingPathExtension().lastPathComponent)
                .write(to: out, atomically: true, encoding: .utf8)
        case ("md", "txt"):
            let attr = try await attributed(html: Markdown.html(String.readText(input)))
            try attr.string.write(to: out, atomically: true, encoding: .utf8)
        case ("txt", "md"):
            try String.readText(input).write(to: out, atomically: true, encoding: .utf8)
        case ("txt", "html"):
            try html(document: textHTML(String.readText(input)), title: input.deletingPathExtension().lastPathComponent)
                .write(to: out, atomically: true, encoding: .utf8)
        default:
            switch dst {
            case "pdf":
                let source = try await bodyHTML(input, src: src)
                try await pdf(html: source).write(to: out)
            case "html":
                let attr = try await attributed(file: input, src: src)
                try await exportHTML(attr).write(to: out)
            case "txt":
                try await attributed(file: input, src: src).string.write(to: out, atomically: true, encoding: .utf8)
            case "md":
                try await markdown(attributed(file: input, src: src)).write(to: out, atomically: true, encoding: .utf8)
            case "rtf":
                let attr = try await attributed(file: input, src: src)
                let data = try attr.data(from: NSRange(location: 0, length: attr.length),
                                         documentAttributes: [.documentType: NSAttributedString.DocumentType.rtf])
                try data.write(to: out)
            default:
                throw ConvertError.unsupported
            }
        }
    }

    // MARK: Reading

    /// A document's body as HTML, for printing.
    static func bodyHTML(_ input: URL, src: String) async throws -> String {
        switch src {
        case "md": return html(document: Markdown.html(try String.readText(input)))
        case "html": return try String.readText(input)
        case "txt": return html(document: textHTML(try String.readText(input)))
        case "rtf":
            let attr = try await attributed(file: input, src: src)
            return String(decoding: try await exportHTML(attr), as: UTF8.self)
        default: throw ConvertError.unsupported
        }
    }

    static func attributed(file: URL, src: String) async throws -> NSAttributedString {
        switch src {
        case "rtf":
            return try NSAttributedString(url: file, options: [.documentType: NSAttributedString.DocumentType.rtf], documentAttributes: nil)
        case "txt":
            return NSAttributedString(string: try String.readText(file), attributes: [.font: UIFont.systemFont(ofSize: 14)])
        case "md":
            return try await attributed(html: html(document: Markdown.html(String.readText(file))))
        default:
            return try await attributed(html: String.readText(file))
        }
    }

    /// HTML importing goes through WebKit, which wants the main thread.
    @MainActor
    static func attributed(html: String) throws -> NSAttributedString {
        try NSAttributedString(data: Data(html.utf8), options: [
            .documentType: NSAttributedString.DocumentType.html,
            .characterEncoding: String.Encoding.utf8.rawValue,
        ], documentAttributes: nil)
    }

    @MainActor
    static func exportHTML(_ attr: NSAttributedString) throws -> Data {
        try attr.data(from: NSRange(location: 0, length: attr.length),
                      documentAttributes: [.documentType: NSAttributedString.DocumentType.html])
    }

    // MARK: Writing

    static func escape(_ s: String) -> String {
        s.replacingOccurrences(of: "&", with: "&amp;").replacingOccurrences(of: "<", with: "&lt;")
            .replacingOccurrences(of: ">", with: "&gt;")
    }

    static func textHTML(_ text: String) -> String {
        text.components(separatedBy: "\n\n").map { p in
            "<p>" + escape(p).replacingOccurrences(of: "\n", with: "<br>") + "</p>"
        }.joined(separator: "\n")
    }

    static func html(document body: String, title: String = "") -> String {
        """
        <!doctype html>
        <html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
        <title>\(escape(title))</title>
        <style>
        body{font-family:-apple-system,"PingFang SC",sans-serif;font-size:15px;line-height:1.6;max-width:760px;margin:2em auto;padding:0 1em;color:#14213d}
        pre{background:#f2f6fc;padding:12px;border-radius:8px;overflow:auto}code{font-family:ui-monospace,Menlo,monospace}
        blockquote{border-left:3px solid #5cb9f7;margin-left:0;padding-left:1em;color:#56627e}
        table{border-collapse:collapse}td,th{border:1px solid #ccd;padding:4px 8px}
        </style></head><body>
        \(body)
        </body></html>
        """
    }

    /// HTML printed onto A4 pages.
    @MainActor
    static func pdf(html: String) throws -> Data {
        let formatter = UIMarkupTextPrintFormatter(markupText: html)
        let renderer = UIPrintPageRenderer()
        renderer.addPrintFormatter(formatter, startingAtPageAt: 0)
        let page = CGRect(x: 0, y: 0, width: 595.2, height: 841.8)
        renderer.setValue(page, forKey: "paperRect")
        renderer.setValue(page.insetBy(dx: 48, dy: 54), forKey: "printableRect")
        let data = NSMutableData()
        UIGraphicsBeginPDFContextToData(data, page, [kCGPDFContextCreator as String: "Formigo"])
        let pages = max(1, renderer.numberOfPages)
        renderer.prepare(forDrawingPages: NSRange(location: 0, length: pages))
        for i in 0..<pages {
            UIGraphicsBeginPDFPage()
            renderer.drawPage(at: i, in: UIGraphicsGetPDFContextBounds())
        }
        UIGraphicsEndPDFContext()
        return data as Data
    }

    /// Rich text as Markdown: headings from type size, bold, italics, links and lists.
    static func markdown(_ attr: NSAttributedString) -> String {
        let ns = attr.string as NSString
        var base: CGFloat = 0
        var sizes: [CGFloat: Int] = [:]
        attr.enumerateAttribute(.font, in: NSRange(location: 0, length: attr.length)) { v, r, _ in
            if let f = v as? UIFont { sizes[f.pointSize, default: 0] += r.length }
        }
        base = sizes.max { $0.value < $1.value }?.key ?? 14
        var out: [String] = []
        ns.enumerateSubstrings(in: NSRange(location: 0, length: ns.length), options: .byParagraphs) { _, pr, _, _ in
            guard pr.length > 0 else { out.append(""); return }
            var line = ""
            var largest: CGFloat = 0
            var allBold = true
            attr.enumerateAttributes(in: pr) { a, r, _ in
                var s = ns.substring(with: r)
                let font = a[.font] as? UIFont
                largest = max(largest, font?.pointSize ?? base)
                let traits = font?.fontDescriptor.symbolicTraits ?? []
                let bold = traits.contains(.traitBold)
                if !bold && !s.trimmingCharacters(in: .whitespaces).isEmpty { allBold = false }
                let trimmed = s.trimmingCharacters(in: .whitespaces)
                if !trimmed.isEmpty {
                    if traits.contains(.traitItalic) { s = s.replacingOccurrences(of: trimmed, with: "*\(trimmed)*") }
                    if bold { s = s.replacingOccurrences(of: trimmed, with: "**\(trimmed)**") }
                    if let link = a[.link] {
                        let url = (link as? URL)?.absoluteString ?? "\(link)"
                        s = "[\(s)](\(url))"
                    }
                }
                line += s
            }
            let text = ns.substring(with: pr).trimmingCharacters(in: .whitespaces)
            let style = attr.attribute(.paragraphStyle, at: pr.location, effectiveRange: nil) as? NSParagraphStyle
            if let list = style?.textLists.last {
                let bullet = list.markerFormat.rawValue.contains("decimal") ? "1." : "-"
                let cleaned = line.replacingOccurrences(of: "^\\s*([•◦▪\\-\\*]|\\d+[.)])\\s*", with: "", options: .regularExpression)
                line = String(repeating: "  ", count: max(0, (style?.textLists.count ?? 1) - 1)) + bullet + " " + cleaned
            } else if largest >= base * 1.6 {
                line = "# " + text
            } else if largest >= base * 1.3 {
                line = "## " + text
            } else if largest >= base * 1.12 || (allBold && largest > base) {
                line = "### " + text
            }
            out.append(line)
        }
        // Paragraphs apart, list items together.
        var result = ""
        for (i, l) in out.enumerated() where !l.isEmpty {
            let isItem = l.hasPrefix("- ") || l.hasPrefix("1. ") || l.hasPrefix("  ")
            let prevItem = i > 0 && (out[i - 1].hasPrefix("- ") || out[i - 1].hasPrefix("1. "))
            if !result.isEmpty { result += (isItem && prevItem) ? "\n" : "\n\n" }
            result += l
        }
        return result + "\n"
    }
}

/// A small CommonMark subset to HTML: headings, paragraphs, lists, quotes,
/// fenced code, rules, tables, emphasis, code, links and images.
enum Markdown {
    static func html(_ md: String) -> String {
        let lines = md.replacingOccurrences(of: "\r\n", with: "\n").components(separatedBy: "\n")
        var out: [String] = []
        var para: [String] = []
        var list: String?
        var i = 0

        func flush() {
            if !para.isEmpty { out.append("<p>" + inline(para.joined(separator: "\n")) + "</p>"); para = [] }
        }
        func closeList() {
            if let l = list { out.append("</\(l)>"); list = nil }
        }

        while i < lines.count {
            let line = lines[i]
            let t = line.trimmingCharacters(in: .whitespaces)
            if t.hasPrefix("```") || t.hasPrefix("~~~") {
                flush(); closeList()
                let fence = String(t.prefix(3))
                let lang = t.dropFirst(3).trimmingCharacters(in: .whitespaces)
                var code: [String] = []
                i += 1
                while i < lines.count, !lines[i].trimmingCharacters(in: .whitespaces).hasPrefix(fence) { code.append(lines[i]); i += 1 }
                let cls = lang.isEmpty ? "" : " class=\"language-\(DocEngine.escape(lang))\""
                out.append("<pre><code\(cls)>" + DocEngine.escape(code.joined(separator: "\n")) + "</code></pre>")
                i += 1
                continue
            }
            if t.isEmpty { flush(); closeList(); i += 1; continue }
            if let m = t.firstMatch(of: /^(#{1,6})\s+(.*?)\s*#*$/) {
                flush(); closeList()
                let n = m.1.count
                out.append("<h\(n)>" + inline(String(m.2)) + "</h\(n)>")
                i += 1; continue
            }
            if t.wholeMatch(of: /^([-*_])(\s*\1){2,}$/) != nil {
                flush(); closeList(); out.append("<hr>"); i += 1; continue
            }
            if t.hasPrefix(">") {
                flush(); closeList()
                var q: [String] = []
                while i < lines.count, lines[i].trimmingCharacters(in: .whitespaces).hasPrefix(">") {
                    q.append(String(lines[i].trimmingCharacters(in: .whitespaces).dropFirst()).trimmingCharacters(in: .whitespaces))
                    i += 1
                }
                out.append("<blockquote>" + html(q.joined(separator: "\n")) + "</blockquote>")
                continue
            }
            if t.hasPrefix("|"), i + 1 < lines.count, lines[i + 1].contains("---") {
                flush(); closeList()
                func cells(_ s: String) -> [String] {
                    s.trimmingCharacters(in: .whitespaces).trimmingCharacters(in: CharacterSet(charactersIn: "|"))
                        .components(separatedBy: "|").map { $0.trimmingCharacters(in: .whitespaces) }
                }
                var rows = "<table><thead><tr>" + cells(t).map { "<th>\(inline($0))</th>" }.joined() + "</tr></thead><tbody>"
                i += 2
                while i < lines.count, lines[i].trimmingCharacters(in: .whitespaces).hasPrefix("|") {
                    rows += "<tr>" + cells(lines[i]).map { "<td>\(inline($0))</td>" }.joined() + "</tr>"
                    i += 1
                }
                out.append(rows + "</tbody></table>")
                continue
            }
            if let m = t.firstMatch(of: /^([-*+]|\d+[.)])\s+(.*)$/) {
                flush()
                let kind = m.1.first!.isNumber ? "ol" : "ul"
                if list != kind { closeList(); out.append("<\(kind)>"); list = kind }
                var item = String(m.2)
                if item.hasPrefix("[ ] ") { item = "☐ " + item.dropFirst(4) }
                if item.lowercased().hasPrefix("[x] ") { item = "☑ " + item.dropFirst(4) }
                out.append("<li>" + inline(item) + "</li>")
                i += 1; continue
            }
            closeList()
            para.append(t)
            i += 1
        }
        flush(); closeList()
        return out.joined(separator: "\n")
    }

    static func inline(_ s: String) -> String {
        var codes: [String] = []
        var r = s.replacing(/`([^`]+)`/) { m in
            codes.append("<code>" + DocEngine.escape(String(m.1)) + "</code>")
            return "\u{0}\(codes.count - 1)\u{0}"
        }
        r = DocEngine.escape(r)
        r = r.replacing(/!\[([^\]]*)\]\(([^)\s]+)\)/) { "<img alt=\"\($0.1)\" src=\"\($0.2)\">" }
        r = r.replacing(/\[([^\]]+)\]\(([^)\s]+)\)/) { "<a href=\"\($0.2)\">\($0.1)</a>" }
        r = r.replacing(/\*\*(.+?)\*\*|__(.+?)__/) { "<strong>\($0.1 ?? $0.2 ?? "")</strong>" }
        r = r.replacing(/\*(.+?)\*|\b_(.+?)_\b/) { "<em>\($0.1 ?? $0.2 ?? "")</em>" }
        r = r.replacing(/~~(.+?)~~/) { "<del>\($0.1)</del>" }
        r = r.replacingOccurrences(of: "  \n", with: "<br>\n")
        for (i, c) in codes.enumerated() { r = r.replacingOccurrences(of: "\u{0}\(i)\u{0}", with: c) }
        return r
    }
}
