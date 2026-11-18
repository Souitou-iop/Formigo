import Foundation
import TOMLKit
import Yams

/// A structured value, with the order of an object's keys kept.
indirect enum DataValue: Equatable {
    case null
    case bool(Bool)
    case int(Int)
    case double(Double)
    case string(String)
    case array([DataValue])
    case object([(String, DataValue)])

    static func == (a: DataValue, b: DataValue) -> Bool {
        switch (a, b) {
        case (.null, .null): true
        case let (.bool(x), .bool(y)): x == y
        case let (.int(x), .int(y)): x == y
        case let (.double(x), .double(y)): x == y
        case let (.int(x), .double(y)), let (.double(y), .int(x)): Double(x) == y
        case let (.string(x), .string(y)): x == y
        case let (.array(x), .array(y)): x == y
        case let (.object(x), .object(y)):
            x.count == y.count && zip(x, y).allSatisfy { $0.0 == $1.0 && $0.1 == $1.1 }
        default: false
        }
    }

    subscript(key: String) -> DataValue? {
        if case .object(let pairs) = self { return pairs.first { $0.0 == key }?.1 }
        return nil
    }

    var isScalar: Bool {
        switch self {
        case .array, .object: false
        default: true
        }
    }

    /// The value as text in a table cell.
    var cellText: String {
        switch self {
        case .null: ""
        case .bool(let b): b ? "true" : "false"
        case .int(let i): String(i)
        case .double(let d): JSONCodec.number(d)
        case .string(let s): s
        default: JSONCodec.encode(self, indent: 0)
        }
    }
}

enum DataEngine {
    static func convert(_ input: URL, from src: String, to dst: String, options: ConvertOptions, to out: URL) throws {
        let value: DataValue
        if src == "plist" {
            value = try PlistCodec.decode(Data(contentsOf: input))
        } else {
            value = try decode(String.readText(input), as: src)
        }
        if dst == "plist" {
            try PlistCodec.encode(value).write(to: out)
        } else {
            try encode(value, as: dst, indent: options.indent).write(to: out, atomically: true, encoding: .utf8)
        }
    }

    static func decode(_ text: String, as format: String) throws -> DataValue {
        do {
            switch format {
            case "json": return try JSONCodec.decode(text)
            case "yaml": return try YAMLCodec.decode(text)
            case "toml": return try JSONCodec.decode(TOMLTable(string: text).convert(to: .json))
            case "csv": return CSVCodec.decode(text, separator: ",")
            case "tsv": return CSVCodec.decode(text, separator: "\t")
            case "xml": return try XMLCodec.decode(text)
            case "plist": return try PlistCodec.decode(Data(text.utf8))
            default: throw ConvertError.unsupported
            }
        } catch let e as ConvertError {
            throw e
        } catch {
            throw ConvertError.failed("\(format.uppercased()) 解析失败")
        }
    }

    static func encode(_ value: DataValue, as format: String, indent: Int = 2) throws -> String {
        switch format {
        case "json": return JSONCodec.encode(value, indent: indent) + "\n"
        case "yaml": return try YAMLCodec.encode(value, indent: indent == 0 ? 2 : indent)
        case "toml": return TOMLCodec.encode(value)
        case "csv": return CSVCodec.encode(value, separator: ",")
        case "tsv": return CSVCodec.encode(value, separator: "\t")
        case "xml": return XMLCodec.encode(value)
        case "plist": return String(decoding: try PlistCodec.encode(value), as: UTF8.self)
        default: throw ConvertError.unsupported
        }
    }
}

// MARK: - JSON

enum JSONCodec {
    struct ParseError: Error {}

    static func decode(_ text: String) throws -> DataValue {
        var p = Parser(bytes: Array(text.utf8))
        p.skip()
        let v = try p.value()
        p.skip()
        guard p.i == p.bytes.count else { throw ParseError() }
        return v
    }

    struct Parser {
        let bytes: [UInt8]
        var i = 0

        mutating func skip() {
            while i < bytes.count, [0x20, 0x09, 0x0A, 0x0D].contains(bytes[i]) { i += 1 }
        }

