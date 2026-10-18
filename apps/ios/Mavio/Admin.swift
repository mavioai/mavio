import MavioKit
import MavioUI
import SwiftUI

/// The paths of the server's procedures the settings call.
enum Procedure {
    static func system(_ method: String) -> String { "/mavio.system.v1.SystemService/\(method)" }
    static func plugins(_ method: String) -> String { "/mavio.system.v1.PluginManagementService/\(method)" }
    static func users(_ method: String) -> String { "/mavio.user.v1.UserService/\(method)" }
    static func activity(_ method: String) -> String { "/mavio.system.v1.ActivityService/\(method)" }
    static func tasks(_ method: String) -> String { "/mavio.system.v1.TaskService/\(method)" }
    static func backups(_ method: String) -> String { "/mavio.system.v1.BackupService/\(method)" }
    static func sessions(_ method: String) -> String { "/mavio.session.v1.SessionService/\(method)" }
    static func issues(_ method: String) -> String { "/mavio.acquisition.v1.IssueService/\(method)" }
    static func items(_ method: String) -> String { "/mavio.library.v1.ItemService/\(method)" }
}

/// The settings' Server group, as the web UI's on a phone (docs/clients.md
/// §6.3): the picker of the servers signed in to, then the sections of the
/// server chosen, in the order a server is set up and kept — its
/// administrators' with the devices signed in as the user among them, and
/// the menus of a second level. The sections the App Store does not accept
/// are left out (docs/clients.md §4.2).
struct ServerGroup: View {
    let server: ServerRecord
    @Binding var serverId: String
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    /// Whether the user administers the server, once known.
    @State private var admin: [String: Bool] = [:]

    var body: some View {
        Section {
            if model.signedInServers.count > 1 {
                Picker(MavioStrings.authServer, selection: Binding(get: { server.id }, set: { serverId = $0 })) {
                    ForEach(model.signedInServers, id: \.id) { Text($0.name).tag($0.id) }
                }
                .tint(theme.colors.textSecondary)
            }
            ForEach(ServerEntry.shown(admin: admin[server.id] == true), id: \.self) { entry in
                NavigationLink {
                    entry.view(server)
                } label: {
                    EntryLabel(entry.title, icon: entry.icon, color: entry.color)
                }
            }
        } header: {
            Text(MavioStrings.settingsServerGroup)
        }
        .task(id: server.id) { await loadAdmin() }
    }

    private func loadAdmin() async {
        let response = try? await model.client.call(
            server.id, Procedure.users("GetCurrentUser"), Mavio_User_V1_GetCurrentUserRequest(),
            as: Mavio_User_V1_GetCurrentUserResponse.self)
        if let response { admin[server.id] = response.user.admin }
    }
}

/// An entry of the Server group: a section, or a menu of them.
enum ServerEntry: Hashable {
    case section(ServerSection)
    case menu(ServerMenu)

    /// The entries the user sees: an administrator's all, others' the
    /// sections any user has; a menu left with one section is that section.
    static func shown(admin: Bool) -> [ServerEntry] {
        let top: [ServerEntry] = [
            .section(.overview), .section(.general), .section(.users), .section(.devices), .section(.apiKeys),
            .menu(.media),
            .menu(.maintenance),
        ]
        return top.compactMap { entry in
            switch entry {
            case .section(let s): return s.forAnyone || admin ? entry : nil
            case .menu(let m):
                let sections = m.sections(admin: admin)
                return sections.count > 1 ? entry : sections.first.map(ServerEntry.section)
            }
        }
    }

    var title: String {
        switch self {
        case .section(let s): s.title
        case .menu(let m): m.title
        }
    }

    var icon: String {
        switch self {
        case .section(let s): s.icon
        case .menu(let m): m.icon
        }
    }

    var color: Color {
        switch self {
        case .section(let s): s.color
        case .menu(let m): m.color
        }
    }

