#if os(macOS)
    import AppKit
    import MavioMac

    /// Acquisition (docs/clients.md §6.3.11), as the web UI's: the indexers
    /// and download clients the server's plugins have configured, the
    /// quality profiles of movies and series, and the pages of what else
    /// decides which releases are grabbed — quality definitions, custom
    /// formats, delay and release profiles and the options — where approved
    /// requests go, and each download client's page.
    final class AcquisitionPage: FormController {
        private var indexers: Mavio_Acquisition_V1_ListIndexersResponse?
        private var indexersFailure: String?
        private var clients: Mavio_Acquisition_V1_ListDownloadClientsResponse?
        private var clientsFailure: String?
        private var profiles: [Mavio_Acquisition_V1_QualityProfile]?
        private var profilesFailure: String?
        /// The names of the server's plugins, by their IDs.
        private var plugins: [String: String] = [:]
        private var loaded = false

        override func viewDidLoad() {
            title = MavioStrings.settingsAcquisition
            message = MavioStrings.acquisitionMessage
            super.viewDidLoad()
        }

        override func viewWillAppear() {
            super.viewWillAppear()
            // Coming back from a profile, its changes show.
            run { [weak self] in await self?.load() }
        }

        override func sections() -> [NSView] {
            var views: [NSView] = [indexerSection(), clientSection()]
            for kind in acquisitionMediaKinds {
                var rows: [NSView] = profilesOf(kind.kind).map { p in
                    FormRow.link(
                        p.name, subtitle: p.upgradeAllowed ? MavioStrings.acquisitionUpgrades : nil, theme: theme
                    ) { [weak self] in self?.openProfile(p.id, kind: p.kind) }
                }
                rows.append(
                    addRow(MavioStrings.acquisitionAddProfile) { [weak self] in self?.openProfile(nil, kind: kind.kind)
                    })
                if let profilesFailure { rows.append(note(profilesFailure, failed: true)) }
                views.append(card(MavioStrings.acquisitionProfilesOf(kind: kind.title), rows))
            }
            let server = server
            let context = context
            views.append(
                card(
                    MavioStrings.acquisitionRules,
                    [
                        rule(
                            "quality", 0xFF2D55, MavioStrings.acquisitionQualities,
                            MavioStrings.acquisitionQualitiesDetail
                        ) {
                            QualityDefinitionsPage(context: context, server: server)
                        },
                        rule("filter", 0xAF52DE, MavioStrings.acquisitionFormats, MavioStrings.acquisitionFormatsDetail)
                        {
                            FormatsPage(context: context, server: server)
                        },
                        rule("pending", 0xFF9500, MavioStrings.acquisitionDelays, MavioStrings.acquisitionDelaysDetail)
                        {
                            DelayProfilesPage(context: context, server: server)
                        },
                        rule("sort", 0x007AFF, MavioStrings.acquisitionReleases, MavioStrings.acquisitionReleasesDetail)
                        {
                            ReleaseProfilesPage(context: context, server: server)
                        },
                        rule(
                            "discover", 0x5AC8FA, MavioStrings.requestsettingsTitle,
                            MavioStrings.requestsettingsDetail
                        ) {
                            RequestSettingsPage(context: context, server: server)
                        },
                        rule("settings", 0x8E8E93, MavioStrings.optionsOptions, MavioStrings.optionsOptionsDetail) {
                            AcquisitionOptionsPage(context: context, server: server)
                        },
                    ]))
            return views
        }

        private func indexerSection() -> NSView {
            var rows: [NSView] = (indexers?.indexers ?? []).map { x in
                let out = leftOut(
                    enabled: x.enabled, until: x.hasDisabledUntil ? x.disabledUntil.date : nil, failure: x.failure)
                return tileRow(
                    x.name,
                    subtitle: dotted([
                        pluginName(x.pluginID), acquisitionProtocolName(x.protocol), privacyName(x.privacy),
                        MavioStrings.acquisitionPriority(priority: String(x.priority)), out,
                    ]),
                    icon: "search", tint: !x.enabled ? 0x8E8E93 : out != nil ? 0xFF9500 : 0x5856D6)
            }
            for f in indexers?.failures ?? [] {
                rows.append(note("\(pluginName(f.pluginID)): \(f.message)", failed: true))
            }
            if let indexers, indexers.indexers.isEmpty, indexers.failures.isEmpty {
                rows.append(note(MavioStrings.acquisitionNoIndexers))
            }
            if let indexersFailure { rows.append(note(indexersFailure, failed: true)) }
            if !loaded { rows.append(spinnerRow()) }
            return card(MavioStrings.acquisitionIndexers, footer: MavioStrings.acquisitionIndexersFooter, rows)
        }

        private func clientSection() -> NSView {
            var rows: [NSView] = (clients?.clients ?? []).map { x in
                let out = leftOut(
                    enabled: x.enabled, until: x.hasDisabledUntil ? x.disabledUntil.date : nil, failure: x.failure)
                // A remote's offline downloads are named as such rather than by the plugin.
                let by = x.pluginID == offlineProvider ? MavioStrings.storageOffline : pluginName(x.pluginID)
                return tileRow(
                    x.name,
                    subtitle: dotted([
                        by, acquisitionProtocolName(x.protocol),
                        MavioStrings.acquisitionPriority(priority: String(x.priority)), out,
                    ]),
                    icon: "downloads", tint: !x.enabled ? 0x8E8E93 : out != nil ? 0xFF9500 : 0x34C759
                ) { [weak self] in self?.openClient(x) }
            }
            for (plugin, error) in (clients?.failures ?? [:]).sorted(by: { $0.key < $1.key }) {
                rows.append(note("\(pluginName(plugin)): \(error)", failed: true))
            }
            if let clients, clients.clients.isEmpty, clients.failures.isEmpty {
                rows.append(note(MavioStrings.acquisitionNoClients))
            }
            if let clientsFailure { rows.append(note(clientsFailure, failed: true)) }
            return card(MavioStrings.acquisitionClients, footer: MavioStrings.acquisitionClientsFooter, rows)
        }

        private func rule(
            _ icon: String, _ tint: UInt32, _ title: String, _ detail: String,
            page: @escaping @MainActor () -> NSViewController
        ) -> FormRow {
            tileRow(title, subtitle: detail, icon: icon, tint: tint) { [weak self] in self?.push(page()) }
        }

        private func openClient(_ client: Mavio_Acquisition_V1_DownloadClient) {
            push(
                DownloadClientPage(
                    context: context, server: server, client: client, pluginName: pluginName(client.pluginID)))
        }

        private func openProfile(_ id: String?, kind: Mavio_Acquisition_V1_MediaKind) {
            push(QualityProfilePage(context: context, server: server, id: id, kind: kind))
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

        /// Why an indexer or a download client is left out: its failure, until when.
        private func leftOut(enabled: Bool, until: Date?, failure: String) -> String? {
            if !enabled { return MavioStrings.acquisitionOff }
            guard let until, until > .now else { return nil }
            return MavioStrings.acquisitionLeftOut(when: fromNow(until), failure: failure)
        }

        private func load() async {
            let model = model
            let server = server
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
            let (ri, rc, rp, rn) = await (i, c, p, names)
            guard !Task.isCancelled else { return }
            switch ri {
            case .success(let r): (indexers, indexersFailure) = (r, nil)
            case .failure(let e): indexersFailure = e.reason
            }
            switch rc {
            case .success(let r): (clients, clientsFailure) = (r, nil)
            case .failure(let e): clientsFailure = e.reason
            }
            switch rp {
            case .success(let r): (profiles, profilesFailure) = (r.profiles, nil)
            case .failure(let e): profilesFailure = e.reason
            }
            if let rn {
                plugins = Dictionary(
                    rn.plugins.map { ($0.manifest.id, $0.manifest.name) }, uniquingKeysWith: { a, _ in a })
            }
            loaded = true
            render()
        }
    }
#endif