        mutating func expect(_ s: String) throws {
            for b in s.utf8 {
                guard i < bytes.count, bytes[i] == b else { throw ParseError() }
                i += 1
            }
        }

        mutating func value() throws -> DataValue {
            guard i < bytes.count else { throw ParseError() }
            switch bytes[i] {
            case UInt8(ascii: "{"):
                i += 1
                var pairs: [(String, DataValue)] = []
                skip()
                if i < bytes.count, bytes[i] == UInt8(ascii: "}") { i += 1; return .object(pairs) }
                while true {
                    skip()
                    let k = try string()
                    skip()
                    try expect(":")
                    skip()
                    pairs.append((k, try value()))
                    skip()
                    guard i < bytes.count else { throw ParseError() }
                    if bytes[i] == UInt8(ascii: ",") { i += 1; continue }
                    if bytes[i] == UInt8(ascii: "}") { i += 1; return .object(pairs) }
                    throw ParseError()
                }
            case UInt8(ascii: "["):
                i += 1
                var items: [DataValue] = []
                skip()
                if i < bytes.count, bytes[i] == UInt8(ascii: "]") { i += 1; return .array(items) }
                while true {
                    skip()
                    items.append(try value())
                    skip()
                    guard i < bytes.count else { throw ParseError() }
                    if bytes[i] == UInt8(ascii: ",") { i += 1; continue }
                    if bytes[i] == UInt8(ascii: "]") { i += 1; return .array(items) }
                    throw ParseError()
                }
            case UInt8(ascii: "\""):
                return .string(try string())
            case UInt8(ascii: "t"): try expect("true"); return .bool(true)
            case UInt8(ascii: "f"): try expect("false"); return .bool(false)
            case UInt8(ascii: "n"): try expect("null"); return .null
            default:
                return try number()
            }
        }

        mutating func number() throws -> DataValue {
            let start = i
            var isInt = true
            while i < bytes.count {
                let b = bytes[i]
                if (b >= 0x30 && b <= 0x39) || b == UInt8(ascii: "-") || b == UInt8(ascii: "+") {
                    i += 1
                } else if b == UInt8(ascii: ".") || b == UInt8(ascii: "e") || b == UInt8(ascii: "E") {
                    isInt = false
                    i += 1
                } else { break }
            }
            let s = String(decoding: bytes[start..<i], as: UTF8.self)
            if isInt, let n = Int(s) { return .int(n) }
            guard let d = Double(s) else { throw ParseError() }
            return .double(d)
        }

        mutating func string() throws -> String {
            try expect("\"")
            var out = [UInt8]()
            while i < bytes.count {
                let b = bytes[i]
                i += 1
                if b == UInt8(ascii: "\"") { return String(decoding: out, as: UTF8.self) }
                if b != UInt8(ascii: "\\") { out.append(b); continue }
                guard i < bytes.count else { throw ParseError() }
                let e = bytes[i]
                i += 1
                switch e {
                case UInt8(ascii: "n"): out.append(0x0A)
                case UInt8(ascii: "t"): out.append(0x09)
                case UInt8(ascii: "r"): out.append(0x0D)
                case UInt8(ascii: "b"): out.append(0x08)
                case UInt8(ascii: "f"): out.append(0x0C)
                case UInt8(ascii: "u"):
                    var code = try hex4()
                    if code >= 0xD800, code < 0xDC00, i + 1 < bytes.count, bytes[i] == UInt8(ascii: "\\"), bytes[i + 1] == UInt8(ascii: "u") {
                        i += 2
                        let low = try hex4()
                        code = 0x10000 + ((code - 0xD800) << 10) + (low - 0xDC00)
                    }
                    out.append(contentsOf: Array(String(UnicodeScalar(code).map(Character.init) ?? "\u{FFFD}").utf8))
                default: out.append(e)
                }
            }
            throw ParseError()
        }

