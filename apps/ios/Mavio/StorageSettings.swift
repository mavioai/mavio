import MavioKit
import MavioUI
import SwiftUI

/// Storage (docs/clients.md §6.3.10): the disks of the server's computer,
/// the rclone the server runs or reaches, and its remotes — added with a
/// kind's settings and the questions their configuration asks, such as
/// authorizing a cloud drive — and their folders. The download clients of
/// cloud drives' offline downloads feed acquisition and are left out
/// (docs/clients.md §4.2).
struct StorageView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var status: Mavio_Storage_V1_GetStatusResponse?
    @State private var statusFailure: String?
    @State private var disks: [Mavio_Storage_V1_Disk]?
    @State private var disksFailure: String?
    @State private var remotes: [Mavio_Storage_V1_Remote]?
    @State private var remotesFailure: String?
    @State private var providers: [Mavio_Storage_V1_Provider] = []

    private var available: Bool { status?.available ?? false }

    var body: some View {
        List {
            Section {
                Text(MavioStrings.storageMessage).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                    .listRowBackground(Color.clear)
            }
            Section {
                ForEach(disks ?? [], id: \.path) { DiskRow(disk: $0) }
                if disks?.isEmpty == true {
                    Text(MavioStrings.storageNoDisks).foregroundStyle(theme.colors.textSecondary)
                } else if disks == nil && disksFailure == nil {
                    ProgressView().frame(maxWidth: .infinity)
                }
                if let disksFailure { Text(disksFailure).foregroundStyle(theme.colors.danger) }
            } header: {
                Text(MavioStrings.storageDisks)
            } footer: {
                Text(MavioStrings.storageDisksFooter)
            }
            if available {
                Section {
                    ForEach(remotes ?? [], id: \.name) { remote in
                        NavigationLink {
                            StorageRemoteView(server: server, name: remote.name, changed: reload)
                        } label: {
                            RemoteRow(server: server, remote: remote, kind: kindName(providers, remote.type))
                        }
                    }
                    if remotes?.isEmpty == true {
                        Text(MavioStrings.storageNoRemotes).foregroundStyle(theme.colors.textSecondary)
                    }
                    if let remotesFailure { Text(remotesFailure).foregroundStyle(theme.colors.danger) }
                    NavigationLink {
                        NewRemoteView(server: server, taken: (remotes ?? []).map(\.name), changed: reload)
                    } label: {
                        Label(MavioStrings.storageAdd, systemImage: "plus")
                    }
                } header: {
                    Text(MavioStrings.storageRemotes)
                } footer: {
                    Text(MavioStrings.storageRemotesFooter)
                }
            }
            Section(MavioStrings.storageRclone) {
                LabeledContent(MavioStrings.storageStatus) {
                    HStack(spacing: MavioSpacing.s8) {
                        Circle()
                            .fill(
                                status == nil && statusFailure == nil
                                    ? theme.colors.fill : available ? Color(hex: 0x34C759) : theme.colors.danger
                            )
                            .frame(width: 8, height: 8)
                        Text(statusText).multilineTextAlignment(.trailing)
                    }
                }
                if let status, !status.version.isEmpty {
                    LabeledContent(MavioStrings.storageVersion, value: status.version)
                }
                if let status, available {
                    LabeledContent {
                        Text(status.managed ? MavioStrings.storageManaged : MavioStrings.storageExternal)
                    } label: {
                        RowLabel(
                            label: MavioStrings.storageRcd,
                            detail: status.managed
                                ? MavioStrings.storageManagedDetail : MavioStrings.storageExternalDetail)
                    }
                }
            }
        }
        .scrollContentBackground(.hidden)
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsStorage)
        .navigationBarTitleDisplayMode(.inline)
        .task { await load() }
        .pullToRefresh { await load() }
    }

    private var statusText: String {
        if let statusFailure { return statusFailure }
        guard let status else { return "…" }
        if status.available { return MavioStrings.storageAvailable }
        return status.error.isEmpty ? MavioStrings.storageUnavailable : status.error
    }

    private func reload() {
        Task { await load() }
    }

    private func load() async {
        async let d: Void = loadDisks()
        do {
            status = try await model.call(
                server, Procedure.storage("GetStatus"), Mavio_Storage_V1_GetStatusRequest(),
                as: Mavio_Storage_V1_GetStatusResponse.self)
            statusFailure = nil
        } catch is CancellationError {
        } catch {
            statusFailure = error.reason
        }
        if available {
            async let p = try? model.call(
                server, Procedure.storage("ListProviders"), Mavio_Storage_V1_ListProvidersRequest(),
                as: Mavio_Storage_V1_ListProvidersResponse.self)
            do {
                remotes = try await model.remotes(server)
                remotesFailure = nil
            } catch is CancellationError {
            } catch {
                remotesFailure = error.reason
            }
            if let r = await p { providers = r.providers }
        }
        await d
    }

    private func loadDisks() async {
        do {
            disks = try await model.call(
                server, Procedure.storage("ListDisks"), Mavio_Storage_V1_ListDisksRequest(),
                as: Mavio_Storage_V1_ListDisksResponse.self
            ).disks
            disksFailure = nil
        } catch is CancellationError {
        } catch {
            disksFailure = error.reason
        }
    }
}

