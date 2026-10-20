#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftProtobuf
    import SwiftUI

    /// Acquisition (docs/clients.md §6.3.11), as the web UI's: the indexers and download clients the server's
    /// plugins have configured, the quality profiles of movies and series, and the pages of what else decides
    /// which releases are grabbed — quality definitions, custom formats, delay and release profiles and
    /// remote path mappings.
    struct AcquisitionView: View {
        let server: ServerRecord
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var indexers: Mavio_Acquisition_V1_ListIndexersResponse?
        @State private var indexersFailure: String?
        @State private var clients: Mavio_Acquisition_V1_ListDownloadClientsResponse?
        @State private var clientsFailure: String?
        @State private var profiles: [Mavio_Acquisition_V1_QualityProfile]?
        @State private var profilesFailure: String?
        /// The names of the server's plugins, by their IDs.
        @State private var plugins: [String: String] = [:]

        var body: some View {
            SettingsList(message: MavioStrings.acquisitionMessage) {
                Section {
                    ForEach(indexers?.indexers ?? [], id: \.key) { x in
                        AcquisitionTileRow(
                            icon: "search",
                            color: !x.enabled
                                ? Color(hex: 0x8E8E93)
                                : leftOut(x) != nil ? Color(hex: 0xFF9500) : Color(hex: 0x5856D6),
                            label: x.name,
                            detail: [
                                pluginName(x.pluginID), acquisitionProtocolName(x.protocol), privacyName(x.privacy),
                                MavioStrings.acquisitionPriority(priority: String(x.priority)), leftOut(x),
                            ]
                            .compactMap { $0 }.filter { !$0.isEmpty }.joined(separator: " · "))
                    }
                    ForEach(indexers?.failures ?? [], id: \.key) { f in
                        AcquisitionNote(text: "\(pluginName(f.pluginID)): \(f.message)", failed: true)
                    }
                    if let indexers, indexers.indexers.isEmpty, indexers.failures.isEmpty {
                        AcquisitionNote(text: MavioStrings.acquisitionNoIndexers)
                    }
                    if let indexersFailure { AcquisitionNote(text: indexersFailure, failed: true) }
                } header: {
                    Text(MavioStrings.acquisitionIndexers)
                } footer: {
                    Text(MavioStrings.acquisitionIndexersFooter)
                }
                Section {
                    ForEach(clients?.clients ?? [], id: \.key) { x in
                        AcquisitionTileRow(
                            icon: "downloads",
                            color: !x.enabled
                                ? Color(hex: 0x8E8E93)
                                : leftOut(x) != nil ? Color(hex: 0xFF9500) : Color(hex: 0x34C759),
                            label: x.name,
                            detail: [
                                pluginName(x.pluginID), acquisitionProtocolName(x.protocol),
                                MavioStrings.acquisitionPriority(priority: String(x.priority)), leftOut(x),
                            ]
                            .compactMap { $0 }.filter { !$0.isEmpty }.joined(separator: " · "))
                    }
                    ForEach((clients?.failures ?? [:]).sorted { $0.key < $1.key }, id: \.key) { plugin, error in
                        AcquisitionNote(text: "\(pluginName(plugin)): \(error)", failed: true)
                    }
                    if let clients, clients.clients.isEmpty, clients.failures.isEmpty {
                        AcquisitionNote(text: MavioStrings.acquisitionNoClients)
                    }
                    if let clientsFailure { AcquisitionNote(text: clientsFailure, failed: true) }
                } header: {
                    Text(MavioStrings.acquisitionClients)
                } footer: {
                    Text(MavioStrings.acquisitionClientsFooter)
                }
                ForEach(acquisitionMediaKinds, id: \.kind) { kind in
                    Section {
                        ForEach(profilesOf(kind.kind), id: \.id) { p in
                            NavigationLink {
                                AcquisitionProfileView(server: server, id: p.id, kind: p.kind)
                            } label: {
                                RowLabel(
                                    label: p.name, detail: p.upgradeAllowed ? MavioStrings.acquisitionUpgrades : nil)
                            }
                            .rowLink()
                        }
                        NavigationLink {
                            AcquisitionProfileView(server: server, id: nil, kind: kind.kind)
                        } label: {
                            Text(MavioStrings.acquisitionAddProfile).foregroundStyle(theme.colors.accentText)
                        }
                        .rowLink()
                        if let profilesFailure { AcquisitionNote(text: profilesFailure, failed: true) }
                    } header: {
                        Text(MavioStrings.acquisitionProfilesOf(kind: kind.title))
                    }
                }
                Section(MavioStrings.acquisitionRules) {
                    rule(
                        "quality", 0xFF2D55, MavioStrings.acquisitionQualities, MavioStrings.acquisitionQualitiesDetail
                    ) {
                        AcquisitionQualityDefinitionsView(server: server)
                    }
                    rule("filter", 0xAF52DE, MavioStrings.acquisitionFormats, MavioStrings.acquisitionFormatsDetail) {
                        AcquisitionFormatsView(server: server)
                    }
                    rule("pending", 0xFF9500, MavioStrings.acquisitionDelays, MavioStrings.acquisitionDelaysDetail) {
                        AcquisitionDelayProfilesView(server: server)
                    }
                    rule("sort", 0x007AFF, MavioStrings.acquisitionReleases, MavioStrings.acquisitionReleasesDetail) {
                        AcquisitionReleaseProfilesView(server: server)
                    }
                    rule("folder", 0x30B0C7, MavioStrings.acquisitionMappings, MavioStrings.acquisitionMappingsDetail) {
                        AcquisitionRemotePathMappingsView(server: server)
                    }
                }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.settingsAcquisition)
            .inlineTitle()
            .task { await load() }
        }

        private func rule(
            _ icon: String, _ color: UInt32, _ label: String, _ detail: String,
            @ViewBuilder page: @escaping () -> some View
        ) -> some View {
            NavigationLink(destination: page) {
                AcquisitionTileRow(icon: icon, color: Color(hex: color), label: label, detail: detail)
            }
            .rowLink()
        }

        private func profilesOf(_ kind: Mavio_Acquisition_V1_MediaKind) -> [Mavio_Acquisition_V1_QualityProfile] {
            (profiles ?? []).filter { $0.kind == kind }.sorted {
                $0.name.localizedCompare($1.name) == .orderedAscending
            }
        }

        private func pluginName(_ id: String) -> String {
            plugins[id].flatMap { $0.isEmpty ? nil : $0 } ?? id
        }

        private func privacyName(_ p: Mavio_Acquisition_V1_IndexerPrivacy) -> String {
            switch p {
            case .public: MavioStrings.acquisitionPublic
            case .semiPrivate: MavioStrings.acquisitionSemiPrivate
            case .private: MavioStrings.acquisitionPrivate
            default: ""
            }
        }

        /// Why an indexer is left out: its failure, until when.
        private func leftOut(_ x: Mavio_Acquisition_V1_Indexer) -> String? {
            leftOut(enabled: x.enabled, until: x.hasDisabledUntil ? x.disabledUntil.date : nil, failure: x.failure)
        }

        /// Why a download client is left out: its failure, until when.
        private func leftOut(_ x: Mavio_Acquisition_V1_DownloadClient) -> String? {
            leftOut(enabled: x.enabled, until: x.hasDisabledUntil ? x.disabledUntil.date : nil, failure: x.failure)
        }

        private func leftOut(enabled: Bool, until: Date?, failure: String) -> String? {
            if !enabled { return MavioStrings.acquisitionOff }
            guard let until, until > .now else { return nil }
            return MavioStrings.acquisitionLeftOut(when: fromNow(until), failure: failure)
        }

        private func load() async {
            async let i = Result {
                try await model.call(
                    server, Procedure.releases("ListIndexers"), Mavio_Acquisition_V1_ListIndexersRequest(),
                    as: Mavio_Acquisition_V1_ListIndexersResponse.self)
            }
            async let c = Result {
                try await model.call(
                    server, Procedure.downloads("ListDownloadClients"),
                    Mavio_Acquisition_V1_ListDownloadClientsRequest(),
                    as: Mavio_Acquisition_V1_ListDownloadClientsResponse.self)
            }
            async let p = Result {
                try await model.call(
                    server, Procedure.profiles("ListQualityProfiles"),
                    Mavio_Acquisition_V1_ListQualityProfilesRequest(),
                    as: Mavio_Acquisition_V1_ListQualityProfilesResponse.self)
            }
            async let names = try? model.call(
                server, Procedure.plugins("ListPlugins"), Mavio_System_V1_ListPluginsRequest(),
                as: Mavio_System_V1_ListPluginsResponse.self)
            switch await i {
            case .success(let r): (indexers, indexersFailure) = (r, nil)
            case .failure(let e): indexersFailure = e.reason
            }
            switch await c {
            case .success(let r): (clients, clientsFailure) = (r, nil)
            case .failure(let e): clientsFailure = e.reason
            }
            switch await p {
            case .success(let r): (profiles, profilesFailure) = (r.profiles, nil)
            case .failure(let e): profilesFailure = e.reason
            }
            if let names = await names {
                plugins = Dictionary(
                    names.plugins.map { ($0.manifest.id, $0.manifest.name) }, uniquingKeysWith: { a, _ in a })
            }
        }
    }

    extension Result where Failure == any Error {
        /// The result of an asynchronous call.
        fileprivate init(catching body: () async throws -> Success) async {
            do { self = .success(try await body()) } catch { self = .failure(error) }
        }
    }

    extension Mavio_Acquisition_V1_Indexer {
        /// The indexer's key among the plugins'.
        fileprivate var key: String { "\(pluginID)/\(id)" }
    }

    extension Mavio_Acquisition_V1_IndexerFailure {
        /// The failure's key among the plugins'.
        fileprivate var key: String { "\(pluginID)/\(indexerID)" }
    }

    extension Mavio_Acquisition_V1_DownloadClient {
        /// The client's key among the plugins'.
        fileprivate var key: String { "\(pluginID)/\(id)" }
    }

    /// The kinds of quality profiles and definitions, named.
    var acquisitionMediaKinds: [(kind: Mavio_Acquisition_V1_MediaKind, title: String)] {
        [(.movie, MavioStrings.acquisitionMovies), (.series, MavioStrings.acquisitionSeries)]
    }

    /// A download protocol's name; empty when it is unspecified.
    func acquisitionProtocolName(_ p: Mavio_Acquisition_V1_DownloadProtocol) -> String {
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

    /// A row of Acquisition: its icon on a colored tile, its label and the line under it.
    struct AcquisitionTileRow: View {
        let icon: String
        let color: Color
        let label: String
        var detail: String?

        var body: some View {
            HStack(spacing: MavioSpacing.s12) {
                ColorTile(icon: icon, color: color)
                RowLabel(label: label, detail: detail?.isEmpty == false ? detail : nil)
            }
        }
    }

    /// A line of a group telling what it lacks or what failed.
    struct AcquisitionNote: View {
        let text: String
        var failed = false
        @Environment(\.theme) private var theme

        var body: some View {
            Text(text)
                .type(theme.type.callout)
                .foregroundStyle(failed ? theme.colors.danger : theme.colors.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
                .selectable()
        }
    }

    /// A number field kept within its range, with buttons to step it.
    struct AcquisitionNumberField: View {
        let label: String
        @Binding var value: Double
        var range: ClosedRange<Double> = 0...Double(Int32.max)
        var step: Double = 1

        var body: some View {
            let clamped = Binding<Double>(
                get: { value }, set: { value = Swift.min(Swift.max($0, range.lowerBound), range.upperBound) })
            HStack(spacing: 2) {
                TextField(
                    label, value: clamped,
                    format: .number.precision(.fractionLength(0...(step < 1 ? 2 : 0))).grouping(.never)
                )
                .labelsHidden()
                .multilineTextAlignment(.trailing)
                .frame(width: 80)
                Stepper(label, value: clamped, in: range, step: step).labelsHidden()
            }
            .accessibilityLabel(label)
        }
    }

    /// The buttons of a page's form under its groups: saving, with what saving told.
    struct AcquisitionSaveBar: View {
        let title: String
        let enabled: Bool
        let busy: Bool
        var notice: String?
        var failure: String?
        let save: () -> Void
        @Environment(\.theme) private var theme

        var body: some View {
            HStack(spacing: MavioSpacing.s12) {
                Button(busy ? "…" : title, action: save)
                    .buttonStyle(.borderedProminent)
                    .disabled(!enabled || busy)
                if let notice {
                    Text(notice).foregroundStyle(theme.colors.textSecondary)
                }
                if let failure {
                    Text(failure).foregroundStyle(theme.colors.danger).fixedSize(horizontal: false, vertical: true)
                }
            }
            .type(theme.type.callout)
        }
    }
#endif
