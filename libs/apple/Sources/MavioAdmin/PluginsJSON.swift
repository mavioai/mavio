#if os(macOS)
    import Foundation

    /// A JSON value as a plugin's configuration and its schema hold it, its
    /// objects' keys kept in their order, as the web UI's form draws them.
    enum PluginsJSON: Equatable, Sendable {
        case null
        case bool(Bool)
        case number(Double)
        case string(String)
        case array([PluginsJSON])
        case object(PluginsJSONObject)

        /// The value of a text, nil when it is not JSON.
        init?(parsing text: String) {
            var parser = Parser(bytes: Array(text.utf8))
            guard let value = try? parser.value() else { return nil }
            parser.skipSpace()
            guard parser.index == parser.bytes.count else { return nil }
            self = value
        }

        /// The object, nil when the value is not one.
        var object: PluginsJSONObject? {
            if case .object(let o) = self { return o }
            return nil
        }

        /// The array, nil when the value is not one.
        var array: [PluginsJSON]? {
            if case .array(let a) = self { return a }
            return nil
        }

        /// The string, nil when the value is not one.
        var string: String? {
            if case .string(let s) = self { return s }
            return nil
        }

        /// The number, nil when the value is not one.
        var number: Double? {
            if case .number(let n) = self { return n }
            return nil
        }

        /// The boolean, nil when the value is not one.
        var bool: Bool? {
            if case .bool(let b) = self { return b }
            return nil
        }

        /// The value as JavaScript's String() writes it, for labels.
        var display: String {
            switch self {
            case .null: "null"
            case .bool(let b): b ? "true" : "false"
            case .number(let n): Self.write(number: n)
            case .string(let s): s
            case .array(let a): a.map(\.display).joined(separator: ",")
            case .object: "[object Object]"
            }
        }

        /// The value as compact JSON, as JSON.stringify writes it.
        var text: String {
            var out = ""
            write(to: &out, indent: nil, depth: 0)
            return out
        }

        /// The value as JSON indented by two spaces.
        var pretty: String {
            var out = ""
            write(to: &out, indent: "  ", depth: 0)
            return out
        }

        private func write(to out: inout String, indent: String?, depth: Int) {
            func newline(_ d: Int) {
                if let indent { out += "\n" + String(repeating: indent, count: d) }
            }
            switch self {
            case .null: out += "null"
            case .bool(let b): out += b ? "true" : "false"
            case .number(let n): out += Self.write(number: n)
            case .string(let s): Self.write(string: s, to: &out)
            case .array(let a):
                out += "["
                for (i, v) in a.enumerated() {
                    if i > 0 { out += "," }
                    newline(depth + 1)
                    v.write(to: &out, indent: indent, depth: depth + 1)
                }
                if !a.isEmpty { newline(depth) }
                out += "]"
            case .object(let o):
                out += "{"
                for (i, k) in o.keys.enumerated() {
                    if i > 0 { out += "," }
                    newline(depth + 1)
                    Self.write(string: k, to: &out)
                    out += indent == nil ? ":" : ": "
                    o.values[k, default: .null].write(to: &out, indent: indent, depth: depth + 1)
                }
                if !o.keys.isEmpty { newline(depth) }
                out += "}"
            }
        }

        private static func write(number n: Double) -> String {
            guard n.isFinite else { return "null" }
            if n == n.rounded(), abs(n) < 1e15 { return String(Int64(n)) }
            return String(n)
        }

        private static func write(string s: String, to out: inout String) {
            out += "\""
            for scalar in s.unicodeScalars {
                switch scalar {
                case "\"": out += "\\\""
                case "\\": out += "\\\\"
                case "\n": out += "\\n"
                case "\r": out += "\\r"
                case "\t": out += "\\t"
                case "\u{08}": out += "\\b"
                case "\u{0C}": out += "\\f"
                default:
                    if scalar.value < 0x20 {
                        out += String(format: "\\u%04x", scalar.value)
                    } else {
                        out.unicodeScalars.append(scalar)
                    }
                }
            }
            out += "\""
        }

        /// A reader of JSON's bytes.
        private struct Parser {
            struct Invalid: Error {}

            let bytes: [UInt8]
            var index = 0

            mutating func skipSpace() {
                while index < bytes.count, [0x20, 0x09, 0x0A, 0x0D].contains(bytes[index]) { index += 1 }
            }

            mutating func value() throws -> PluginsJSON {
                skipSpace()
                guard index < bytes.count else { throw Invalid() }
                switch bytes[index] {
                case UInt8(ascii: "{"): return .object(try object())
                case UInt8(ascii: "["): return .array(try array())
                case UInt8(ascii: "\""): return .string(try string())
                case UInt8(ascii: "t"): try literal("true"); return .bool(true)
                case UInt8(ascii: "f"): try literal("false"); return .bool(false)
                case UInt8(ascii: "n"): try literal("null"); return .null
                default: return .number(try number())
                }
            }

            private mutating func literal(_ word: String) throws {
                let w = Array(word.utf8)
                guard index + w.count <= bytes.count, Array(bytes[index..<index + w.count]) == w else {
                    throw Invalid()
                }
                index += w.count
            }

            private mutating func object() throws -> PluginsJSONObject {
                index += 1
                var o = PluginsJSONObject()
                skipSpace()
                if index < bytes.count, bytes[index] == UInt8(ascii: "}") {
                    index += 1
                    return o
                }
                while true {
                    skipSpace()
                    guard index < bytes.count, bytes[index] == UInt8(ascii: "\"") else { throw Invalid() }
                    let key = try string()
                    skipSpace()
                    guard index < bytes.count, bytes[index] == UInt8(ascii: ":") else { throw Invalid() }
                    index += 1
                    o[key] = try value()
                    skipSpace()
                    guard index < bytes.count else { throw Invalid() }
                    if bytes[index] == UInt8(ascii: ",") {
                        index += 1
                    } else if bytes[index] == UInt8(ascii: "}") {
                        index += 1
                        return o
                    } else {
                        throw Invalid()
                    }
                }
            }

            private mutating func array() throws -> [PluginsJSON] {
                index += 1
                var a: [PluginsJSON] = []
                skipSpace()
                if index < bytes.count, bytes[index] == UInt8(ascii: "]") {
                    index += 1
                    return a
                }
                while true {
                    a.append(try value())
                    skipSpace()
                    guard index < bytes.count else { throw Invalid() }
                    if bytes[index] == UInt8(ascii: ",") {
                        index += 1
                    } else if bytes[index] == UInt8(ascii: "]") {
                        index += 1
                        return a
                    } else {
                        throw Invalid()
                    }
                }
            }

            private mutating func hex4() throws -> UInt32 {
                guard index + 4 <= bytes.count,
                    let v = UInt32(String(decoding: bytes[index..<index + 4], as: UTF8.self), radix: 16)
                else { throw Invalid() }
                index += 4
                return v
            }

            private mutating func string() throws -> String {
                index += 1
                var out: [UInt8] = []
                while index < bytes.count {
                    let b = bytes[index]
                    index += 1
                    switch b {
                    case UInt8(ascii: "\""):
                        return String(decoding: out, as: UTF8.self)
                    case UInt8(ascii: "\\"):
                        guard index < bytes.count else { throw Invalid() }
                        let e = bytes[index]
                        index += 1
                        switch e {
                        case UInt8(ascii: "\""), UInt8(ascii: "\\"), UInt8(ascii: "/"): out.append(e)
                        case UInt8(ascii: "n"): out.append(0x0A)
                        case UInt8(ascii: "r"): out.append(0x0D)
                        case UInt8(ascii: "t"): out.append(0x09)
                        case UInt8(ascii: "b"): out.append(0x08)
                        case UInt8(ascii: "f"): out.append(0x0C)
                        case UInt8(ascii: "u"):
                            var code = try hex4()
                            if (0xD800..<0xDC00).contains(code), index + 1 < bytes.count,
                                bytes[index] == UInt8(ascii: "\\"), bytes[index + 1] == UInt8(ascii: "u")
                            {
                                index += 2
                                let low = try hex4()
                                code = 0x10000 + ((code - 0xD800) << 10) + (low - 0xDC00)
                            }
                            out.append(
                                contentsOf: Array(String(Unicode.Scalar(code).map(Character.init) ?? "\u{FFFD}").utf8))
                        default: throw Invalid()
                        }
                    default:
                        out.append(b)
                    }
                }
                throw Invalid()
            }

            private mutating func number() throws -> Double {
                let start = index
                while index < bytes.count, "+-0123456789.eE".utf8.contains(bytes[index]) { index += 1 }
                guard let n = Double(String(decoding: bytes[start..<index], as: UTF8.self)) else { throw Invalid() }
                return n
            }
        }
    }

    /// A JSON object: its keys in their order and their values.
    struct PluginsJSONObject: Equatable, Sendable {
        /// The keys, in their order.
        private(set) var keys: [String] = []
        /// The values, by key.
        private(set) var values: [String: PluginsJSON] = [:]

        /// Whether the object has no keys.
        var isEmpty: Bool { keys.isEmpty }

        /// A key's value; setting nil removes the key, setting a new key adds
        /// it last.
        subscript(key: String) -> PluginsJSON? {
            get { values[key] }
            set {
                if let newValue {
                    if values[key] == nil { keys.append(key) }
                    values[key] = newValue
                } else if values.removeValue(forKey: key) != nil {
                    keys.removeAll { $0 == key }
                }
            }
        }
    }
#endif