extension AppModel {
    /// The rclone remotes of a server, by name.
    fileprivate func remotes(_ server: ServerRecord) async throws -> [Mavio_Storage_V1_Remote] {
        try await call(
            server, Procedure.storage("ListRemotes"), Mavio_Storage_V1_ListRemotesRequest(),
            as: Mavio_Storage_V1_ListRemotesResponse.self
        ).remotes.sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
    }

    /// The folders in a folder of a remote, "rclone:<remote>:<path>".
    fileprivate func remoteFolders(_ server: ServerRecord, _ path: String) async throws -> [String] {
        var request = Mavio_Storage_V1_ListFoldersRequest()
        request.path = path
        return try await call(
            server, Procedure.storage("ListFolders"), request, as: Mavio_Storage_V1_ListFoldersResponse.self
        ).folders
    }
}

/// A size in terabytes, gigabytes or megabytes as it is large.
private func capacity(_ bytes: Int64) -> String {
    let n = Double(bytes)
    let (value, unit): (Double, UnitInformationStorage) =
        n >= 1e12 ? (n / 1e12, .terabytes) : n >= 1e9 ? (n / 1e9, .gigabytes) : (n / 1e6, .megabytes)
    return Measurement(value: value, unit: unit).formatted(
        .measurement(
            width: .abbreviated, usage: .asProvided, numberFormatStyle: .number.precision(.fractionLength(0...1))))
}

/// The keys of the kinds of remotes whose names are not camelCase as they are.
private let kindKeys = [
    "b2": "backblaze", "cloud115": "cloudOneFifteen", "cloud123": "cloudOneTwentyThree", "s3": "amazon",
]

/// A kind of remote's name in the UI's language, else as rclone describes it.
private func kindName(_ providers: [Mavio_Storage_V1_Provider], _ type: String) -> String {
    var key = kindKeys[type] ?? ""
    if key.isEmpty {
        var upper = false
        for c in type {
            if c == " " {
                upper = true
            } else {
                key += upper ? c.uppercased() : String(c)
                upper = false
            }
        }
    }
    let missing = "\u{0}"
    let name = MavioStrings.bundle.localizedString(forKey: "storage.provider.\(key)", value: missing, table: nil)
    if name != missing { return name }
    let description = providers.first { $0.name == type }?.description_p ?? ""
    return description.isEmpty ? type : description
}

/// A bar of how much of a disk or a remote is used, orange when nearly
/// full and red when full, and its sizes.
private struct UsageBar: View {
    let used: Int64
    let total: Int64
    @Environment(\.theme) private var theme

    var body: some View {
        let share = total > 0 ? min(1, Double(used) / Double(total)) : 0
        VStack(alignment: .leading, spacing: MavioSpacing.s4) {
            ProgressView(value: share)
                .tint(share >= 0.95 ? theme.colors.danger : share >= 0.85 ? Color(hex: 0xFF9500) : Color(hex: 0x007AFF))
            Text(MavioStrings.storageUsage(used: capacity(used), total: capacity(total)))
                .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
        }
        .padding(.top, 2)
    }
}

/// A disk of the server's computer: its name or path, its kind and file
/// system, and how much of it is used.
private struct DiskRow: View {
    let disk: Mavio_Storage_V1_Disk
    @Environment(\.theme) private var theme

    private var kind: (icon: String, color: Color, label: String) {
        switch disk.kind {
        case .ssd: ("disk", Color(hex: 0x5E5CE6), MavioStrings.storageDiskSsd)
        case .hdd: ("disk", Color(hex: 0x5E5CE6), MavioStrings.storageDiskHdd)
        case .network: ("network", Color(hex: 0x007AFF), MavioStrings.storageDiskNetwork)
        case .cloud: ("cloud", Color(hex: 0x30B0C7), MavioStrings.storageDiskCloud)
        default: ("disk", Color(hex: 0x8E8E93), MavioStrings.storageDiskUnknown)
        }
    }