    @MainActor @ViewBuilder func view(_ server: ServerRecord) -> some View {
        switch self {
        case .section(let s): s.view(server)
        case .menu(let m): ServerMenuView(menu: m, server: server)
        }
    }
}

/// A menu of the Server group's second level.
enum ServerMenu: Hashable {
    case media, maintenance

    func sections(admin: Bool) -> [ServerSection] {
        let all: [ServerSection] =
            switch self {
            case .media: [.libraries, .storage, .transcoding]
            case .maintenance: [.activity, .issues, .tasks, .logs, .backups]
            }
        return all.filter { admin || $0.forAnyone }
    }

    var title: String {
        switch self {
        case .media: MavioStrings.settingsMediaMenu
        case .maintenance: MavioStrings.settingsMaintenanceMenu
        }
    }

    var icon: String {
        switch self {
        case .media: "movies"
        case .maintenance: "admin"
        }
    }

    var color: Color {
        switch self {
        case .media: Color(hex: 0xFF9F0A)
        case .maintenance: Color(hex: 0x8E8E93)
        }
    }
}

/// A menu's page: its sections, as the Server group lists its own.
private struct ServerMenuView: View {
    let menu: ServerMenu
    let server: ServerRecord
    @Environment(\.theme) private var theme

    var body: some View {
        List {
            // Only an administrator sees a menu: one section is shown alone.
            ForEach(menu.sections(admin: true), id: \.self) { section in
                NavigationLink {
                    section.view(server)
                } label: {
                    EntryLabel(section.title, icon: section.icon, color: section.color)
                }
            }
        }
        .scrollContentBackground(.hidden)
        .background(theme.colors.background)
        .navigationTitle(menu.title)
        .navigationBarTitleDisplayMode(.inline)
    }
}

/// A section of the Server group.
enum ServerSection: Hashable {
    case overview, general, users, devices, apiKeys, libraries, storage, transcoding, activity, issues, tasks, logs,
        backups

    /// Whether users who do not administer the server see it.
    var forAnyone: Bool { self == .devices || self == .issues }

    var title: String {
        switch self {
        case .overview: MavioStrings.settingsOverview
        case .general: MavioStrings.settingsGeneral
        case .users: MavioStrings.settingsUsers
        case .apiKeys: MavioStrings.settingsApiKeys
        case .libraries: MavioStrings.settingsLibraries
        case .storage: MavioStrings.settingsStorage
        case .transcoding: MavioStrings.settingsTranscoding
        case .devices: MavioStrings.settingsDevices
        case .activity: MavioStrings.settingsActivity
        case .issues: MavioStrings.settingsIssues
        case .tasks: MavioStrings.settingsTasks
        case .logs: MavioStrings.settingsLogs
        case .backups: MavioStrings.settingsBackups
        }
    }

    var icon: String {
        switch self {
        case .overview: "server"
        case .general: "settings"
        case .users: "users"
        case .apiKeys: "lock"
        case .libraries: "folder"
        case .storage: "server"
        case .transcoding: "quality"
        case .devices: "devices"
        case .activity: "activity"
        case .issues: "error"
        case .tasks: "pending"
        case .logs: "info"
        case .backups: "folder"
        }
    }

    var color: Color {
        switch self {
        case .overview, .general, .logs: Color(hex: 0x8E8E93)
        case .apiKeys: Color(hex: 0x636366)
        case .users: Color(hex: 0xFF9500)
        case .libraries: Color(hex: 0x007AFF)
        case .storage: Color(hex: 0x30B0C7)
        case .transcoding: Color(hex: 0xFF2D55)
        case .devices: Color(hex: 0x32ADE6)
        case .activity: Color(hex: 0xFF2D55)
        case .issues: Color(hex: 0xFF9500)
        case .tasks, .backups: Color(hex: 0x5856D6)
        }
    }