        mutating func hex4() throws -> UInt32 {
            guard i + 4 <= bytes.count, let v = UInt32(String(decoding: bytes[i..<i + 4], as: UTF8.self), radix: 16) else {
                throw ParseError()
            }
            i += 4
            return v
        }
    }

    static func number(_ d: Double) -> String {
        if d.isNaN || d.isInfinite { return "null" }
        if d == d.rounded(), abs(d) < 1e15 { return String(format: "%.1f", d) }
        return String(d)
    }

    static func quote(_ s: String) -> String {
        var out = "\""
        for u in s.unicodeScalars {
            switch u {
            case "\"": out += "\\\""
            case "\\": out += "\\\\"
            case "\n": out += "\\n"
            case "\r": out += "\\r"
            case "\t": out += "\\t"
            default:
                if u.value < 0x20 { out += String(format: "\\u%04x", u.value) } else { out.unicodeScalars.append(u) }
            }
        }
        return out + "\""
    }

    static func encode(_ v: DataValue, indent: Int, level: Int = 0) -> String {
        let pad = indent > 0 ? "\n" + String(repeating: " ", count: indent * (level + 1)) : ""
        let end = indent > 0 ? "\n" + String(repeating: " ", count: indent * level) : ""
        let sep = indent > 0 ? ": " : ":"
        switch v {
        case .null: return "null"
        case .bool(let b): return b ? "true" : "false"
        case .int(let i): return String(i)
        case .double(let d): return number(d)
        case .string(let s): return quote(s)
        case .array(let a):
            if a.isEmpty { return "[]" }
            return "[" + a.map { pad + encode($0, indent: indent, level: level + 1) }.joined(separator: ",") + end + "]"
        case .object(let o):
            if o.isEmpty { return "{}" }
            return "{" + o.map { pad + quote($0.0) + sep + encode($0.1, indent: indent, level: level + 1) }.joined(separator: ",") + end + "}"
        }
    }
}

// MARK: - YAML

enum YAMLCodec {
    static func decode(_ text: String) throws -> DataValue {
        guard let node = try Yams.compose(yaml: text) else { return .null }
        return value(node)
    }

    static func value(_ node: Node) -> DataValue {
        switch node {
        case .scalar(let s):
            switch Tag.Name(rawValue: node.tag.rawValue) {
            case .null: return .null
            case .bool: return .bool(node.bool ?? false)
            case .int: return node.int.map(DataValue.int) ?? .string(s.string)
            case .float: return node.float.map(DataValue.double) ?? .string(s.string)
            default: return .string(s.string)
            }
        case .mapping(let m):
            return .object(m.map { (k, v) in (k.string ?? "\(k)", value(v)) })
        case .sequence(let seq):
            return .array(seq.map(value))
        case .alias:
            return .null
        }
    }

    static func needsQuotes(_ s: String) -> Bool {
        let lower = s.lowercased()
        if s.isEmpty || Double(s) != nil || Int(s) != nil { return true }
        if ["true", "false", "yes", "no", "on", "off", "null", "~", "y", "n", ".inf", "-.inf", ".nan"].contains(lower) { return true }
        return false
    }

    static func node(_ v: DataValue) -> Node {
        switch v {
        case .null: return Node("null", .implicit)
        case .bool(let b): return Node(b ? "true" : "false", .implicit)
        case .int(let i): return Node(String(i), .implicit)
        case .double(let d): return Node(d.isNaN ? ".nan" : d.isInfinite ? (d > 0 ? ".inf" : "-.inf") : String(d), .implicit)
        case .string(let s): return Node(s, .implicit, needsQuotes(s) ? .doubleQuoted : .any)
        case .array(let a): return Node(a.map(node), .implicit)
        case .object(let o): return Node(o.map { (Node($0.0, .implicit, needsQuotes($0.0) ? .doubleQuoted : .any), node($0.1)) }, .implicit)
        }
    }

    static func encode(_ v: DataValue, indent: Int) throws -> String {
        try Yams.serialize(node: node(v), indent: indent, width: -1, allowUnicode: true)
    }
}

// MARK: - TOML

