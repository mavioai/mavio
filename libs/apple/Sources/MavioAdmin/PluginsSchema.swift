#if os(macOS)
    import Foundation

    /// The subset of JSON Schema the plugins' configuration form draws, as
    /// the web UI's (apps/web's schema-form).
    final class PluginsSchema: Sendable {
        let type: [String]?
        let title: String?
        let description: String?
        /// The properties in their order, nil when the schema names none.
        let properties: [(name: String, schema: PluginsSchema)]?
        let required: [String]
        let items: PluginsSchema?
        let enumValues: [PluginsJSON]?
        let defaultValue: PluginsJSON?
        let format: String?
        let minimum: Double?
        let maximum: Double?
        let writeOnly: Bool
        let readOnly: Bool
        let oneOf: [PluginsSchema]?
        let constValue: PluginsJSON?
        /// Where the property's values come from (x-mavio-options).
        let options: Options?
        /// The sibling property whose value names the container dependency a
        /// switch needs on (x-mavio-requires-container).
        let requiresContainer: String?

        /// Where a property's values come from, and the sibling properties a
        /// chosen option fills.
        struct Options: Sendable {
            let source: String
            /// The property taking the chosen option's settings, by their schema.
            let settings: String?
            /// The property given the chosen option's name while it has none of its own.
            let name: String?
        }

        /// The schema of a JSON text, nil when it does not parse.
        convenience init?(parsing text: String) {
            guard let json = PluginsJSON(parsing: text), let o = json.object else { return nil }
            self.init(o)
        }

        /// The schema an object describes.
        init(_ o: PluginsJSONObject) {
            switch o["type"] {
            case .string(let s): type = [s]
            case .array(let a): type = a.compactMap(\.string)
            default: type = nil
            }
            title = o["title"]?.string
            description = o["description"]?.string
            properties = o["properties"]?.object.map { p in
                p.keys.compactMap { k in p[k]?.object.map { (k, PluginsSchema($0)) } }
            }
            required = o["required"]?.array?.compactMap(\.string) ?? []
            items = o["items"]?.object.map(PluginsSchema.init)
            enumValues = o["enum"]?.array
            defaultValue = o["default"]
            format = o["format"]?.string
            minimum = o["minimum"]?.number
            maximum = o["maximum"]?.number
            writeOnly = o["writeOnly"]?.bool ?? false
            readOnly = o["readOnly"]?.bool ?? false
            oneOf = o["oneOf"]?.array?.compactMap { $0.object.map(PluginsSchema.init) }
            constValue = o["const"]
            options = o["x-mavio-options"]?.object.flatMap { spec in
                spec["source"]?.string.map {
                    Options(source: $0, settings: spec["settings"]?.string, name: spec["name"]?.string)
                }
            }
            requiresContainer = o["x-mavio-requires-container"]?.string
        }

        /// A copy titled and described anew, as a property gives an option's settings.
        init(_ s: PluginsSchema, title: String?, description: String?) {
            type = s.type
            self.title = title
            self.description = description
            properties = s.properties
            required = s.required
            items = s.items
            enumValues = s.enumValues
            defaultValue = s.defaultValue
            format = s.format
            minimum = s.minimum
            maximum = s.maximum
            writeOnly = s.writeOnly
            readOnly = s.readOnly
            oneOf = s.oneOf
            constValue = s.constValue
            options = s.options
            requiresContainer = s.requiresContainer
        }

        /// Whether the schema has properties to draw.
        var hasProperties: Bool { !(properties ?? []).isEmpty }

        /// The sources of options the schema names.
        var optionSources: [String] {
            var out: [String] = []
            func walk(_ s: PluginsSchema) {
                if let source = s.options?.source, !out.contains(source) { out.append(source) }
                s.properties?.forEach { walk($0.schema) }
                if let items = s.items { walk(items) }
            }
            walk(self)
            return out
        }

        /// Whether the server generates the property: a readOnly UUID.
        var generated: Bool { readOnly && format == "uuid" }

        /// The choices of an enumeration, of enum or of oneOf's constants
        /// with their titles.
        var choices: [(value: PluginsJSON, label: String)]? {
            if let enumValues { return enumValues.map { ($0, $0.display) } }
            if let oneOf, !oneOf.isEmpty, oneOf.allSatisfy({ $0.constValue != nil }) {
                return oneOf.map { ($0.constValue ?? .null, $0.title ?? ($0.constValue ?? .null).display) }
            }
            return nil
        }

        /// The type the form draws the value as.
        var kind: String? {
            if let type { return type.first { $0 != "null" } }
            if choices != nil { return "enum" }
            if properties != nil { return "object" }
            return nil
        }

        /// An empty value of the schema, with its required properties' defaults.
        var initial: PluginsJSON? {
            switch kind {
            case "object":
                var o = PluginsJSONObject()
                for (k, p) in properties ?? [] where required.contains(k) {
                    if let d = p.defaultValue { o[k] = d }
                }
                return .object(o)
            case "array": return .array([])
            default: return defaultValue
            }
        }

        /// A property's label: its title, else its key as words, api_key
        /// becoming "API key".
        func label(_ key: String) -> String {
            if let title { return title }
            var spaced = ""
            var previous: Character?
            for c in key {
                if c == "_" || c == "-" {
                    if !spaced.hasSuffix(" ") { spaced.append(" ") }
                } else {
                    if let p = previous, p.isLowercase, p.isASCII, c.isUppercase, c.isASCII { spaced.append(" ") }
                    spaced.append(c)
                }
                previous = c
            }
            let upper: Set<String> = [
                "api", "url", "id", "ip", "ssdp", "http", "https", "dns", "tls", "rss", "nzb", "tmdb", "imdb", "tvdb",
            ]
            let words = spaced.lowercased().split(separator: " ", omittingEmptySubsequences: false).map {
                upper.contains(String($0)) ? $0.uppercased() : String($0)
            }
            let joined = words.joined(separator: " ")
            return joined.prefix(1).uppercased() + joined.dropFirst()
        }

        /// Whether a property holds a secret, to draw as a password.
        func secret(_ key: String) -> Bool {
            if writeOnly || format == "password" { return true }
            let k = key.lowercased()
            return ["password", "secret", "token", "api_key", "key", "passkey"].contains {
                k == $0 || k.hasSuffix("_" + $0)
            }
        }
    }
#endif
