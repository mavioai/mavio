#if os(macOS)
    import AppKit
    import MavioMac

    /// The server's settings as a page edits them: read, changed and saved,
    /// each change applied to the settings the server has when saving so that
    /// what other pages changed in the meantime is kept.
    @MainActor
    final class SettingsEditor {
        private let server: ServerRecord
        private let model: AppModel
        /// The settings as last read or saved.
        private(set) var saved: Mavio_System_V1_GetServerSettingsResponse?
        /// Why reading or saving failed.
        var failure: String?
        /// Whether a save runs.
        private(set) var saving = false
        /// What the last save said.
        var notice: String?

        /// An editor of server's settings.
        init(server: ServerRecord, model: AppModel) {
            self.server = server
            self.model = model
        }

        /// Reads the settings.
        func load() async {
            do {
                saved = try await model.call(
                    server, Procedure.system("GetServerSettings"), Mavio_System_V1_GetServerSettingsRequest(),
                    as: Mavio_System_V1_GetServerSettingsResponse.self)
            } catch is CancellationError {
            } catch {
                failure = error.reason
            }
        }

        /// Saves a change, saying so with the notice given.
        func save(_ notice: String, _ change: @escaping (inout Mavio_System_V1_ServerSettings) -> Void) async -> Bool {
            saving = true
            failure = nil
            defer { saving = false }
            do {
                var current = try await model.call(
                    server, Procedure.system("GetServerSettings"), Mavio_System_V1_GetServerSettingsRequest(),
                    as: Mavio_System_V1_GetServerSettingsResponse.self)
                change(&current.settings)
                var request = Mavio_System_V1_UpdateServerSettingsRequest()
                request.settings = current.settings
                let response = try await model.call(
                    server, Procedure.system("UpdateServerSettings"), request,
                    as: Mavio_System_V1_UpdateServerSettingsResponse.self)
                current.settings = response.settings
                saved = current
                self.notice = notice
                return true
            } catch {
                failure = error.reason
                return false
            }
        }
    }

    extension AppModel {
        /// The languages and countries of a server's metadata, named in the
        /// user's language where it can, by name, without repeats.
        func localizationChoices(_ server: ServerRecord) async -> (
            languages: [(code: String, name: String)], countries: [(code: String, name: String)]
        ) {
            async let langs = try? call(
                server, Procedure.localization("ListLanguages"), Mavio_System_V1_ListLanguagesRequest(),
                as: Mavio_System_V1_ListLanguagesResponse.self)
            async let ctrs = try? call(
                server, Procedure.localization("ListCountries"), Mavio_System_V1_ListCountriesRequest(),
                as: Mavio_System_V1_ListCountriesResponse.self)
            let locale = Locale.current
            let languages = Self.named((await langs)?.languages.map { ($0.code, $0.name) } ?? []) {
                locale.localizedString(forLanguageCode: $0)
            }
            let countries = Self.named((await ctrs)?.countries.map { ($0.code, $0.name) } ?? []) {
                locale.localizedString(forRegionCode: $0)
            }
            return (languages, countries)
        }

        private static func named(_ list: [(String, String)], _ of: (String) -> String?) -> [(
            code: String, name: String
        )] {
            var seen = Set<String>()
            return
                list
                .filter { !$0.0.isEmpty && seen.insert($0.0).inserted }
                .map { (code: $0.0, name: of($0.0) ?? $0.1) }
                .sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
        }
    }

    /// A page editing the server's settings: its heading with Save, its
    /// sections rebuilt as choices change, and its spinner or failure while
    /// the settings are read.
    class SettingsPage: ServerPage {
        /// The editor of the server's settings.
        private(set) lazy var editor = SettingsEditor(server: server, model: model)
        /// The page's form.
        private(set) lazy var form = FormPage(theme: theme)
        /// The page's Save button.
        private(set) lazy var save = saveButton { [weak self] in self?.saveTapped() }
        private let heading: String
        private var started = false
        private var busy = false

        /// A page of title, for server.
        init(context: MacContext, server: ServerRecord, title: String) {
            heading = title
            super.init(context: context, server: server)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = heading
        }

        override func appeared() {
            guard !started else { return }
            started = true
            run { [weak self] in
                guard let self else { return }
                await editor.load()
                loaded()
                if editor.saved != nil {
                    rebuild()
                    show(form)
                } else {
                    showError(editor.failure ?? "") { [weak self] in
                        self?.started = false
                        self?.show(LoadingView())
                        self?.appeared()
                    }
                }
            }
        }

        /// Called once the settings are read, to take the draft from them.
        func loaded() {}

        /// The sections of the form.
        func sections() -> [NSView] { [] }

        /// Whether the draft differs from the settings saved.
        var changed: Bool { false }

        /// Whether the draft can be saved.
        var valid: Bool { true }

        /// Saves the draft.
        func saveTapped() {}

        /// Builds the form again from the draft.
        func rebuild() {
            var views = [heading(heading, actions: [save])]
            views += sections()
            if let failure = editor.failure { views.append(note(failure, failed: true)) }
            form.setSections(views)
            refresh()
            refocus()
        }

        /// Enables Save as the draft changes.
        func refresh() {
            save.isEnabled = changed && valid && !busy
            save.title = busy ? "…" : MavioStrings.actionSave
        }

        /// Saves a change, saying so with notice, then takes the draft from
        /// what was saved and builds the form again.
        func perform(
            _ notice: String, _ change: @escaping (inout Mavio_System_V1_ServerSettings) -> Void,
            then: @escaping @MainActor () async -> Void = {}
        ) {
            endEditing()
            run { [weak self] in
                guard let self else { return }
                busy = true
                refresh()
                let done = await editor.save(notice, change)
                busy = false
                if done {
                    loaded()
                    rebuild()
                    await then()
                } else {
                    rebuild()
                }
            }
        }

        /// The notice of the last save, while nothing changed since.
        var notice: String? { changed ? nil : editor.notice }
    }

    // MARK: General

    /// General (docs/clients.md §6.3.8): the server's name, the language and
    /// country of its metadata, how long devices stay signed in, the plugin
    /// that resets forgotten passwords, and how it is reached.
    final class GeneralPage: SettingsPage {
        private var draft = Draft()
        private var languages: [(code: String, name: String)] = []
        private var countries: [(code: String, name: String)] = []
        private var resetters: [(id: String, name: String)] = []
        private var mavioDev = false
        private var networkSection: FormSection?
        private var certificateShown = false

        /// The settings as edited.
        struct Draft: Equatable {
            var name = ""
            var language = ""
            var country = ""
            var idleDays: Int32 = 1
            var resetPlugin = ""
            var baseURL = ""
            var httpsPort: Int32 = 0
            var certificate = ""
            var key = ""
            var discovery = false
            var relay = false

            init() {}

            init(_ s: Mavio_System_V1_ServerSettings) {
                name = s.network.serverName
                language = s.metadata.preferredLanguage
                country = s.metadata.country
                idleDays = s.sessionIdleDays
                resetPlugin = s.passwordResetPlugin
                baseURL = s.network.baseURL
                httpsPort = s.network.httpsPort
                certificate = s.network.certificatePath
                key = s.network.keyPath
                discovery = s.network.localDiscovery
                relay = s.network.relay
            }

            var valid: Bool { (1...3650).contains(idleDays) && (0...65535).contains(httpsPort) }

            func apply(to s: inout Mavio_System_V1_ServerSettings) {
                s.network.serverName = name.trimmingCharacters(in: .whitespaces)
                s.network.baseURL = baseURL.trimmingCharacters(in: .whitespaces)
                s.network.httpsPort = httpsPort
                s.network.certificatePath = certificate.trimmingCharacters(in: .whitespaces)
                s.network.keyPath = key.trimmingCharacters(in: .whitespaces)
                s.network.localDiscovery = discovery
                s.network.relay = relay
                s.metadata.preferredLanguage = language
                s.metadata.country = country
                s.sessionIdleDays = idleDays
                s.passwordResetPlugin = resetPlugin
            }
        }

        /// The General page of server.
        init(context: MacContext, server: ServerRecord) {
            super.init(context: context, server: server, title: MavioStrings.settingsGeneral)
        }

        override func appeared() {
            super.appeared()
            run { [weak self] in await self?.loadChoices() }
        }

        override func loaded() {
            if let s = editor.saved?.settings { draft = Draft(s) }
        }

        override var changed: Bool { editor.saved.map { draft != Draft($0.settings) } ?? false }
        override var valid: Bool { draft.valid }

        private func loadChoices() async {
            async let info = try? model.call(
                server, Procedure.auth("GetAuthInfo"), Mavio_Auth_V1_GetAuthInfoRequest(),
                as: Mavio_Auth_V1_GetAuthInfoResponse.self)
            async let plugins = try? model.call(
                server, Procedure.plugins("ListPlugins"), Mavio_System_V1_ListPluginsRequest(),
                as: Mavio_System_V1_ListPluginsResponse.self)
            (languages, countries) = await model.localizationChoices(server)
            mavioDev = await info?.mode == .mavioDev
            resetters = ((await plugins)?.plugins ?? [])
                .filter { $0.manifest.capabilities.contains(.passwordReset) }
                .map { ($0.manifest.id, $0.manifest.name.isEmpty ? $0.manifest.id : $0.manifest.name) }
            if editor.saved != nil {
                endEditing()
                rebuild()
            }
        }

        /// Changes the draft without rebuilding, for fields.
        private func edit(_ change: (inout Draft) -> Void) {
            change(&draft)
            refresh()
            networkSection?.footer = footer
        }

        /// Changes the draft and rebuilds, for choices.
        private func choose(_ change: (inout Draft) -> Void) {
            change(&draft)
            rebuild()
        }

        private var footer: String {
            [MavioStrings.generalNetworkFooter, notice].compactMap(\.self).joined(separator: "\n\n")
        }

        override func sections() -> [NSView] {
            let languageOptions =
                [(MavioStrings.generalLanguageDefault, "")]
                + choices(languages, draft.language).map { ($0.name, $0.code) }
            let countryOptions =
                [(MavioStrings.generalCountryDefault, "")]
                + choices(countries, draft.country).map { ($0.name, $0.code) }
            let main = section(
                nil,
                [
                    fieldRow(MavioStrings.generalName, MavioStrings.generalNameDetail, text: draft.name) {
                        [weak self] v in self?.edit { $0.name = v }
                    },
                    choiceRow(
                        MavioStrings.generalLanguage, MavioStrings.generalLanguageDetail, languageOptions,
                        draft.language
                    ) { [weak self] v in self?.choose { $0.language = v } },
                    choiceRow(
                        MavioStrings.generalCountry, MavioStrings.generalCountryDetail, countryOptions, draft.country
                    ) { [weak self] v in self?.choose { $0.country = v } },
                ])
            var signIn: [NSView] = [
                numberRow(
                    MavioStrings.generalIdleDays, MavioStrings.generalIdleDaysDetail, value: Double(draft.idleDays),
                    range: 1...3650
                ) { [weak self] v in self?.edit { $0.idleDays = v.map { Int32(clamping: Int64($0.rounded())) } ?? 0 } }
            ]
            if !mavioDev {
                let options =
                    [(MavioStrings.generalResetOff, "")]
                    + choices(resetters.map { (code: $0.id, name: $0.name) }, draft.resetPlugin).map {
                        ($0.name, $0.code)
                    }
                signIn.append(
                    choiceRow(
                        MavioStrings.generalResetPlugin,
                        resetters.isEmpty ? MavioStrings.generalResetPluginNone : MavioStrings.generalResetPluginDetail,
                        options, draft.resetPlugin
                    ) { [weak self] v in self?.choose { $0.resetPlugin = v } })
            }
            var network: [NSView] = [
                fieldRow(
                    MavioStrings.generalBaseUrl, MavioStrings.generalBaseUrlDetail, text: draft.baseURL,
                    placeholder: "/mavio"
                ) { [weak self] v in self?.edit { $0.baseURL = v } },
                numberRow(
                    MavioStrings.generalHttpsPort,
                    mavioDev ? MavioStrings.generalHttpsPortMavioDev : MavioStrings.generalHttpsPortDetail,
                    value: Double(draft.httpsPort), range: 0...65535,
                    onChange: { [weak self] v in
                        self?.edit { $0.httpsPort = v.map { Int32(clamping: Int64($0.rounded())) } ?? -1 }
                    },
                    onEnd: { [weak self] in
                        // The certificate's rows come and go with the port.
                        guard let self, certificateShown != (draft.httpsPort > 0) else { return }
                        rebuild()
                    }),
            ]
            certificateShown = draft.httpsPort > 0
            if certificateShown {
                network.append(
                    fieldRow(
                        MavioStrings.generalCertificate, MavioStrings.generalCertificateDetail, text: draft.certificate,
                        placeholder: "/etc/mavio/cert.pem"
                    ) { [weak self] v in self?.edit { $0.certificate = v } })
                network.append(
                    fieldRow(MavioStrings.generalKey, text: draft.key, placeholder: "/etc/mavio/key.pem") {
                        [weak self] v in self?.edit { $0.key = v }
                    })
            }
            network.append(
                toggleRow(MavioStrings.generalDiscovery, MavioStrings.generalDiscoveryDetail, draft.discovery) {
                    [weak self] v in self?.edit { $0.discovery = v }
                })
            if mavioDev {
                network.append(
                    toggleRow(MavioStrings.generalRelay, MavioStrings.generalRelayDetail, draft.relay) {
                        [weak self] v in self?.edit { $0.relay = v }
                    })
            }
            let networkSection = section(MavioStrings.generalNetwork, footer: footer, network)
            self.networkSection = networkSection
            return [main, section(MavioStrings.generalSignIn, signIn), networkSection]
        }

        override func saveTapped() {
            let draft = draft
            perform(MavioStrings.generalSaved, { draft.apply(to: &$0) }) { [weak self] in
                guard let self else { return }
                // The app names the server as it now calls itself.
                await model.refreshServerInfo(server.id)
            }
        }
    }

    // MARK: Transcoding

    /// Transcoding (docs/clients.md §6.3.9): how the server re-encodes what
    /// devices cannot play — its hardware acceleration, the encoder's preset
    /// and quality, tone mapping, deinterlacing, the downmix, cropping and
    /// the folder of running transcodes.
    final class TranscodingPage: SettingsPage {
        private var draft = Mavio_System_V1_TranscodingSettings()
        private var folderSection: FormSection?

        private static let presets = [
            "ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow",
        ]
        private static let tonemaps = ["bt2390", "hable", "mobius", "reinhard", "clip", "linear", "gamma"]
        private static let ranges = ["auto", "tv", "pc"]
        private static let deinterlacers = ["yadif", "bwdif"]

        /// The Transcoding page of server.
        init(context: MacContext, server: ServerRecord) {
            super.init(context: context, server: server, title: MavioStrings.settingsTranscoding)
        }

        override func loaded() {
            if let s = editor.saved?.settings { draft = s.transcoding }
        }

        override var changed: Bool { editor.saved.map { draft != $0.settings.transcoding } ?? false }

        override var valid: Bool {
            (0...51).contains(draft.h264Crf) && (0...51).contains(draft.h265Crf) && (0...256).contains(draft.threads)
                && (0...10).contains(draft.tonemapDesat) && (0...10000).contains(draft.tonemapPeak)
                && (0.5...3).contains(draft.downmixBoost)
        }

        private func edit(_ change: (inout Mavio_System_V1_TranscodingSettings) -> Void) {
            change(&draft)
            refresh()
            folderSection?.footer = footer
        }

        private func choose(_ change: (inout Mavio_System_V1_TranscodingSettings) -> Void) {
            change(&draft)
            rebuild()
        }

        private var footer: String {
            [MavioStrings.transcodingFolderFooter, notice].compactMap(\.self).joined(separator: "\n\n")
        }

        private static func int(_ v: Double?) -> Int32 {
            v.map { Int32(clamping: Int64($0.rounded())) } ?? -1
        }

        override func sections() -> [NSView] {
            guard let saved = editor.saved else { return [] }
            let message = NSTextField.label(
                MavioStrings.transcodingMessage, theme.fonts.callout, color: theme.colors.textSecondary, lines: 0)
            var hardware: [NSView] = [
                choiceRow(
                    MavioStrings.transcodingAcceleration, MavioStrings.transcodingAccelerationDetail,
                    accelerations(saved.availableHardwareAccelerations).map { (Self.name($0), $0) },
                    draft.hardwareAcceleration
                ) { [weak self] v in self?.choose { $0.hardwareAcceleration = v } }
            ]
            if draft.hardwareAcceleration != Mavio_System_V1_HardwareAcceleration.none {
                hardware.append(
                    toggleRow(
                        MavioStrings.transcodingHardwareEncoding, MavioStrings.transcodingHardwareEncodingDetail,
                        draft.hardwareEncoding
                    ) { [weak self] v in self?.edit { $0.hardwareEncoding = v } })
            }
            let encoding: [NSView] = [
                choiceRow(
                    MavioStrings.transcodingPreset, MavioStrings.transcodingPresetDetail,
                    [(MavioStrings.transcodingPresetAuto, "")] + Self.presets.map { ($0, $0) }, draft.encoderPreset
                ) { [weak self] v in self?.choose { $0.encoderPreset = v } },
                numberRow(
                    MavioStrings.transcodingAvcCrf, MavioStrings.transcodingCrfDetail, value: Double(draft.h264Crf),
                    range: 0...51
                ) { [weak self] v in self?.edit { $0.h264Crf = Self.int(v) } },
                numberRow(MavioStrings.transcodingHevcCrf, value: Double(draft.h265Crf), range: 0...51) {
                    [weak self] v in self?.edit { $0.h265Crf = Self.int(v) }
                },
                numberRow(
                    MavioStrings.transcodingThreads, MavioStrings.transcodingThreadsDetail,
                    value: Double(draft.threads), range: 0...256
                ) { [weak self] v in self?.edit { $0.threads = Self.int(v) } },
            ]
            let video: [NSView] = [
                list(
                    MavioStrings.transcodingTonemap, MavioStrings.transcodingTonemapDetail, draft.tonemapAlgorithm,
                    Self.tonemaps
                ) { $0.tonemapAlgorithm = $1 },
                list(MavioStrings.transcodingTonemapRange, nil, draft.tonemapRange, Self.ranges) {
                    $0.tonemapRange = $1
                },
                numberRow(
                    MavioStrings.transcodingTonemapDesat, value: draft.tonemapDesat, range: 0...10, decimals: true
                ) { [weak self] v in self?.edit { $0.tonemapDesat = v ?? -1 } },
                numberRow(
                    MavioStrings.transcodingTonemapPeak, MavioStrings.transcodingTonemapPeakDetail,
                    value: draft.tonemapPeak, range: 0...10000
                ) { [weak self] v in self?.edit { $0.tonemapPeak = v ?? -1 } },
                list(MavioStrings.transcodingDeinterlace, nil, draft.deinterlaceMethod, Self.deinterlacers) {
                    $0.deinterlaceMethod = $1
                },
                toggleRow(
                    MavioStrings.transcodingDoubleRate, MavioStrings.transcodingDoubleRateDetail,
                    draft.deinterlaceDoubleRate
                ) { [weak self] v in self?.edit { $0.deinterlaceDoubleRate = v } },
                toggleRow(MavioStrings.transcodingCrop, MavioStrings.transcodingCropDetail, draft.cropBlackBorders) {
                    [weak self] v in self?.edit { $0.cropBlackBorders = v }
                },
            ]
            let audio: [NSView] = [
                numberRow(
                    MavioStrings.transcodingDownmixBoost, MavioStrings.transcodingDownmixBoostDetail,
                    value: draft.downmixBoost, range: 0.5...3, decimals: true
                ) { [weak self] v in self?.edit { $0.downmixBoost = v ?? -1 } }
            ]
            let folder = section(
                MavioStrings.transcodingFolder, footer: footer,
                [
                    fieldRow(
                        MavioStrings.transcodingFolder, text: draft.transcodeDir,
                        placeholder: MavioStrings.transcodingFolderDefault
                    ) { [weak self] v in self?.edit { $0.transcodeDir = v } }
                ])
            folderSection = folder
            return [
                message,
                section(MavioStrings.transcodingHardware, hardware),
                section(MavioStrings.transcodingEncoding, encoding),
                section(MavioStrings.transcodingVideo, video),
                section(MavioStrings.transcodingAudio, audio),
                folder,
            ]
        }

        /// A row choosing among a list's names, with the value though the
        /// list lacks it.
        private func list(
            _ title: String, _ detail: String?, _ value: String, _ names: [String],
            _ set: @escaping (inout Mavio_System_V1_TranscodingSettings, String) -> Void
        ) -> FormRow {
            let all = names.contains(value) ? names : names + [value]
            return choiceRow(title, detail, all.map { ($0, $0) }, value) { [weak self] v in
                self?.choose { set(&$0, v) }
            }
        }

        /// The accelerations offered: the server's, with the one chosen.
        private func accelerations(_ available: [Mavio_System_V1_HardwareAcceleration])
            -> [Mavio_System_V1_HardwareAcceleration]
        {
            var list = available.isEmpty ? [.auto, Mavio_System_V1_HardwareAcceleration.none] : available
            if !list.contains(draft.hardwareAcceleration) { list.append(draft.hardwareAcceleration) }
            return list.filter { $0 != .unspecified }
        }

        private static func name(_ a: Mavio_System_V1_HardwareAcceleration) -> String {
            switch a {
            case .auto: MavioStrings.transcodingAccelerationAuto
            case .none: MavioStrings.transcodingAccelerationNone
            case .videotoolbox: "VideoToolbox"
            default: String(a.rawValue)
            }
        }

        override func saveTapped() {
            var draft = draft
            draft.transcodeDir = draft.transcodeDir.trimmingCharacters(in: .whitespaces)
            perform(MavioStrings.transcodingSaved) { $0.transcoding = draft }
        }
    }
#endif
