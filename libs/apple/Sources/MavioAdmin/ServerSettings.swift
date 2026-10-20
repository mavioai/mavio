import MavioKit
import MavioUI
import SwiftUI

/// A row of a settings form: its label and what it means, then its control.
struct FormRow<Control: View>: View {
    let label: String
    var detail: String?
    @ViewBuilder let control: Control
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(spacing: MavioSpacing.s12) {
            RowLabel(label: label, detail: detail)
            Spacer(minLength: 0)
            control
        }
    }
}

/// A row's label, with what it means below it.
struct RowLabel: View {
    let label: String
    var detail: String?
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(label)
            if let detail {
                Text(detail).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
            }
        }
    }
}

/// A field of a number, red while it is outside its range.
struct NumberRow: View {
    let label: String
    var detail: String?
    @Binding var value: Double
    let range: ClosedRange<Double>
    var decimals = false
    @Environment(\.theme) private var theme

    var body: some View {
        FormRow(label: label, detail: detail) {
            TextField(
                label, value: $value,
                format: .number.precision(.fractionLength(0...(decimals ? 2 : 0))).grouping(.never)
            )
            .numberKeyboard(decimals: decimals)
            .multilineTextAlignment(.trailing)
            .frame(width: 80)
            .foregroundStyle(range.contains(value) ? theme.colors.textPrimary : theme.colors.danger)
        }
    }
}

/// A text field's row, its value right of its label.
struct TextRow: View {
    let label: String
    var detail: String?
    @Binding var text: String
    var prompt: String?

    var body: some View {
        FormRow(label: label, detail: detail) {
            TextField(label, text: $text, prompt: prompt.map { Text($0) })
                .noCapitals()
                .autocorrectionDisabled()
                .multilineTextAlignment(.trailing)
                .frame(maxWidth: 170)
        }
    }
}

extension Binding where Value == Int32 {
    /// The number as a Double, for NumberRow.
    var double: Binding<Double> {
        Binding<Double>(get: { Double(wrappedValue) }, set: { wrappedValue = Int32($0.rounded()) })
    }
}

/// A choice of a picker: a code and its name.
typealias Choice = (code: String, name: String)

/// The choices of a list, with the value chosen though the list lacks it.
func choices(_ list: [Choice], _ chosen: String) -> [Choice] {
    chosen.isEmpty || list.contains { $0.code == chosen } ? list : list + [(chosen, chosen)]
}

extension AppModel {
    /// The languages and countries of a server's metadata, named in the
    /// user's language where it can, by name, without repeats.
    func localization(_ server: ServerRecord) async -> (languages: [Choice], countries: [Choice]) {
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

    private static func named(_ list: [(String, String)], _ of: (String) -> String?) -> [Choice] {
        var seen = Set<String>()
        return
            list
            .filter { !$0.0.isEmpty && seen.insert($0.0).inserted }
            .map { (code: $0.0, name: of($0.0) ?? $0.1) }
            .sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
    }
}

/// The server's settings as a page edits them: read, changed and saved,
/// each change applied to the settings the server has when saving so that
/// what other pages changed in the meantime is kept.
@MainActor @Observable
private final class SettingsEditor {
    let server: ServerRecord
    private let model: AppModel
    private(set) var saved: Mavio_System_V1_GetServerSettingsResponse?
    var failure: String?
    var saving = false
    var notice: String?

    init(server: ServerRecord, model: AppModel) {
        self.server = server
        self.model = model
    }

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

// MARK: General

/// General (docs/clients.md §6.3.8): the server's name, the language and
/// country of its metadata, how long devices stay signed in, the plugin that
/// resets forgotten passwords, and how it is reached.
struct GeneralView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var editor: SettingsEditor?
    @State private var draft = Draft()
    @State private var languages: [Choice] = []
    @State private var countries: [Choice] = []
    @State private var resetters: [(id: String, name: String)] = []
    @State private var mavioDev = false

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

    var body: some View {
        List {
            if let saved = editor?.saved {
                form(saved.settings)
            } else if editor?.failure == nil {
                ProgressView().frame(maxWidth: .infinity).listRowBackground(Color.clear)
            }
            if let failure = editor?.failure {
                Text(failure).foregroundStyle(theme.colors.danger)
            }
        }
        .scrollDismissesKeyboard(.interactively)
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsGeneral)
        .inlineTitle()
        .toolbar {
            if editor?.saving == true {
                ProgressView()
            } else if let saved = editor?.saved {
                Button(MavioStrings.actionSave, action: save)
                    .disabled(draft == Draft(saved.settings) || !draft.valid)
            }
        }
        .task { await load() }
    }