    var body: some View {
        let k = kind
        HStack(alignment: .top, spacing: MavioSpacing.s12) {
            ColorTile(icon: k.icon, color: k.color)
            VStack(alignment: .leading, spacing: 2) {
                HStack(alignment: .firstTextBaseline, spacing: MavioSpacing.s12) {
                    Text(disk.name.isEmpty ? disk.path : disk.name).lineLimit(1)
                    Spacer(minLength: 0)
                    Text([k.label, disk.fileSystem].filter { !$0.isEmpty }.joined(separator: " · "))
                        .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary).lineLimit(1)
                }
                if !disk.name.isEmpty {
                    Text(disk.path).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary).lineLimit(1)
                }
                if disk.hasTotalBytes {
                    UsageBar(used: disk.usedBytes, total: disk.totalBytes)
                } else {
                    Text(MavioStrings.storageNoAnswer).type(theme.type.footnote)
                        .foregroundStyle(theme.colors.textTertiary)
                }
            }
        }
    }
}

/// A remote in the list, with a dot telling whether the server reaches it:
/// listing its root, green when that succeeds and red, with the error, when
/// it fails, as a remote whose login expired does; and how much of it is
/// used, where its backend tells.
private struct RemoteRow: View {
    let server: ServerRecord
    let remote: Mavio_Storage_V1_Remote
    let kind: String
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    /// nil while checking, else the error of listing its root, empty when reached.
    @State private var reached: String?
    @State private var usage: Mavio_Storage_V1_GetRemoteUsageResponse?

    var body: some View {
        HStack(alignment: .top, spacing: MavioSpacing.s12) {
            ColorTile(icon: "cloud", color: Color(hex: 0x30B0C7))
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: MavioSpacing.s8) {
                    Text(remote.name).lineLimit(1)
                    Circle()
                        .fill(
                            reached == nil
                                ? theme.colors.fill : reached!.isEmpty ? Color(hex: 0x34C759) : theme.colors.danger
                        )
                        .frame(width: 6, height: 6)
                        .accessibilityLabel(
                            reached == nil
                                ? MavioStrings.storageChecking
                                : reached!.isEmpty ? MavioStrings.storageReached : MavioStrings.storageUnreached)
                }
                Text(kind).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary).lineLimit(1)
                if let reached, !reached.isEmpty {
                    Text(reached).type(theme.type.footnote).foregroundStyle(theme.colors.danger).lineLimit(2)
                } else if let used {
                    if let usage, usage.hasTotalBytes, usage.totalBytes > 0 {
                        UsageBar(used: used, total: usage.totalBytes)
                    } else {
                        Text(MavioStrings.storageUsed(used: capacity(used)))
                            .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    }
                }
            }
        }
        .task(id: remote.name) {
            async let u: Void = loadUsage()
            do {
                _ = try await model.remoteFolders(server, "rclone:\(remote.name):")
                reached = ""
            } catch is CancellationError {
            } catch {
                reached = error.reason
            }
            await u
        }
    }

    private var used: Int64? {
        guard let usage else { return nil }
        if usage.hasUsedBytes { return usage.usedBytes }
        if usage.hasTotalBytes && usage.hasFreeBytes { return usage.totalBytes - usage.freeBytes }
        return nil
    }

    private func loadUsage() async {
        var request = Mavio_Storage_V1_GetRemoteUsageRequest()
        request.remote = remote.name
        usage = try? await model.call(
            server, Procedure.storage("GetRemoteUsage"), request, as: Mavio_Storage_V1_GetRemoteUsageResponse.self)
    }
}

// MARK: Options

/// Whether an rclone provider filter, "AWS,Ceph" or "!AWS,Ceph", takes a provider.
func providerMatches(_ filter: String, _ provider: String) -> Bool {
    if filter.isEmpty { return true }
    let negated = filter.hasPrefix("!")
    let list = (negated ? String(filter.dropFirst()) : filter).split(separator: ",").map {
        $0.trimmingCharacters(in: .whitespaces)
    }
    return list.contains(provider) != negated
}

/// The provider an S3-like kind's values choose, which its options depend on.
private func providerOf(_ options: [Mavio_Storage_V1_Option], _ values: [String: String]) -> String {
    values["provider"] ?? options.first { $0.name == "provider" }?.defaultValue ?? ""
}

/// The options of a kind shown for the values set: those its provider takes.
private func shownOptions(
    _ options: [Mavio_Storage_V1_Option], _ values: [String: String], advanced: Bool
) -> [Mavio_Storage_V1_Option] {
    let provider = providerOf(options, values)
    return options.filter { o in
        (advanced || !o.advanced || o.required) && providerMatches(o.provider, provider)
    }
}