    @MainActor @ViewBuilder func view(_ server: ServerRecord) -> some View {
        switch self {
        case .overview: OverviewView(server: server)
        case .general: GeneralView(server: server)
        case .users: UsersView(server: server)
        case .apiKeys: ApiKeysView(server: server)
        case .libraries: ManagedLibrariesView(server: server)
        case .storage: StorageView(server: server)
        case .transcoding: TranscodingView(server: server)
        case .devices: DevicesView(server: server)
        case .activity: ActivityView(server: server)
        case .issues: IssuesView(server: server)
        case .tasks: TasksView(server: server)
        case .logs: LogsView(server: server)
        case .backups: BackupsView(server: server)
        }
    }
}

extension AppModel {
    /// Calls a procedure of a server as its session.
    func call<Response: SwiftProtobuf.Message>(
        _ server: ServerRecord, _ procedure: String, _ request: some SwiftProtobuf.Message,
        as _: Response.Type = Response.self
    ) async throws -> Response {
        try await client.call(server.id, procedure, request, as: Response.self)
    }
}

/// When something happened or will, relative to now, as "5 minutes ago".
func fromNow(_ date: Date) -> String {
    date.formatted(.relative(presentation: .named))
}

/// A length of time, as "1 hr, 30 min".
func lengthOf(_ seconds: TimeInterval) -> String {
    Duration.seconds(seconds).formatted(
        .units(allowed: [.days, .hours, .minutes, .seconds], width: .abbreviated, maximumUnitCount: 2))
}

// MARK: Overview

/// Overview (docs/clients.md §6.3.6): whether the server serves, its
/// version and what it runs on, and the load of its computer, sampled every
/// two seconds while the page shows.
struct OverviewView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var info: Mavio_System_V1_GetSystemInfoResponse?
    @State private var serving: Bool?
    @State private var runtime: Mavio_System_V1_ContainerRuntime?
    @State private var samples: [Mavio_System_V1_LoadSample] = []
    @State private var failure: String?
    @State private var loadFailure: String?

    var body: some View {
        List {
            Section(MavioStrings.overviewServer) {
                LabeledContent(MavioStrings.overviewStatus) {
                    HStack(spacing: MavioSpacing.s8) {
                        Circle()
                            .fill(
                                serving == nil
                                    ? theme.colors.fill : serving == true ? Color(hex: 0x34C759) : theme.colors.danger
                            )
                            .frame(width: 8, height: 8)
                        Text(
                            serving == nil
                                ? "…" : serving == true ? MavioStrings.overviewServing : MavioStrings.overviewNotServing
                        )
                    }
                }
                if let info {
                    fact(MavioStrings.overviewVersion, info.version)
                    fact(MavioStrings.overviewPlatform, "\(info.os)/\(info.arch)")
                    fact(MavioStrings.overviewDatabase, Self.database(info.database))
                    fact(
                        MavioStrings.overviewFfmpeg,
                        info.ffmpegVersion.isEmpty ? MavioStrings.overviewFfmpegMissing : info.ffmpegVersion)
                }
                if let runtime {
                    LabeledContent(MavioStrings.overviewContainerEngine) {
                        VStack(alignment: .trailing, spacing: 2) {
                            Text(Self.engine(runtime))
                            if runtime.available {
                                Text(runtime.endpoint).type(theme.type.footnote)
                            }
                        }
                        .multilineTextAlignment(.trailing)
                    }
                }
                if let start = info?.startTime.time {
                    fact(
                        MavioStrings.overviewUptime,
                        Duration.seconds(Date.now.timeIntervalSince(start)).formatted(
                            .units(allowed: [.days, .hours, .minutes], width: .narrow, maximumUnitCount: 2)))
                }
                if let failure {
                    Text(failure).foregroundStyle(theme.colors.danger)
                }
            }
            Section {
                if let loadFailure {
                    Text(loadFailure).foregroundStyle(theme.colors.danger)
                } else {
                    LoadCards(samples: samples)
                }
            } header: {
                Text(MavioStrings.overviewLoad)
            } footer: {
                Text(MavioStrings.overviewLoadFooter)
            }
        }
        .scrollContentBackground(.hidden)
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsOverview)
        .navigationBarTitleDisplayMode(.inline)
        .task { await loadInfo() }
        .task {
            while !Task.isCancelled {
                await loadSamples()
                try? await Task.sleep(for: .seconds(2))
            }
        }
    }

    private func fact(_ label: String, _ value: String) -> some View {
        LabeledContent(label) { Text(value).textSelection(.enabled) }
    }

    private func loadInfo() async {
        let client = model.client
        async let health = try? client.call(
            server.id, Procedure.system("GetHealth"), Mavio_System_V1_GetHealthRequest(),
            as: Mavio_System_V1_GetHealthResponse.self)
        async let engine = try? client.call(
            server.id, Procedure.plugins("GetContainerRuntime"), Mavio_System_V1_GetContainerRuntimeRequest(),
            as: Mavio_System_V1_GetContainerRuntimeResponse.self)
        do {
            info = try await client.call(
                server.id, Procedure.system("GetSystemInfo"), Mavio_System_V1_GetSystemInfoRequest(),
                as: Mavio_System_V1_GetSystemInfoResponse.self)
            failure = nil
        } catch {
            failure = error.reason
        }
        serving = await health.map { $0.status == .serving } ?? false
        runtime = await engine?.runtime
    }

    private func loadSamples() async {
        do {
            samples = try await model.client.call(
                server.id, Procedure.system("GetSystemLoad"), Mavio_System_V1_GetSystemLoadRequest(),
                as: Mavio_System_V1_GetSystemLoadResponse.self
            ).samples
            loadFailure = nil
        } catch is CancellationError {
        } catch {
            loadFailure = error.reason
        }
    }

    private static func database(_ name: String) -> String {
        switch name {
        case "postgres": "PostgreSQL"
        case "sqlite": "SQLite"
        default: name
        }
    }

    private static func engine(_ r: Mavio_System_V1_ContainerRuntime) -> String {
        guard r.available else { return MavioStrings.overviewContainerEngineNone(reason: r.reason) }
        let name = ["docker": "Docker", "podman": "Podman"][r.engine] ?? r.engine
        return [
            "\(name) \(r.version)", "API \(r.apiVersion)",
            r.rootless ? MavioStrings.overviewContainerEngineRootless : "",
        ]
        .filter { !$0.isEmpty }.joined(separator: " · ")
    }
}

