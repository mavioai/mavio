#if os(macOS)
    import AppKit
    import MavioMac

    /// The languages releases are parsed into, by their English names, as libs/release names them.
    let acquisitionReleaseLanguages = [
        "English", "French", "Spanish", "German", "Italian", "Danish", "Dutch", "Japanese", "Icelandic", "Chinese",
        "Russian", "Polish", "Vietnamese", "Swedish", "Norwegian", "Finnish", "Turkish", "Portuguese", "Flemish",
        "Greek", "Korean", "Hungarian", "Hebrew", "Lithuanian", "Czech", "Arabic", "Hindi", "Bulgarian", "Malayalam",
        "Ukrainian", "Slovak", "Thai", "Portuguese (Brazil)", "Spanish (Latino)", "Romanian", "Latvian", "Persian",
        "Catalan", "Croatian", "Serbian", "Bosnian", "Estonian", "Tamil", "Indonesian", "Macedonian", "Slovenian",
        "Bengali", "Telugu", "Kannada", "Albanian", "Afrikaans", "Marathi", "Tagalog", "Urdu", "Romansh", "Mongolian",
        "Georgian",
    ]

    /// The kinds of quality profiles and definitions, named.
    @MainActor var acquisitionMediaKinds: [(kind: Mavio_Acquisition_V1_MediaKind, title: String)] {
        [(.movie, MavioStrings.acquisitionMovies), (.series, MavioStrings.acquisitionSeries)]
    }

    /// A download protocol's name; empty when it is unspecified.
    @MainActor func acquisitionProtocolName(_ p: Mavio_Acquisition_V1_DownloadProtocol) -> String {
        switch p {
        case .torrent: MavioStrings.acquisitionTorrent
        case .usenet: MavioStrings.acquisitionUsenet
        default: ""
        }
    }

    /// Whether two messages hold the same values, a field set to its default the same as one unset.
    func acquisitionSame<M: SwiftProtobuf.Message>(_ a: M, _ b: M) -> Bool {
        var options = JSONEncodingOptions()
        options.alwaysPrintEnumsAsInts = true
        options.alwaysPrintInt64sAsNumbers = true
        func values(_ m: M) -> NSObject? {
            guard let data = try? m.jsonUTF8Data(options: options),
                let json = try? JSONSerialization.jsonObject(with: data)
            else { return nil }
            return withoutDefaults(json) as? NSObject
        }
        return values(a) == values(b)
    }

    /// A message's JSON without the fields holding their defaults, which a message sets or leaves out alike.
    private func withoutDefaults(_ json: Any) -> Any? {
        switch json {
        case let object as [String: Any]:
            let kept = object.compactMapValues(withoutDefaults)
            return kept.isEmpty ? nil : kept
        case let array as [Any]:
            return array.isEmpty ? nil : array.map { withoutDefaults($0) ?? NSNull() }
        case let string as String:
            return string.isEmpty || string == "0s" ? nil : string
        case let number as NSNumber:
            return number.doubleValue == 0 ? nil : number
        default:
            return json
        }
    }

    /// A file's size in megabytes, or gigabytes from a thousand megabytes.
    func acquisitionSize(_ bytes: Int64) -> String {
        let n = Double(bytes)
        let (value, unit, digits): (Double, UnitInformationStorage, Int) =
            n >= 1e9 ? (n / 1e9, .gigabytes, 2) : (n / 1e6, .megabytes, 1)
        return Measurement(value: value, unit: unit).formatted(
            .measurement(
                width: .abbreviated, usage: .asProvided,
                numberFormatStyle: .number.precision(.fractionLength(0...digits))))
    }

    /// Texts joined by dots, the empty ones left out.
    func dotted(_ parts: [String?]) -> String {
        parts.compactMap { $0 }.filter { !$0.isEmpty }.joined(separator: " · ")
    }

    /// A choice of a menu: a code and its name.
    typealias Choice = (code: String, name: String)

    /// The choices of a list, with the value chosen though the list lacks it.
    func choices(_ list: [Choice], _ chosen: String) -> [Choice] {
        chosen.isEmpty || list.contains { $0.code == chosen } ? list : list + [(chosen, chosen)]
    }

    extension AppModel {
        /// The languages of a server's metadata, named in the user's
        /// language where it can, by name, without repeats.
        func languages(_ server: ServerRecord) async -> [Choice] {
            let list = try? await call(
                server, Procedure.localization("ListLanguages"), Mavio_System_V1_ListLanguagesRequest(),
                as: Mavio_System_V1_ListLanguagesResponse.self)
            let locale = Locale.current
            var seen = Set<String>()
            return (list?.languages ?? [])
                .filter { !$0.code.isEmpty && seen.insert($0.code).inserted }
                .map { (code: $0.code, name: locale.localizedString(forLanguageCode: $0.code) ?? $0.name) }
                .sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
        }
    }

    /// What the pages of Subtitles share: the languages subtitles can have, and how they are named.
    @MainActor
    enum SubtitlesNaming {
        /// Languages a subtitle can have beyond the server's, which name only ISO 639-1 codes.
        private static let extraLanguages = ["pt-BR", "zh-Hans", "zh-Hant"]

        /// The cutoff that ends the search once any language has subtitles.
        static let cutoffAny: Int32 = 65535

        /// The choices of subtitle languages: the server's, named in the user's language, with those it lacks
        /// and any of values, by name.
        static func languages(_ known: [Choice], _ values: [String]) -> [Choice] {
            var seen = Set(known.map(\.code))
            var all = known
            for code in extraLanguages + values where !code.isEmpty && seen.insert(code).inserted {
                all.append((code: code, name: languageName(code)))
            }
            return all.sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
        }

        /// A language's name in the user's language, or its tag.
        static func languageName(_ tag: String) -> String {
            Locale.current.localizedString(forIdentifier: tag) ?? tag
        }

        /// A profile's language, as "French (forced)".
        static func itemName(_ item: Mavio_Acquisition_V1_SubtitleProfileItem) -> String {
            let kinds = [
                item.forced ? MavioStrings.subtitlesForced : "",
                item.hearingImpaired == .required ? MavioStrings.subtitlesHiShort : "",
                item.hearingImpaired == .excluded ? MavioStrings.subtitlesNoHiShort : "",
            ].filter { !$0.isEmpty }
            let name = languageName(item.language)
            return kinds.isEmpty ? name : "\(name) (\(kinds.joined(separator: ", ")))"
        }
    }
#endif