/// The values to send: those set, differing from their defaults.
private func changedValues(_ options: [Mavio_Storage_V1_Option], _ values: [String: String]) -> [String: String] {
    values.filter { k, v in
        guard let o = options.first(where: { $0.name == k }) else { return !v.isEmpty }
        return v != o.defaultValue && (!v.isEmpty || o.defaultValue.isEmpty)
            && !v.trimmingCharacters(in: .whitespaces).isEmpty
    }
}

/// The control of an option's value: a switch, a choice among its examples,
/// or a field suggesting them.
private struct OptionControl: View {
    let option: Mavio_Storage_V1_Option
    @Binding var value: String
    let provider: String
    var placeholder: String?
    @Environment(\.theme) private var theme

    var body: some View {
        let o = option
        let examples = o.examples.filter { providerMatches($0.provider, provider) }
        let current = value.isEmpty ? o.defaultValue : value
        if o.type == "bool" {
            Toggle(
                o.name,
                isOn: Binding(get: { current == "true" }, set: { value = $0 ? "true" : "false" })
            )
            .labelsHidden()
        } else if !examples.isEmpty && o.exclusive {
            Picker(o.name, selection: Binding(get: { current }, set: { value = $0 })) {
                if !examples.contains(where: { $0.value == current }) {
                    Text(current.isEmpty ? "—" : current).tag(current)
                }
                ForEach(examples, id: \.value) { e in
                    Text(Self.label(e)).tag(e.value)
                }
            }
            .labelsHidden()
            .pickerStyle(.menu)
        } else {
            HStack(spacing: MavioSpacing.s8) {
                Group {
                    if o.password {
                        SecureField(o.name, text: $value, prompt: Text(placeholder ?? o.defaultValue))
                            .textContentType(.newPassword)
                    } else {
                        TextField(o.name, text: $value, prompt: Text(placeholder ?? o.defaultValue))
                    }
                }
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .textFieldStyle(.roundedBorder)
                if !examples.isEmpty && !o.password {
                    Menu {
                        ForEach(examples, id: \.value) { e in
                            Button(Self.label(e)) { value = e.value }
                        }
                    } label: {
                        Icon("expand", size: 16).foregroundStyle(theme.colors.textSecondary)
                    }
                    .accessibilityLabel(o.name)
                }
            }
        }
    }

    private static func label(_ e: Mavio_Storage_V1_Example) -> String {
        let help = e.help.split(separator: "\n").first.map(String.init) ?? ""
        return help.isEmpty ? e.value : "\(e.value) — \(help)"
    }
}

/// An option: its name, its whole help and its control.
private struct OptionField: View {
    let option: Mavio_Storage_V1_Option
    @Binding var value: String
    let provider: String
    var placeholder: String?
    @Environment(\.theme) private var theme

    var body: some View {
        let help = option.help.trimmingCharacters(in: .whitespacesAndNewlines)
        let inline = option.type == "bool"
        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
            HStack(alignment: .firstTextBaseline) {
                VStack(alignment: .leading, spacing: 2) {
                    (Text(option.name).monospaced()
                        + Text(option.required ? " *" : "").foregroundStyle(theme.colors.danger))
                    if !help.isEmpty {
                        Text(help).type(theme.type.caption).foregroundStyle(theme.colors.textSecondary)
                    }
                }
                if inline {
                    Spacer(minLength: MavioSpacing.s8)
                    OptionControl(option: option, value: $value, provider: provider, placeholder: placeholder)
                }
            }
            if !inline {
                OptionControl(option: option, value: $value, provider: provider, placeholder: placeholder)
            }
        }
        .padding(.vertical, 2)
    }
}

/// A remote's settings: its kind's options, with those rarely changed
/// behind a switch.
private struct OptionsSection: View {
    let options: [Mavio_Storage_V1_Option]
    @Binding var values: [String: String]
    var placeholder: (Mavio_Storage_V1_Option) -> String? = { _ in nil }
    @State private var advanced = false

    var body: some View {
        let provider = providerOf(options, values)
        Section {
            ForEach(shownOptions(options, values, advanced: advanced), id: \.name) { o in
                OptionField(
                    option: o,
                    value: Binding(get: { values[o.name] ?? "" }, set: { values[o.name] = $0 }),
                    provider: provider, placeholder: placeholder(o))
            }
            if options.contains(where: { $0.advanced && !$0.required }) {
                Toggle(MavioStrings.storageAdvanced, isOn: $advanced)
            }
        } header: {
            Text(MavioStrings.storageSettings)
        } footer: {
            Text(MavioStrings.storageSettingsFooter)
        }
    }
}

// MARK: Remotes