extension Error {
    /// What went wrong, as the server or the core told it.
    var reason: String { (self as? CoreError)?.reason ?? localizedDescription }
}

/// The cards of the load of the server's computer: its CPU, memory, disks
/// and network, each with what it is now, three figures more and a graph of
/// the last two minutes.
private struct LoadCards: View {
    let samples: [Mavio_System_V1_LoadSample]

    private var now: Mavio_System_V1_LoadSample? { samples.last }

    var body: some View {
        cpu
        memory
        disk
        network
    }

    private var cpu: some View {
        let c = now.flatMap { $0.hasCpu ? $0.cpu : nil }
        return LoadCard(
            icon: "cpu", color: Color(hex: 0x007AFF), title: MavioStrings.overviewCpu,
            told: now == nil || c != nil,
            value: c.map { Self.percent($0.user + $0.system) },
            stats: [
                (MavioStrings.overviewCpuUser, c.map { Self.percent($0.user) }),
                (MavioStrings.overviewCpuSystem, c.map { Self.percent($0.system) }),
                (MavioStrings.overviewByServer, c.map { Self.percent($0.server) }),
            ],
            graph: samples.map { $0.hasCpu ? $0.cpu.user + $0.cpu.system : nil })
    }

    private var memory: some View {
        let m = now.flatMap { $0.hasMemory ? $0.memory : nil }
        let share = m.flatMap { $0.totalBytes > 0 ? Double($0.usedBytes) / Double($0.totalBytes) : nil }
        return LoadCard(
            icon: "memory", color: Color(hex: 0x34C759), title: MavioStrings.overviewMemory,
            badge: share.map { $0 >= 0.9 },
            told: now == nil || m != nil,
            value: m.map { Self.bytes($0.usedBytes) },
            after: m.map { MavioStrings.overviewMemoryOf(total: Self.bytes($0.totalBytes)) },
            stats: [
                (MavioStrings.overviewMemoryAvailable, m.map { Self.bytes($0.totalBytes - $0.usedBytes) }),
                (MavioStrings.overviewSwap, m.map { $0.swapTotalBytes > 0 ? Self.bytes($0.swapUsedBytes) : "–" }),
                (MavioStrings.overviewByServer, m.map { Self.bytes($0.serverBytes) }),
            ],
            graph: samples.map {
                $0.hasMemory && $0.memory.totalBytes > 0
                    ? Double($0.memory.usedBytes) / Double($0.memory.totalBytes) : nil
            })
    }