    @ViewBuilder private func form(_ settings: Mavio_System_V1_ServerSettings) -> some View {
        Section {
            TextRow(label: MavioStrings.generalName, detail: MavioStrings.generalNameDetail, text: $draft.name)
            Picker(selection: $draft.language) {
                Text(MavioStrings.generalLanguageDefault).tag("")
                ForEach(choices(languages, draft.language), id: \.code) { Text($0.name).tag($0.code) }
            } label: {
                RowLabel(label: MavioStrings.generalLanguage, detail: MavioStrings.generalLanguageDetail)
            }
            .listPicker()
            Picker(selection: $draft.country) {
                Text(MavioStrings.generalCountryDefault).tag("")
                ForEach(choices(countries, draft.country), id: \.code) { Text($0.name).tag($0.code) }
            } label: {
                RowLabel(label: MavioStrings.generalCountry, detail: MavioStrings.generalCountryDetail)
            }
            .listPicker()
        }
        Section(MavioStrings.generalSignIn) {
            NumberRow(
                label: MavioStrings.generalIdleDays, detail: MavioStrings.generalIdleDaysDetail,
                value: $draft.idleDays.double, range: 1...3650)
            if !mavioDev {
                Picker(selection: $draft.resetPlugin) {
                    Text(MavioStrings.generalResetOff).tag("")
                    ForEach(choices(resetters.map { ($0.id, $0.name) }, draft.resetPlugin), id: \.code) {
                        Text($0.name).tag($0.code)
                    }
                } label: {
                    RowLabel(
                        label: MavioStrings.generalResetPlugin,
                        detail: resetters.isEmpty
                            ? MavioStrings.generalResetPluginNone : MavioStrings.generalResetPluginDetail)
                }
            }
        }
        Section {
            TextRow(
                label: MavioStrings.generalBaseUrl, detail: MavioStrings.generalBaseUrlDetail, text: $draft.baseURL,
                prompt: "/mavio")
            NumberRow(
                label: MavioStrings.generalHttpsPort,
                detail: mavioDev ? MavioStrings.generalHttpsPortMavioDev : MavioStrings.generalHttpsPortDetail,
                value: $draft.httpsPort.double, range: 0...65535)
            if draft.httpsPort > 0 {
                TextRow(
                    label: MavioStrings.generalCertificate, detail: MavioStrings.generalCertificateDetail,
                    text: $draft.certificate, prompt: "/etc/mavio/cert.pem")
                TextRow(label: MavioStrings.generalKey, text: $draft.key, prompt: "/etc/mavio/key.pem")
            }
            Toggle(isOn: $draft.discovery) {
                RowLabel(label: MavioStrings.generalDiscovery, detail: MavioStrings.generalDiscoveryDetail)
            }
            if mavioDev {
                Toggle(isOn: $draft.relay) {
                    RowLabel(label: MavioStrings.generalRelay, detail: MavioStrings.generalRelayDetail)
                }
            }
        } header: {
            Text(MavioStrings.generalNetwork)
        } footer: {
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                Text(MavioStrings.generalNetworkFooter)
                if let notice = editor?.notice, draft == Draft(settings) { Text(notice) }
            }
        }
    }

    private func load() async {
        if editor == nil {
            let e = SettingsEditor(server: server, model: model)
            editor = e
            await e.load()
            if let s = e.saved?.settings { draft = Draft(s) }
        }
        async let info = try? model.call(
            server, Procedure.auth("GetAuthInfo"), Mavio_Auth_V1_GetAuthInfoRequest(),
            as: Mavio_Auth_V1_GetAuthInfoResponse.self)
        async let plugins = try? model.call(
            server, Procedure.plugins("ListPlugins"), Mavio_System_V1_ListPluginsRequest(),
            as: Mavio_System_V1_ListPluginsResponse.self)
        (languages, countries) = await model.localization(server)
        mavioDev = await info?.mode == .mavioDev
        resetters = ((await plugins)?.plugins ?? [])
            .filter { $0.manifest.capabilities.contains(.passwordReset) }
            .map { ($0.manifest.id, $0.manifest.name.isEmpty ? $0.manifest.id : $0.manifest.name) }
    }

    private func save() {
        guard let editor else { return }
        let draft = draft
        Task {
            if await editor.save(MavioStrings.generalSaved, { draft.apply(to: &$0) }),
                let s = editor.saved?.settings
            {
                self.draft = Draft(s)
                // The app names the server as it now calls itself.
                await model.refreshServerInfo(server.id)
            }
        }
    }
}