/// The page adding a remote: its name, its kind and the kind's settings,
/// then the questions of its configuration, and the remote's page once
/// it is added.
private struct NewRemoteView: View {
    let server: ServerRecord
    let taken: [String]
    let changed: () -> Void
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss
    @State private var providers: [Mavio_Storage_V1_Provider] = []
    @State private var name = ""
    @State private var type = ""
    @State private var values: [String: String] = [:]
    @State private var question: Mavio_Storage_V1_ConfigQuestion?
    @State private var created = false
    @State private var saving = false
    @State private var failure: String?

    private var trimmed: String { name.trimmingCharacters(in: .whitespaces) }
    private var provider: Mavio_Storage_V1_Provider? { providers.first { $0.name == type } }
    private var valid: Bool {
        trimmed.wholeMatch(of: /[A-Za-z0-9_][A-Za-z0-9_.+@-]*/) != nil && !taken.contains(trimmed)
    }
    private var missing: Bool {
        guard let provider else { return true }
        return shownOptions(provider.options, values, advanced: false).contains { o in
            o.required && (values[o.name] ?? o.defaultValue).isEmpty
        }
    }

    var body: some View {
        if created {
            StorageRemoteView(server: server, name: trimmed, changed: changed)
        } else if let question {
            RemoteQuestionView(
                server: server, name: trimmed, question: question,
                done: { created = true },
                cancel: cancel)
        } else {
            form
        }
    }

