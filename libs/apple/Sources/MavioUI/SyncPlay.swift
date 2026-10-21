import MavioCommon
import MavioKit
import SwiftUI

/// Watch Together (docs/clients.md §6.3.24): the device's group, or a
/// server's groups to join and a new one; in a group on the item's server,
/// the item plays in it from its resume position.
public struct SyncPlayView: View {
    let server: ServerRecord
    let item: (id: String, name: String, position: TimeInterval?)?
    /// Opens the player on what the group plays, at the group's position.
    let watch: (PlayRequest) -> Void
    /// Whether the player shows what the group plays, which needs no Watch.
    let watching: Bool
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss
    @State private var groups: [Mavio_Session_V1_SyncPlayGroup]?
    @State private var name: String
    @State private var busy = false
    @State private var failure: String?
    @State private var entryName = "…"

    /// The dialog of a server's groups; item, when given, plays in the
    /// group, from position when it was partly watched.
    public init(
        server: ServerRecord, item: (id: String, name: String, position: TimeInterval?)? = nil,
        watching: Bool = false, watch: @escaping (PlayRequest) -> Void
    ) {
        (self.server, self.item, self.watching, self.watch) = (server, item, watching, watch)
        _name = State(initialValue: MavioStrings.syncplayDefaultName(name: server.userName))
    }

    /// The view's content.
    public var body: some View {
        NavigationStack {
            Group {
                if let joined = model.syncPlay.joined { group(joined) } else { groupsList }
            }
            .hiddenListBackground()
            .background(theme.colors.background)
            .inlineTitle()
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button(model.syncPlay.joined == nil ? MavioStrings.actionCancel : MavioStrings.actionDone) {
                        dismiss()
                    }
                }
            }
        }
    }

    private func group(_ joined: SyncPlay.Joined) -> some View {
        let group = joined.group
        let entry = group.entry
        return Form {
            Section {
                Text(groupState(group, entryName: entryName)).foregroundStyle(theme.colors.textSecondary)
            }
            Section(MavioStrings.syncplayMembers) {
                ForEach(group.members, id: \.sessionID) { m in
                    HStack(spacing: MavioSpacing.s12) {
                        Icon("devices", size: 20).foregroundStyle(theme.colors.textSecondary)
                        VStack(alignment: .leading, spacing: 2) {
                            Text(m.userName).lineLimit(1)
                            let me = m.sessionID == model.syncPlay.sessionId(joined.serverId)
                            let detail = [m.deviceName, me ? MavioStrings.settingsThisDevice : ""]
                                .filter { !$0.isEmpty }.joined(separator: " · ")
                            if !detail.isEmpty {
                                Text(detail).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                                    .lineLimit(1)
                            }
                        }
                        Spacer()
                        if m.buffering {
                            Text(MavioStrings.syncplayLoading).type(theme.type.footnote)
                                .foregroundStyle(theme.colors.textSecondary)
                        }
                    }
                }
            }
            Section {
                if let entry, group.state != .idle, !watching {
                    Button(MavioStrings.syncplayWatch) {
                        dismiss()
                        watch(
                            PlayRequest(
                                serverId: joined.serverId, itemId: entry.itemID, title: entryName,
                                start: group.position(at: model.syncPlay.serverNow())))
                    }
                }
                if let item, joined.serverId == server.id {
                    Button(MavioStrings.syncplayPlayHere(name: item.name)) {
                        run {
                            try await model.playInGroup(server.id, itemId: item.id, from: item.position ?? 0)
                            dismiss()
                        }
                    }
                }
                Button(MavioStrings.syncplayLeave, role: .destructive) {
                    run {
                        try await model.syncPlay.leave()
                        dismiss()
                    }
                }
            }
            .disabled(busy)
            if let failure { Section { Text(failure).foregroundStyle(theme.colors.danger) } }
        }
        .navigationTitle(group.name)
        .task(id: entry?.itemID) {
            guard let entry else { return }
            if let d = try? await model.client.details(serverId: joined.serverId, itemId: entry.itemID) {
                entryName = d.card.title
            }
        }
    }

    private var groupsList: some View {
        Form {
            Section {
                Text(MavioStrings.syncplayIntro).foregroundStyle(theme.colors.textSecondary)
            }
            Section {
                if let groups {
                    if groups.isEmpty {
                        Text(MavioStrings.syncplayNoGroups).foregroundStyle(theme.colors.textSecondary)
                    }
                    ForEach(groups, id: \.id) { g in
                        HStack(spacing: MavioSpacing.s12) {
                            Icon("users", size: 20).foregroundStyle(theme.colors.textSecondary)
                            VStack(alignment: .leading, spacing: 2) {
                                Text(g.name).lineLimit(1)
                                Text(g.members.map(\.userName).joined(separator: ", "))
                                    .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                                    .lineLimit(1)
                            }
                            Spacer()
                            Button(MavioStrings.syncplayJoin) {
                                run { try await model.joinSyncPlayGroup(server.id, groupId: g.id) }
                            }
                            .disabled(busy)
                        }
                    }
                } else {
                    ProgressView()
                }
            }
            Section {
                TextField(MavioStrings.syncplayGroupName, text: $name)
                Button(MavioStrings.syncplayCreate) {
                    let name = String(name.trimmingCharacters(in: .whitespaces).prefix(200))
                    run { try await model.createSyncPlayGroup(server.id, name: name) }
                }
                .disabled(busy || name.trimmingCharacters(in: .whitespaces).isEmpty)
            }
            if let failure { Section { Text(failure).foregroundStyle(theme.colors.danger) } }
        }
        .navigationTitle(MavioStrings.syncplayTitle)
        .task {
            do {
                groups = try await model.syncPlayGroups(server.id)
            } catch {
                groups = []
                failure = error.reason
            }
        }
    }

    private func run(_ action: @escaping () async throws -> Void) {
        busy = true
        failure = nil
        Task {
            do { try await action() } catch { failure = error.reason }
            busy = false
        }
    }
}

/// The device's group, floating at the bottom right of the screens while it
/// is in one, as the web UI's; it opens the group's dialog.
public struct GroupIndicator: View {
    let lift: CGFloat
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @Environment(\.theme) private var theme
    @State private var open = false

    /// The indicator, lift points above the bottom, over a tab bar.
    public init(lift: CGFloat = 0) { self.lift = lift }

    /// The view's content.
    public var body: some View {
        if let joined = model.syncPlay.joined {
            Button {
                open = true
            } label: {
                Label {
                    Text(groupChip(joined.group)).lineLimit(1)
                } icon: {
                    Icon("users", fill: true, size: 18)
                }
                .type(theme.type.footnote).fontWeight(.semibold)
                .foregroundStyle(theme.colors.textPrimary)
                .padding(.horizontal, MavioSpacing.s16)
                .frame(height: 40)
                .glass(in: .capsule)
            }
            .buttonStyle(.plain)
            .padding(.trailing, MavioSpacing.s16)
            .padding(.bottom, MavioSpacing.s16 + lift)
            .accessibilityLabel(MavioStrings.syncplayTitle)
            .sheet(isPresented: $open) {
                if let server = model.server(joined.serverId) {
                    SyncPlayView(server: server) { router.play($0) }.environment(\.theme, theme)
                }
            }
        }
    }
}