enum TOMLCodec {
    static func key(_ k: String) -> String {
        let bare = !k.isEmpty && k.unicodeScalars.allSatisfy { CharacterSet.alphanumerics.contains($0) && $0.isASCII || $0 == "_" || $0 == "-" }
        return bare ? k : JSONCodec.quote(k)
    }

    static func inline(_ v: DataValue) -> String? {
        switch v {
        case .null: return nil
        case .bool(let b): return b ? "true" : "false"
        case .int(let i): return String(i)
        case .double(let d): return d.isNaN ? "nan" : d.isInfinite ? (d > 0 ? "inf" : "-inf") : JSONCodec.number(d)
        case .string(let s): return JSONCodec.quote(s)
        case .array(let a): return "[" + a.compactMap(inline).joined(separator: ", ") + "]"
        case .object(let o): return "{ " + o.compactMap { p in inline(p.1).map { key(p.0) + " = " + $0 } }.joined(separator: ", ") + " }"
        }
    }

    static func isTableArray(_ v: DataValue) -> Bool {
        if case .array(let a) = v, !a.isEmpty {
            return a.allSatisfy { if case .object = $0 { return true } else { return false } }
        }
        return false
    }

    static func encode(_ v: DataValue) -> String {
        var out = ""
        let root: [(String, DataValue)]
        switch v {
        case .object(let o): root = o
        default: root = [("items", v)]
        }
        table(root, path: [], into: &out)
        return out.trimmingCharacters(in: .newlines) + "\n"
    }

    static func table(_ pairs: [(String, DataValue)], path: [String], into out: inout String) {
        for (k, v) in pairs {
            if case .object = v { continue }
            if isTableArray(v) { continue }
            if let s = inline(v) { out += key(k) + " = " + s + "\n" }
        }
        for (k, v) in pairs {
            let p = path + [k]
            let name = p.map(key).joined(separator: ".")
            if case .object(let o) = v {
                out += "\n[\(name)]\n"
                table(o, path: p, into: &out)
            } else if isTableArray(v), case .array(let a) = v {
                for item in a {
                    if case .object(let o) = item {
                        out += "\n[[\(name)]]\n"
                        table(o, path: p, into: &out)
                    }
                }
            }
        }
    }
}

// MARK: - CSV / TSV

enum CSVCodec {
    static func rows(_ text: String, separator: Character) -> [[String]] {
        var rows: [[String]] = []
        var row: [String] = []
        var field = ""
        var quoted = false
        var chars = Array(text.replacingOccurrences(of: "\r\n", with: "\n"))
        if chars.last != "\n" { chars.append("\n") }
        var i = 0
        while i < chars.count {
            let c = chars[i]
            if quoted {
                if c == "\"" {
                    if i + 1 < chars.count, chars[i + 1] == "\"" { field.append("\""); i += 1 } else { quoted = false }
                } else { field.append(c) }
            } else if c == "\"" && field.isEmpty {
                quoted = true
            } else if c == separator {
                row.append(field); field = ""
            } else if c == "\n" || c == "\r" {
                row.append(field); field = ""
                if !(row.count == 1 && row[0].isEmpty) { rows.append(row) }
                row = []
            } else {
                field.append(c)
            }
            i += 1
        }
        return rows
    }

    static func infer(_ s: String) -> DataValue {
        if let i = Int(s) { return .int(i) }
        if let d = Double(s), s.contains(".") { return .double(d) }
        switch s.lowercased() {
        case "true": return .bool(true)
        case "false": return .bool(false)
        default: return .string(s)
        }
    }

    static func decode(_ text: String, separator: Character) -> DataValue {
        let all = rows(text, separator: separator)
        guard let header = all.first else { return .array([]) }
        return .array(all.dropFirst().map { r in
            .object(header.enumerated().map { (i, h) in (h, i < r.count ? infer(r[i]) : .string("")) })
        })
    }