    private var form: some View {
        List {
            Section {
                TextRow(
                    label: MavioStrings.storageName,
                    detail: taken.contains(trimmed) ? MavioStrings.storageNameTaken : MavioStrings.storageNameDetail,
                    text: $name)
                Picker(MavioStrings.storageKind, selection: $type) {
                    Text(MavioStrings.storageChooseKind).tag("")
                    ForEach(providers, id: \.name) { p in
                        Text("\(kindName(providers, p.name)) (\(p.name))").tag(p.name)
                    }
                }
                .pickerStyle(.navigationLink)
                .onChange(of: type) { values = [:] }
            }
            if let provider {
                OptionsSection(options: provider.options, values: $values).id(provider.name)
            }
            if let failure {
                Section { Text(failure).foregroundStyle(theme.colors.danger) }
            }
        }
        .scrollDismissesKeyboard(.interactively)
        .scrollContentBackground(.hidden)
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.storageNew)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            if saving {
                ProgressView()
            } else {
                Button(MavioStrings.storageCreate, action: create).disabled(!valid || missing)
            }
        }
        .task { await loadProviders() }
    }

    private func loadProviders() async {
        do {
            let list = try await model.call(
                server, Procedure.storage("ListProviders"), Mavio_Storage_V1_ListProvidersRequest(),
                as: Mavio_Storage_V1_ListProvidersResponse.self
            ).providers
            providers = list.sorted {
                kindName(list, $0.name).localizedStandardCompare(kindName(list, $1.name)) == .orderedAscending
            }
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
    }

    private func create() {
        guard let provider else { return }
        UIApplication.shared.sendAction(#selector(UIResponder.resignFirstResponder), to: nil, from: nil, for: nil)
        saving = true
        failure = nil
        Task {
            defer { saving = false }
            var request = Mavio_Storage_V1_CreateRemoteRequest()
            request.name = trimmed
            request.type = type
            request.parameters = changedValues(provider.options, values)
            do {
                let r = try await model.call(
                    server, Procedure.storage("CreateRemote"), request,
                    as: Mavio_Storage_V1_CreateRemoteResponse.self)
                changed()
                if r.hasQuestion { question = r.question } else { created = true }
            } catch {
                failure = error.reason
            }
        }
    }

    /// Cancelling the questions of a new remote deletes it.
    private func cancel() {
        Task {
            var request = Mavio_Storage_V1_DeleteRemoteRequest()
            request.name = trimmed
            _ = try? await model.call(
                server, Procedure.storage("DeleteRemote"), request, as: Mavio_Storage_V1_DeleteRemoteResponse.self)
            changed()
            dismiss()
        }
    }
}

/// A remote's page: its kind's settings, configuring it again, its folders
/// and deleting it; the questions of its configuration take the page while
/// they are answered.
private struct StorageRemoteView: View {
    let server: ServerRecord
    let name: String
    let changed: () -> Void
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss
    @State private var remote: Mavio_Storage_V1_Remote?
    @State private var gone = false
    @State private var provider: Mavio_Storage_V1_Provider?
    @State private var providers: [Mavio_Storage_V1_Provider] = []
    @State private var values: [String: String] = [:]
    @State private var question: Mavio_Storage_V1_ConfigQuestion?
    @State private var saving = false
    @State private var notice: String?
    @State private var failure: String?
    @State private var confirming = false

    var body: some View {
        if let question {
            RemoteQuestionView(
                server: server, name: name, question: question,
                done: {
                    self.question = nil
                    notice = MavioStrings.storageSaved
                    Task { await load() }
                },
                cancel: { self.question = nil })
        } else {
            page
        }
    }

    /// Secrets are never listed: an empty one keeps what is set.
    private static func secret(_ o: Mavio_Storage_V1_Option) -> Bool { o.password || o.sensitive }

    private var changes: [String: String] {
        guard let remote, let provider else { return [:] }
        return values.filter { k, v in
            if let o = provider.options.first(where: { $0.name == k }), Self.secret(o) { return !v.isEmpty }
            return v != (remote.parameters[k] ?? "")
        }
    }

    private var page: some View {
        List {
            if let remote {
                Section {
                } header: {
                    HStack(spacing: MavioSpacing.s12) {
                        ColorTile(icon: "cloud", color: Color(hex: 0x30B0C7), size: 40)
                        VStack(alignment: .leading, spacing: 2) {
                            Text(remote.name).type(theme.type.title3).foregroundStyle(theme.colors.textPrimary)
                            Text(kindName(providers, remote.type)).type(theme.type.footnote)
                                .foregroundStyle(theme.colors.textSecondary)
                        }
                    }
                    .textCase(nil)
                }
                if let provider {
                    OptionsSection(
                        options: provider.options,
                        values: Binding(
                            get: { values },
                            set: {
                                values = $0; notice = nil
                            }),
                        placeholder: { Self.secret($0) ? MavioStrings.storageSecretKept : nil })
                }
                Section {
                    Button(MavioStrings.storageReconfigure, action: reconfigure).disabled(saving)
                }
                RemoteFolders(server: server, remote: remote.name)
                Section {
                    Button(MavioStrings.storageRemove, role: .destructive) { confirming = true }
                        .confirmationDialog(
                            MavioStrings.storageRemoveConfirm(name: remote.name), isPresented: $confirming,
                            titleVisibility: .visible
                        ) {
                            Button(MavioStrings.storageRemove, role: .destructive, action: delete)
                        } message: {
                            Text(MavioStrings.storageRemoveDetail)
                        }
                }
            } else if gone {
                Text(MavioStrings.storageNoRemote).foregroundStyle(theme.colors.textSecondary)
            } else if failure == nil {
                ProgressView().frame(maxWidth: .infinity).listRowBackground(Color.clear)
            }
            if notice != nil && changes.isEmpty || failure != nil {
                Section {
                    if let notice, changes.isEmpty { Text(notice).foregroundStyle(theme.colors.textSecondary) }
                    if let failure { Text(failure).foregroundStyle(theme.colors.danger) }
                }
            }
        }
        .scrollDismissesKeyboard(.interactively)
        .scrollContentBackground(.hidden)
        .background(theme.colors.background)
        .navigationTitle(name)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            if saving {
                ProgressView()
            } else if remote != nil {
                Button(MavioStrings.actionSave, action: save).disabled(changes.isEmpty)
            }
        }
        .task { if remote == nil { await load() } }
    }

    private func load() async {
        do {
            async let p = model.call(
                server, Procedure.storage("ListProviders"), Mavio_Storage_V1_ListProvidersRequest(),
                as: Mavio_Storage_V1_ListProvidersResponse.self)
            let found = try await model.remotes(server).first { $0.name == name }
            providers = try await p.providers
            remote = found
            gone = found == nil
            provider = providers.first { $0.name == found?.type }
            values = found?.parameters ?? [:]
            failure = nil
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
    }

    private func update(_ parameters: [String: String], notice done: String) {
        UIApplication.shared.sendAction(#selector(UIResponder.resignFirstResponder), to: nil, from: nil, for: nil)
        saving = true
        failure = nil
        Task {
            defer { saving = false }
            var request = Mavio_Storage_V1_UpdateRemoteRequest()
            request.name = name
            request.parameters = parameters
            do {
                let r = try await model.call(
                    server, Procedure.storage("UpdateRemote"), request,
                    as: Mavio_Storage_V1_UpdateRemoteResponse.self)
                if r.hasQuestion {
                    question = r.question
                    return
                }
                if r.hasRemote {
                    remote = r.remote
                    values = r.remote.parameters
                } else if let remote {
                    values = remote.parameters
                }
                notice = done
                changed()
            } catch {
                failure = error.reason
            }
        }
    }

    private func save() { update(changes, notice: MavioStrings.storageSaved) }

    private func reconfigure() { update([:], notice: MavioStrings.storageReconfigured) }

    private func delete() {
        Task {
            var request = Mavio_Storage_V1_DeleteRemoteRequest()
            request.name = name
            do {
                _ = try await model.call(
                    server, Procedure.storage("DeleteRemote"), request,
                    as: Mavio_Storage_V1_DeleteRemoteResponse.self)
                changed()
                dismiss()
            } catch {
                failure = error.reason
            }
        }
    }
}

/// A remote's folders, to walk and to see that it is reached.
private struct RemoteFolders: View {
    let server: ServerRecord
    let remote: String
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var path = ""
    @State private var folders: [String]?
    @State private var failure: String?

    private var root: String { "rclone:\(remote):" }

    var body: some View {
        let here = path.isEmpty ? root : path
        Section {
            Text(here).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary).textSelection(.enabled)
            if here != root {
                Button {
                    path = here.contains("/") ? String(here[..<here.lastIndex(of: "/")!]) : root
                } label: {
                    Label(MavioStrings.librariesParentFolder, systemImage: "chevron.backward")
                }
            }
            ForEach(folders ?? [], id: \.self) { f in
                Button {
                    path = f
                } label: {
                    Label {
                        Text(f[f.index(after: f.lastIndex(of: f.contains("/") ? "/" : ":")!)...])
                            .foregroundStyle(theme.colors.textPrimary)
                    } icon: {
                        Icon("folder", size: 18).foregroundStyle(theme.colors.textSecondary)
                    }
                }
            }
            if let failure {
                Text(failure).foregroundStyle(theme.colors.danger)
            } else if folders?.isEmpty == true {
                Text(MavioStrings.librariesNoFolders).foregroundStyle(theme.colors.textSecondary)
            } else if folders == nil {
                ProgressView().frame(maxWidth: .infinity)
            }
        } header: {
            Text(MavioStrings.storageFolders)
        } footer: {
            Text(MavioStrings.storageFoldersFooter)
        }
        .task(id: here) {
            folders = nil
            failure = nil
            do {
                folders = try await model.remoteFolders(server, here)
            } catch is CancellationError {
            } catch {
                failure = error.reason
            }
        }
    }
}

// MARK: Questions

/// A QR code an option's help draws for a terminal: rows of half blocks,
/// each character two modules high, a block where a module is light; dark
/// holds its modules, true where dark.
private struct HelpQR: Equatable {
    var before: String
    var dark: [[Bool]]
    var after: String

    init?(_ help: String) {
        let lines = help.components(separatedBy: "\n")
        let art = { (l: String) in
            l.count >= 21 && l.allSatisfy { " █▀▄".contains($0) } && l.contains { "█▀▄".contains($0) }
        }
        guard let start = lines.firstIndex(where: art) else { return nil }
        var end = start
        while end < lines.count && art(lines[end]) { end += 1 }
        var dark: [[Bool]] = []
        for (i, l) in lines[start..<end].enumerated() {
            let chars = Array(l)
            dark.append(chars.map { $0 != "█" && $0 != "▀" })
            // An odd last row is drawn as the top halves of its line alone.
            if i < end - start - 1 || l.contains(where: { $0 == "█" || $0 == "▄" }) {
                dark.append(chars.map { $0 != "█" && $0 != "▄" })
            }
        }
        guard dark.count >= 21 else { return nil }
        self.before = lines[..<start].joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines)
        self.dark = dark
        self.after = lines[end...].joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines)
    }
}