    private var disk: some View {
        let d = now.flatMap { $0.hasDisk ? $0.disk : nil }
        return LoadCard(
            icon: "disk", color: Color(hex: 0xFF9500), title: MavioStrings.overviewDisk,
            told: now == nil || d != nil,
            value: d.map { $0.hasTotalBytes ? Self.bytes($0.availableBytes) : "–" },
            after: d.flatMap {
                $0.hasTotalBytes ? MavioStrings.overviewDiskAvailable(total: Self.bytes($0.totalBytes)) : nil
            },
            stats: [
                (MavioStrings.overviewRead, d.map { $0.hasReadRate ? Self.rate($0.readRate) : "–" }),
                (MavioStrings.overviewWrite, d.map { $0.hasWriteRate ? Self.rate($0.writeRate) : "–" }),
                (MavioStrings.overviewWritten, d.map { $0.hasWrittenBytes ? Self.bytes($0.writtenBytes) : "–" }),
            ],
            graph: Self.scaled(samples.map { $0.hasDisk ? $0.disk.readRate + $0.disk.writeRate : nil }, floor: 1e6))
    }

    private var network: some View {
        let n = now.flatMap { $0.hasNetwork ? $0.network : nil }
        return LoadCard(
            icon: "network", color: Color(hex: 0x30B0C7), title: MavioStrings.overviewNetwork,
            told: now == nil || n != nil,
            value: n.map { Self.rate($0.receiveRate) },
            after: n.map { _ in MavioStrings.overviewDownload },
            stats: [
                (MavioStrings.overviewUpload, n.map { Self.rate($0.sendRate) }),
                (MavioStrings.overviewReceived, n.map { Self.bytes($0.receivedBytes) }),
                (MavioStrings.overviewSent, n.map { Self.bytes($0.sentBytes) }),
            ],
            graph: Self.scaled(
                samples.map { $0.hasNetwork ? $0.network.receiveRate + $0.network.sendRate : nil }, floor: 1e5))
    }

    static func percent(_ share: Double) -> String {
        share.formatted(.percent.precision(.fractionLength(0)))
    }

    static func bytes(_ count: Int64) -> String {
        count.formatted(.byteCount(style: .decimal))
    }

    /// Bytes per second in kilobytes, megabytes or gigabytes as it is large,
    /// a decimal only below ten.
    static func rate(_ bytes: Double) -> String {
        let (n, unit): (Double, UnitInformationStorage) =
            bytes >= 1e9
            ? (bytes / 1e9, .gigabytes) : bytes >= 1e6 ? (bytes / 1e6, .megabytes) : (bytes / 1e3, .kilobytes)
        let value = Measurement(value: n, unit: unit).formatted(
            .measurement(
                width: .abbreviated, usage: .asProvided,
                numberFormatStyle: .number.precision(.fractionLength(n < 10 ? 1 : 0))))
        return "\(value)/s"
    }

    /// Rates as shares of the highest of them, or of floor when all are lower.
    static func scaled(_ values: [Double?], floor: Double) -> [Double?] {
        let top = max(floor, values.compactMap(\.self).max() ?? 0)
        return values.map { $0.map { $0 / top } }
    }
}