// MARK: Transcoding

/// Transcoding (docs/clients.md §6.3.9): how the server re-encodes what
/// devices cannot play — its hardware acceleration, the encoder's preset and
/// quality, tone mapping, deinterlacing, the downmix, cropping and the folder
/// of running transcodes.
struct TranscodingView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var editor: SettingsEditor?
    @State private var draft = Mavio_System_V1_TranscodingSettings()

    private static let presets = [
        "ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow",
    ]
    private static let tonemaps = ["bt2390", "hable", "mobius", "reinhard", "clip", "linear", "gamma"]
    private static let ranges = ["auto", "tv", "pc"]
    private static let deinterlacers = ["yadif", "bwdif"]

    var body: some View {
        List {
            if let saved = editor?.saved {
                form(saved)
            } else if editor?.failure == nil {
                ProgressView().frame(maxWidth: .infinity).listRowBackground(Color.clear)
            }
            if let failure = editor?.failure {
                Text(failure).foregroundStyle(theme.colors.danger)
            }
        }
        .scrollDismissesKeyboard(.interactively)
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsTranscoding)
        .inlineTitle()
        .toolbar {
            if editor?.saving == true {
                ProgressView()
            } else if let saved = editor?.saved {
                Button(MavioStrings.actionSave, action: save)
                    .disabled(draft == saved.settings.transcoding || !valid)
            }
        }
        .task {
            guard editor == nil else { return }
            let e = SettingsEditor(server: server, model: model)
            editor = e
            await e.load()
            if let s = e.saved?.settings { draft = s.transcoding }
        }
    }

    private var valid: Bool {
        (0...51).contains(draft.h264Crf) && (0...51).contains(draft.h265Crf) && (0...256).contains(draft.threads)
            && (0...10).contains(draft.tonemapDesat) && (0...10000).contains(draft.tonemapPeak)
            && (0.5...3).contains(draft.downmixBoost)
    }

    @ViewBuilder private func form(_ saved: Mavio_System_V1_GetServerSettingsResponse) -> some View {
        Section {
            Text(MavioStrings.transcodingMessage).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                .listRowBackground(Color.clear)
        }
        Section(MavioStrings.transcodingHardware) {
            Picker(selection: $draft.hardwareAcceleration) {
                ForEach(accelerations(saved.availableHardwareAccelerations), id: \.self) {
                    Text(Self.name($0)).tag($0)
                }
            } label: {
                RowLabel(
                    label: MavioStrings.transcodingAcceleration, detail: MavioStrings.transcodingAccelerationDetail)
            }
            if draft.hardwareAcceleration != Mavio_System_V1_HardwareAcceleration.none {
                Toggle(isOn: $draft.hardwareEncoding) {
                    RowLabel(
                        label: MavioStrings.transcodingHardwareEncoding,
                        detail: MavioStrings.transcodingHardwareEncodingDetail)
                }
            }
        }
        Section(MavioStrings.transcodingEncoding) {
            Picker(selection: $draft.encoderPreset) {
                Text(MavioStrings.transcodingPresetAuto).tag("")
                ForEach(Self.presets, id: \.self) { Text($0).tag($0) }
            } label: {
                RowLabel(label: MavioStrings.transcodingPreset, detail: MavioStrings.transcodingPresetDetail)
            }
            .listPicker()
            NumberRow(
                label: MavioStrings.transcodingAvcCrf, detail: MavioStrings.transcodingCrfDetail,
                value: $draft.h264Crf.double, range: 0...51)
            NumberRow(label: MavioStrings.transcodingHevcCrf, value: $draft.h265Crf.double, range: 0...51)
            NumberRow(
                label: MavioStrings.transcodingThreads, detail: MavioStrings.transcodingThreadsDetail,
                value: $draft.threads.double, range: 0...256)
        }
        Section(MavioStrings.transcodingVideo) {
            choice(
                MavioStrings.transcodingTonemap, MavioStrings.transcodingTonemapDetail, $draft.tonemapAlgorithm,
                Self.tonemaps)
            choice(MavioStrings.transcodingTonemapRange, nil, $draft.tonemapRange, Self.ranges)
            NumberRow(
                label: MavioStrings.transcodingTonemapDesat, value: $draft.tonemapDesat, range: 0...10, decimals: true)
            NumberRow(
                label: MavioStrings.transcodingTonemapPeak, detail: MavioStrings.transcodingTonemapPeakDetail,
                value: $draft.tonemapPeak, range: 0...10000)
            choice(MavioStrings.transcodingDeinterlace, nil, $draft.deinterlaceMethod, Self.deinterlacers)
            Toggle(isOn: $draft.deinterlaceDoubleRate) {
                RowLabel(label: MavioStrings.transcodingDoubleRate, detail: MavioStrings.transcodingDoubleRateDetail)
            }
            Toggle(isOn: $draft.cropBlackBorders) {
                RowLabel(label: MavioStrings.transcodingCrop, detail: MavioStrings.transcodingCropDetail)
            }
        }
        Section(MavioStrings.transcodingAudio) {
            NumberRow(
                label: MavioStrings.transcodingDownmixBoost, detail: MavioStrings.transcodingDownmixBoostDetail,
                value: $draft.downmixBoost, range: 0.5...3, decimals: true)
        }
        Section {
            TextRow(
                label: MavioStrings.transcodingFolder, text: $draft.transcodeDir,
                prompt: MavioStrings.transcodingFolderDefault)
        } header: {
            Text(MavioStrings.transcodingFolder)
        } footer: {
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                Text(MavioStrings.transcodingFolderFooter)
                if let notice = editor?.notice, draft == saved.settings.transcoding { Text(notice) }
            }
        }
    }

    private func choice(_ label: String, _ detail: String?, _ value: Binding<String>, _ list: [String]) -> some View {
        Picker(selection: value) {
            ForEach(list.contains(value.wrappedValue) ? list : list + [value.wrappedValue], id: \.self) {
                Text($0).tag($0)
            }
        } label: {
            RowLabel(label: label, detail: detail)
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

    private func save() {
        guard let editor else { return }
        var draft = draft
        draft.transcodeDir = draft.transcodeDir.trimmingCharacters(in: .whitespaces)
        Task {
            if await editor.save(MavioStrings.transcodingSaved, { $0.transcoding = draft }),
                let s = editor.saved?.settings
            {
                self.draft = s.transcoding
            }
        }
    }
}

// MARK: API keys

/// API keys (docs/clients.md §6.3.15): the keys scripts call the server
/// with, each acting as the administrator who made it; a key's token is
/// shown once, when it is made.
struct ApiKeysView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var keys: [Mavio_Auth_V1_ApiKey]?
    @State private var users: [String: String] = [:]
    @State private var failure: String?
    @State private var name = ""
    @State private var creating = false
    @State private var createFailure: String?
    @State private var made: (id: String, name: String, token: String)?
    @State private var copied = false
    @State private var revoking: Mavio_Auth_V1_ApiKey?
    @State private var confirming = false
    @FocusState private var naming: Bool

    var body: some View {
        List {
            Section {
                Text(MavioStrings.keysMessage).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                    .listRowBackground(Color.clear)
            }
            Section {
                HStack(spacing: MavioSpacing.s12) {
                    TextField(MavioStrings.keysName, text: $name)
                        .focused($naming)
                        .onSubmit(create)
                        .submitLabel(.done)
                    Button(creating ? "…" : MavioStrings.keysCreate, action: create)
                        .buttonStyle(.borderedProminent)
                        .disabled(name.trimmingCharacters(in: .whitespaces).isEmpty || creating)
                }
                if let createFailure {
                    Text(createFailure).foregroundStyle(theme.colors.danger)
                }
            } footer: {
                Text(MavioStrings.keysCreateFooter)
            }
            if let made, keys?.contains(where: { $0.id == made.id }) == true {
                Section {
                    Text(made.token)
                        .font(.system(.callout, design: .monospaced))
                        .selectable()
                    HStack {
                        if canCopy {
                            Button(copied ? MavioStrings.usersCopied : MavioStrings.keysCopy) {
                                copyToClipboard(made.token)
                                copied = true
                            }
                            .buttonStyle(.bordered)
                        }
                        Spacer()
                        Button(MavioStrings.keysDone) { self.made = nil }
                            .buttonStyle(.borderless)
                    }
                } header: {
                    Text(MavioStrings.keysMadeTitle(name: made.name))
                } footer: {
                    Text(MavioStrings.keysMadeFooter)
                }
            }
            Section(MavioStrings.keysList) {
                ForEach(keys ?? [], id: \.id) { key in
                    VStack(alignment: .leading, spacing: 2) {
                        Text(key.name)
                        Text(detail(key))
                            .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    }
                    .rowActions {
                        Button(MavioStrings.keysRevoke) { confirm(key) }
                            .tint(theme.colors.danger)
                    }
                    .contextMenu {
                        Button(MavioStrings.keysRevoke, systemImage: "trash", role: .destructive) { confirm(key) }
                    }
                }
                if let failure {
                    Text(failure).foregroundStyle(theme.colors.danger)
                } else if keys?.isEmpty == true {
                    Text(MavioStrings.keysNone).foregroundStyle(theme.colors.textSecondary)
                }
            }
        }
        .scrollDismissesKeyboard(.interactively)
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsApiKeys)
        .inlineTitle()
        .confirmationDialog(
            MavioStrings.keysRevokeTitle(name: revoking?.name ?? ""), isPresented: $confirming,
            titleVisibility: .visible
        ) {
            Button(MavioStrings.keysRevoke, role: .destructive) {
                if let revoking { revoke(revoking) }
            }
        } message: {
            Text(MavioStrings.keysRevokeMessage)
        }
        .task { await load() }
        .pullToRefresh { await load() }
    }

    private func detail(_ key: Mavio_Auth_V1_ApiKey) -> String {
        [
            users[key.userID].map { MavioStrings.keysActsAs(name: $0) },
            key.hasCreateTime ? MavioStrings.keysCreated(when: fromNow(key.createTime.date)) : nil,
            key.hasLastUseTime
                ? MavioStrings.keysUsed(when: fromNow(key.lastUseTime.date)) : MavioStrings.keysNeverUsed,
        ]
        .compactMap { $0 }
        .joined(separator: " · ")
    }

    private func load() async {
        do {
            keys = try await model.call(
                server, Procedure.auth("ListApiKeys"), Mavio_Auth_V1_ListApiKeysRequest(),
                as: Mavio_Auth_V1_ListApiKeysResponse.self
            ).apiKeys
            failure = nil
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
        if users.isEmpty,
            let r = try? await model.call(
                server, Procedure.users("ListUsers"), Mavio_User_V1_ListUsersRequest(),
                as: Mavio_User_V1_ListUsersResponse.self)
        {
            users = Dictionary(r.users.map { ($0.id, $0.name) }, uniquingKeysWith: { a, _ in a })
        }
    }

    private func create() {
        let trimmed = name.trimmingCharacters(in: .whitespaces)
        guard !trimmed.isEmpty, !creating else { return }
        naming = false
        creating = true
        createFailure = nil
        Task {
            defer { creating = false }
            var request = Mavio_Auth_V1_CreateApiKeyRequest()
            request.name = trimmed
            do {
                let r = try await model.call(
                    server, Procedure.auth("CreateApiKey"), request, as: Mavio_Auth_V1_CreateApiKeyResponse.self)
                made = (r.apiKey.id, r.apiKey.name.isEmpty ? trimmed : r.apiKey.name, r.token)
                copied = false
                name = ""
                await load()
            } catch {
                createFailure = error.reason
            }
        }
    }

    private func confirm(_ key: Mavio_Auth_V1_ApiKey) {
        revoking = key
        confirming = true
    }

    private func revoke(_ key: Mavio_Auth_V1_ApiKey) {
        Task {
            var request = Mavio_Auth_V1_RevokeApiKeyRequest()
            request.id = key.id
            do {
                _ = try await model.call(
                    server, Procedure.auth("RevokeApiKey"), request, as: Mavio_Auth_V1_RevokeApiKeyResponse.self)
                await load()
            } catch {
                failure = error.reason
            }
        }
    }
}