/// A QR code, drawn dark on light whatever the theme, for a phone to scan.
private struct QRCodeView: View {
    let dark: [[Bool]]

    var body: some View {
        let width = dark.map(\.count).max() ?? 0
        Canvas { context, size in
            let module = min(size.width / CGFloat(width), size.height / CGFloat(dark.count))
            context.fill(Path(CGRect(origin: .zero, size: size)), with: .color(.white))
            var path = Path()
            for (y, row) in dark.enumerated() {
                for (x, on) in row.enumerated() where on {
                    path.addRect(CGRect(x: CGFloat(x) * module, y: CGFloat(y) * module, width: module, height: module))
                }
            }
            context.fill(path, with: .color(.black))
        }
        .aspectRatio(CGFloat(width) / CGFloat(max(dark.count, 1)), contentMode: .fit)
        // A quiet zone around it, as scanners need.
        .padding(12)
        .background(.white)
        .frame(maxWidth: 224)
        .clipShape(.rect(cornerRadius: 8))
        .accessibilityLabel("QR")
    }
}

/// The questions of a remote's configuration — a yes or no, a choice, a
/// value, or the token of authorizing a cloud drive — answered until none
/// is left. A question drawing a QR code, as 115's login asks, is answered
/// yes again and again until the phone confirms the login, its code kept on
/// screen while rclone asks again without it. Cancelling waits for the
/// answer being sent, then calls cancel.
private struct RemoteQuestionView: View {
    let server: ServerRecord
    let name: String
    let done: () -> Void
    let cancel: () -> Void
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var q: Mavio_Storage_V1_ConfigQuestion
    @State private var answer: String
    @State private var scan: (state: String, qr: HelpQR)?
    @State private var pending = false
    @State private var failure: String?
    @State private var cancelled = false