    /// The rows a value lays out as: a list of objects, a list of lists, an
    /// object holding one list, or an object as a single row.
    static func table(_ v: DataValue) -> (header: [String], rows: [[DataValue]]) {
        switch v {
        case .array(let a):
            if a.allSatisfy({ if case .object = $0 { return true }; return false }) {
                var header: [String] = []
                for case .object(let o) in a { for (k, _) in o where !header.contains(k) { header.append(k) } }
                return (header, a.map { item in header.map { item[$0] ?? .null } })
            }
            if a.allSatisfy({ if case .array = $0 { return true }; return false }) {
                let rows = a.map { if case .array(let r) = $0 { return r }; return [] }
                guard let first = rows.first else { return ([], []) }
                return (first.map(\.cellText), Array(rows.dropFirst()))
            }
            return (["value"], a.map { [$0] })
        case .object(let o):
            let lists = o.filter { if case .array = $0.1 { return true }; return false }
            if o.count == 1, let only = lists.first { return table(only.1) }
            return (o.map(\.0), [o.map(\.1)])
        default:
            return (["value"], [[v]])
        }
    }

    static func field(_ s: String, separator: Character) -> String {
        if s.contains(separator) || s.contains("\"") || s.contains("\n") || s.contains("\r") {
            return "\"" + s.replacingOccurrences(of: "\"", with: "\"\"") + "\""
        }
        return s
    }

    static func encode(_ v: DataValue, separator: Character) -> String {
        let t = table(v)
        let sep = String(separator)
        var lines = [t.header.map { field($0, separator: separator) }.joined(separator: sep)]
        for r in t.rows { lines.append(r.map { field($0.cellText, separator: separator) }.joined(separator: sep)) }
        return lines.joined(separator: "\n") + "\n"
    }
}

// MARK: - XML

enum XMLCodec {
    final class Element {
        let name: String
        var attributes: [(String, String)] = []
        var children: [Element] = []
        var text = ""
        init(name: String) { self.name = name }
    }

    final class Builder: NSObject, XMLParserDelegate {
        var stack: [Element] = []
        var root: Element?

        func parser(_ parser: XMLParser, didStartElement name: String, namespaceURI: String?, qualifiedName: String?,
                    attributes: [String: String]) {
            let e = Element(name: name)
            e.attributes = attributes.sorted { $0.key < $1.key }.map { ($0.key, $0.value) }
            stack.last?.children.append(e)
            stack.append(e)
        }

        func parser(_ parser: XMLParser, foundCharacters string: String) { stack.last?.text += string }

        func parser(_ parser: XMLParser, foundCDATA data: Data) { stack.last?.text += String(decoding: data, as: UTF8.self) }

        func parser(_ parser: XMLParser, didEndElement name: String, namespaceURI: String?, qualifiedName: String?) {
            let e = stack.removeLast()
            if stack.isEmpty { root = e }
        }
    }

    static func decode(_ text: String) throws -> DataValue {
        let parser = XMLParser(data: Data(text.utf8))
        let b = Builder()
        parser.delegate = b
        guard parser.parse(), let root = b.root else { throw ConvertError.failed("XML 解析失败") }
        return .object([(root.name, value(root))])
    }

    static func value(_ e: Element) -> DataValue {
        let text = e.text.trimmingCharacters(in: .whitespacesAndNewlines)
        if e.attributes.isEmpty && e.children.isEmpty { return CSVCodec.infer(text) }
        var pairs: [(String, DataValue)] = e.attributes.map { ("@" + $0.0, .string($0.1)) }
        var order: [String] = []
        var groups: [String: [DataValue]] = [:]
        for c in e.children {
            if groups[c.name] == nil { order.append(c.name) }
            groups[c.name, default: []].append(value(c))
        }
        for n in order {
            let g = groups[n]!
            pairs.append((n, g.count == 1 ? g[0] : .array(g)))
        }
        if !text.isEmpty { pairs.append(("#text", .string(text))) }
        return .object(pairs)
    }

    static func escape(_ s: String, attribute: Bool = false) -> String {
        var r = s.replacingOccurrences(of: "&", with: "&amp;").replacingOccurrences(of: "<", with: "&lt;")
            .replacingOccurrences(of: ">", with: "&gt;")
        if attribute { r = r.replacingOccurrences(of: "\"", with: "&quot;") }
        return r
    }

