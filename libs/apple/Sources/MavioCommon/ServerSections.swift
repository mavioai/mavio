import Foundation
import MavioDesign
import MavioKit

/// An entry of the Server group: a section, or a menu of them.
public enum ServerEntry: Hashable {
    case section(ServerSection)
    case menu(ServerMenu)

    /// The entries the user sees: an administrator's all, others' the
    /// sections any user has; a menu left with one section is that section.
    public static func shown(admin: Bool) -> [ServerEntry] {
        #if os(macOS)
            // The Mac, not in the App Store, keeps acquiring media, the AI
            // providers, the plugins and the proxies.
            let top: [ServerEntry] = [
                .section(.overview), .section(.general), .section(.users), .section(.devices), .menu(.media),
                .section(.acquisition), .section(.wanted), .section(.downloads), .section(.subtitles), .section(.ai),
                .section(.plugins), .section(.proxies), .menu(.maintenance),
            ]
        #else
            let top: [ServerEntry] = [
                .section(.overview), .section(.general), .section(.users), .section(.devices), .menu(.media),
                .menu(.maintenance),
            ]
        #endif
        return top.compactMap { entry in
            switch entry {
            case .section(let s): return s.forAnyone || admin ? entry : nil
            case .menu(let m):
                let sections = m.sections(admin: admin)
                return sections.count > 1 ? entry : sections.first.map(ServerEntry.section)
            }
        }
    }

    /// The title.
    public var title: String {
        switch self {
        case .section(let s): s.title
        case .menu(let m): m.title
        }
    }

    /// The icon's name.
    public var icon: String {
        switch self {
        case .section(let s): s.icon
        case .menu(let m): m.icon
        }
    }

    /// The color of the icon's tile, as an sRGB hex value.
    public var tint: UInt32 {
        switch self {
        case .section(let s): s.tint
        case .menu(let m): m.tint
        }
    }

}

/// A menu of the Server group's second level.
public enum ServerMenu: Hashable {
    case media, maintenance

    /// The menu's sections the user sees.
    public func sections(admin: Bool) -> [ServerSection] {
        let all: [ServerSection] =
            switch self {
            case .media: [.libraries, .storage, .transcoding]
            case .maintenance: [.activity, .issues, .tasks, .logs, .backups, .apiKeys]
            }
        return all.filter { admin || $0.forAnyone }
    }

    /// The title.
    public var title: String {
        switch self {
        case .media: MavioStrings.settingsMediaMenu
        case .maintenance: MavioStrings.settingsMaintenanceMenu
        }
    }

    /// The icon's name.
    public var icon: String {
        switch self {
        case .media: "movies"
        case .maintenance: "admin"
        }
    }

    /// The color of the icon's tile, as an sRGB hex value.
    public var tint: UInt32 {
        switch self {
        case .media: 0xFF9F0A
        case .maintenance: 0x8E8E93
        }
    }
}

/// A section of the Server group.
public enum ServerSection: Hashable {
    case overview, general, users, devices, apiKeys, libraries, storage, transcoding, activity, issues, tasks, logs,
        backups
    /// The sections only the Mac shows, which the App Store does not
    /// accept (docs/clients.md §4.2): acquiring media, the AI providers
    /// and conversations, the plugins, the proxies and the indexer search.
    case acquisition, wanted, downloads, subtitles, ai, plugins, proxies, conversations, indexerSearch

    /// Whether users who do not administer the server see it.
    public var forAnyone: Bool { self == .devices || self == .issues }

    /// The title.
    public var title: String {
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
        case .acquisition: MavioStrings.settingsAcquisition
        case .wanted: MavioStrings.settingsWanted
        case .downloads: MavioStrings.settingsDownloads
        case .subtitles: MavioStrings.settingsSubtitles
        case .ai: MavioStrings.settingsAi
        case .plugins: MavioStrings.settingsPlugins
        case .proxies: MavioStrings.settingsProxies
        case .conversations: MavioStrings.settingsConversations
        case .indexerSearch: MavioStrings.indexersTitle
        }
    }

    /// The icon's name.
    public var icon: String {
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
        case .acquisition, .downloads: "downloads"
        case .wanted: "pending"
        case .subtitles: "subtitles"
        case .ai: "key"
        case .plugins: "plugins"
        case .proxies: "network"
        case .conversations: "info"
        case .indexerSearch: "indexers"
        }
    }

    /// The color of the icon's tile, as an sRGB hex value.
    public var tint: UInt32 {
        switch self {
        case .overview, .general, .logs: 0x8E8E93
        case .apiKeys: 0x636366
        case .users: 0xFF9500
        case .libraries: 0x007AFF
        case .storage: 0x30B0C7
        case .transcoding: 0xFF2D55
        case .devices: 0x32ADE6
        case .activity: 0xFF2D55
        case .issues: 0xFF9500
        case .tasks, .backups: 0x5856D6
        case .acquisition, .downloads: 0x34C759
        case .wanted: 0xFF3B30
        case .subtitles: 0xFF9500
        case .ai: 0xBF5AF2
        case .plugins: 0xAF52DE
        case .proxies: 0x30B0C7
        case .conversations: 0x5E5CE6
        case .indexerSearch: 0x0A84FF
        }
    }

}

/// When something happened or will, relative to now, as "5 minutes ago".
public func fromNow(_ date: Date) -> String {
    date.formatted(.relative(presentation: .named))
}

/// A length of time, as "1 hr, 30 min".
public func lengthOf(_ seconds: TimeInterval) -> String {
    Duration.seconds(seconds).formatted(
        .units(allowed: [.days, .hours, .minutes, .seconds], width: .abbreviated, maximumUnitCount: 2))
}