    init(
        server: ServerRecord, name: String, question: Mavio_Storage_V1_ConfigQuestion,
        done: @escaping () -> Void, cancel: @escaping () -> Void
    ) {
        self.server = server
        self.name = name
        self.done = done
        self.cancel = cancel
        self._q = State(initialValue: question)
        self._answer = State(initialValue: question.option.defaultValue)
    }

    private var option: Mavio_Storage_V1_Option? { q.hasOption ? q.option : nil }
    private var drawn: HelpQR? { option.flatMap { HelpQR($0.help) } }
    private var qr: HelpQR? { drawn ?? (scan?.state == q.state ? scan?.qr : nil) }
    private var confirm: Bool { option?.type == "bool" }
    private var waiting: Bool { qr != nil && confirm && failure == nil && !cancelled }

    private struct Loop: Equatable {
        var state: String
        var waiting: Bool
        var pending: Bool
    }

    var body: some View {
        List {
            Section {
                Text(MavioStrings.storageQuestionMessage).type(theme.type.callout)
                    .foregroundStyle(theme.colors.textSecondary)
                    .listRowBackground(Color.clear)
            }
            Section {
                if let o = option {
                    if let qr {
                        if !qr.before.isEmpty { Text(qr.before) }
                        QRCodeView(dark: qr.dark).frame(maxWidth: .infinity)
                        if !cancelled {
                            Text(waiting ? MavioStrings.storageWaitingScan : qr.after)
                                .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                        }
                    } else {
                        let help = o.help.trimmingCharacters(in: .whitespacesAndNewlines)
                        if !help.isEmpty { Text(help).textSelection(.enabled) }
                    }
                    if !confirm {
                        OptionControl(option: o, value: $answer, provider: "")
                    }
                } else {
                    Text(MavioStrings.storageQuestionNone).foregroundStyle(theme.colors.textSecondary)
                }
                if !q.error.isEmpty { Text(q.error).foregroundStyle(theme.colors.danger) }
            } footer: {
                if option?.name == "config_token" { Text(MavioStrings.storageQuestionFooter) }
            }
            Section {
                if !waiting && !cancelled {
                    if confirm {
                        Button(MavioStrings.storageYes) { send("true") }.disabled(pending)
                        Button(MavioStrings.storageNo) { send("false") }.disabled(pending)
                    } else {
                        Button(pending ? "…" : MavioStrings.storageContinue) { send(answer) }.disabled(pending)
                    }
                }
                Button(cancelled ? MavioStrings.storageCancelling : MavioStrings.actionCancel, role: .cancel) {
                    cancelled = true
                    if !pending { cancel() }
                }
                .disabled(cancelled)
                if let failure { Text(failure).foregroundStyle(theme.colors.danger) }
            }
        }
        .scrollDismissesKeyboard(.interactively)
        .scrollContentBackground(.hidden)
        .background(theme.colors.background)
        .navigationTitle(name)
        .navigationBarTitleDisplayMode(.inline)
        .navigationBarBackButtonHidden(pending || waiting)
        // Each answer waits for the last; a code waiting to be scanned is
        // answered yes again every few seconds.
        .task(id: Loop(state: q.state, waiting: waiting, pending: pending)) {
            guard waiting, !pending else { return }
            try? await Task.sleep(for: .seconds(2.5))
            if !Task.isCancelled { send("true") }
        }
    }

    private func send(_ value: String) {
        UIApplication.shared.sendAction(#selector(UIResponder.resignFirstResponder), to: nil, from: nil, for: nil)
        pending = true
        failure = nil
        let drawn = drawn
        Task {
            defer { pending = false }
            var request = Mavio_Storage_V1_ContinueRemoteRequest()
            request.name = name
            request.state = q.state
            request.answer = value
            do {
                let r = try await model.call(
                    server, Procedure.storage("ContinueRemote"), request,
                    as: Mavio_Storage_V1_ContinueRemoteResponse.self)
                if cancelled { return cancel() }
                guard r.hasQuestion else { return done() }
                if let drawn { scan = (q.state, drawn) }
                q = r.question
                answer = r.question.option.defaultValue
            } catch {
                if cancelled { return cancel() }
                failure = error.reason
            }
        }
    }
}