/// A card of the load: its tile and title, what it is now, large, three
/// figures more and the graph of the last samples. told is false where the
/// server's system does not tell it; while no sample came, the figures wait.
private struct LoadCard: View {
    let icon: String
    let color: Color
    let title: String
    /// Whether the memory is high, for its badge.
    var badge: Bool? = nil
    let told: Bool
    let value: String?
    var after: String? = nil
    let stats: [(String, String?)]
    let graph: [Double?]
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s12) {
            HStack(spacing: MavioSpacing.s8) {
                Icon(icon, fill: true, size: 14)
                    .foregroundStyle(.white)
                    .frame(width: 24, height: 24)
                    .background(color, in: .rect(cornerRadius: 6))
                Text(title).type(theme.type.headline).foregroundStyle(color)
                Spacer(minLength: 0)
                if let badge {
                    Text(badge ? MavioStrings.overviewMemoryHigh : MavioStrings.overviewMemoryNormal)
                        .type(theme.type.caption)
                        .padding(.horizontal, MavioSpacing.s8)
                        .padding(.vertical, 2)
                        .foregroundStyle(badge ? Color(hex: 0xC93400) : Color(hex: 0x248A3D))
                        .background((badge ? Color(hex: 0xFF9500) : Color(hex: 0x34C759)).opacity(0.15), in: .capsule)
                }
            }
            if told {
                HStack(alignment: .firstTextBaseline, spacing: MavioSpacing.s8) {
                    Text(value ?? "…")
                        .type(theme.type.title2).fontWeight(.semibold).monospacedDigit()
                        .foregroundStyle(value == nil ? theme.colors.textTertiary : theme.colors.textPrimary)
                    if value != nil, let after {
                        Text(after).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    }
                }
                HStack(alignment: .top, spacing: MavioSpacing.s8) {
                    ForEach(stats, id: \.0) { label, v in
                        VStack(alignment: .leading, spacing: 2) {
                            Text(label).type(theme.type.caption).foregroundStyle(theme.colors.textSecondary)
                            Text(v ?? "…").type(theme.type.footnote).monospacedDigit()
                        }
                        .lineLimit(1)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    }
                }
                Dots(values: graph, color: color)
            } else {
                Text(MavioStrings.overviewLoadUnknown).type(theme.type.callout).foregroundStyle(
                    theme.colors.textSecondary)
            }
        }
        .padding(.vertical, MavioSpacing.s4)
        .accessibilityElement(children: .combine)
    }
}

/// A graph of dots: a column per sample, the latest at the right, lit from
/// the bottom as high as its value, from 0 to 1; columns without a sample
/// stay unlit.
private struct Dots: View {
    let values: [Double?]
    let color: Color
    @Environment(\.theme) private var theme

    /// The samples a graph has room for: two minutes', as the server keeps.
    private static let columns = 60
    private static let rows = 6

    var body: some View {
        Canvas { context, size in
            let cell = size.width / CGFloat(Self.columns)
            let shown = values.suffix(Self.columns)
            let offset = Self.columns - shown.count
            let unlit = theme.colors.textSecondary.opacity(0.12)
            for x in 0..<Self.columns {
                let v = x >= offset ? shown[shown.startIndex + x - offset] : nil
                // A value above nothing lights a dot, so that activity shows.
                let lit = v.map { $0 > 0 ? max(1, Int((min(1, $0) * Double(Self.rows)).rounded())) : 0 } ?? 0
                for y in 0..<Self.rows {
                    let on = Self.rows - y <= lit
                    let r = cell * 0.28
                    let center = CGPoint(x: (CGFloat(x) + 0.5) * cell, y: (CGFloat(y) + 0.5) * cell)
                    context.fill(
                        Path(ellipseIn: CGRect(x: center.x - r, y: center.y - r, width: r * 2, height: r * 2)),
                        with: .color(on ? color : unlit))
                }
            }
        }
        .aspectRatio(CGFloat(Self.columns) / CGFloat(Self.rows), contentMode: .fit)
        .accessibilityHidden(true)
    }
}