    static func name(_ s: String) -> String {
        var n = String(s.map { $0.isLetter || $0.isNumber || "_-.".contains($0) ? $0 : "_" })
        if n.isEmpty || n.first!.isNumber || n.first == "-" || n.first == "." { n = "_" + n }
        return n
    }

    static func encode(_ v: DataValue) -> String {
        var out = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"
        if case .object(let o) = v, o.count == 1, !o[0].0.hasPrefix("@"), !isList(o[0].1) {
            element(o[0].0, o[0].1, level: 0, into: &out)
        } else {
            element("root", v, level: 0, into: &out)
        }
        return out
    }

    static func isList(_ v: DataValue) -> Bool { if case .array = v { return true }; return false }

    static func element(_ tag: String, _ v: DataValue, level: Int, into out: inout String) {
        let pad = String(repeating: "  ", count: level)
        let tag = name(tag)
        switch v {
        case .array(let a):
            for item in a { element(tag == "root" ? "item" : tag, item, level: level, into: &out) }
        case .object(let o):
            let attrs = o.filter { $0.0.hasPrefix("@") && $0.1.isScalar }
            let text = o.first { $0.0 == "#text" }?.1
            let kids = o.filter { !$0.0.hasPrefix("@") && $0.0 != "#text" }
            var open = "<" + tag
            for (k, val) in attrs { open += " \(name(String(k.dropFirst())))=\"\(escape(val.cellText, attribute: true))\"" }
            if kids.isEmpty {
                if let t = text { out += pad + open + ">" + escape(t.cellText) + "</\(tag)>\n" } else { out += pad + open + "/>\n" }
                return
            }
            out += pad + open + ">\n"
            if let t = text { out += pad + "  " + escape(t.cellText) + "\n" }
            for (k, val) in kids {
                if case .array(let a) = val {
                    for item in a { element(k, item, level: level + 1, into: &out) }
                } else {
                    element(k, val, level: level + 1, into: &out)
                }
            }
            out += pad + "</\(tag)>\n"
        case .null:
            out += pad + "<\(tag)/>\n"
        default:
            out += pad + "<\(tag)>" + escape(v.cellText) + "</\(tag)>\n"
        }
    }
}

// MARK: - Property lists

enum PlistCodec {
    static func decode(_ data: Data) throws -> DataValue {
        do {
            return value(try PropertyListSerialization.propertyList(from: data, format: nil))
        } catch {
            throw ConvertError.failed("PLIST 解析失败")
        }
    }

    static func value(_ any: Any) -> DataValue {
        switch any {
        case let d as [String: Any]: return .object(d.keys.sorted().map { ($0, value(d[$0]!)) })
        case let a as [Any]: return .array(a.map(value))
        case let s as String: return .string(s)
        case let date as Date: return .string(ISO8601DateFormatter().string(from: date))
        case let data as Data: return .string(data.base64EncodedString())
        case let n as NSNumber:
            if CFGetTypeID(n) == CFBooleanGetTypeID() { return .bool(n.boolValue) }
            if CFNumberIsFloatType(n) { return .double(n.doubleValue) }
            return .int(n.intValue)
        default: return .string("\(any)")
        }
    }

    static func object(_ v: DataValue) -> Any? {
        switch v {
        case .null: return nil
        case .bool(let b): return b
        case .int(let i): return i
        case .double(let d): return d
        case .string(let s): return s
        case .array(let a): return a.compactMap(object)
        case .object(let o):
            var d: [String: Any] = [:]
            for (k, val) in o { if let x = object(val) { d[k] = x } }
            return d
        }
    }

    static func encode(_ v: DataValue) throws -> Data {
        var root = object(v) ?? [String: Any]()
        if !(root is [String: Any]) && !(root is [Any]) { root = ["value": root] }
        return try PropertyListSerialization.data(fromPropertyList: root, format: .xml, options: 0)
    }
}
