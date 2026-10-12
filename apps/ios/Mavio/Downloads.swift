import MavioKit
import MavioUI
import SwiftUI

/// The items downloaded for offline playback, with the downloads going on.
struct DownloadsView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @Environment(\.theme) private var theme

    var body: some View {
        let downloads = model.downloads
        List {
            ForEach(downloads.entries, id: \.self) { entry in
                Button {
                    if let file = downloads.file(entry) {
                        router.play(
                            PlayRequest(
                                serverId: entry.card.serverId, itemId: entry.card.itemId, title: entry.card.title,
                                start: entry.card.progress.flatMap { p in entry.card.runtime.map { $0 * Double(p) } },
                                file: file))
                    }
                } label: {
                    DownloadRow(entry: entry)
                }
                .buttonStyle(.plain)
                .swipeActions {
                    Button(MavioStrings.itemRemoveDownload, role: .destructive) {
                        Task { await downloads.remove(entry) }
                    }
                    if entry.state == .paused || entry.state == .failed {
                        Button(MavioStrings.downloadResume) { Task { try? await downloads.resume(entry) } }
                    }
                }
            }
        }
        .listStyle(.plain)
        .overlay {
            if downloads.entries.isEmpty {
                StatusView(
                    icon: "downloads", title: MavioStrings.stateEmpty, message: MavioStrings.downloadEmptyMessage)
            }
        }
        .navigationTitle(MavioStrings.navDownloads)
        .onAppear { downloads.reload() }
    }
}

private struct DownloadRow: View {
    let entry: DownloadEntry
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(spacing: MavioSpacing.s12) {
            ArtworkView(entry.card.poster?.url ?? entry.card.thumb?.url)
                .frame(width: 60, height: 90)
                .clipShape(.rect(cornerRadius: MavioRadius.sm))
            VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                Text(entry.card.title).type(theme.type.headline).lineLimit(2)
                if let subtitle = entry.card.subtitle {
                    Text(subtitle).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                }
                Text(entry.serverName).type(theme.type.caption).foregroundStyle(theme.colors.textTertiary)
                state
            }
            Spacer(minLength: 0)
        }
        .contentShape(.rect)
    }

    @ViewBuilder private var state: some View {
        let size = { (n: UInt64) in ByteCountFormatter.string(fromByteCount: Int64(n), countStyle: .file) }
        switch entry.state {
        case .queued:
            Text(MavioStrings.downloadWaiting).type(theme.type.caption).foregroundStyle(theme.colors.textSecondary)
        case .running:
            VStack(alignment: .leading, spacing: 2) {
                ProgressView(value: entry.total > 0 ? Double(entry.received) / Double(entry.total) : 0)
                Text(MavioStrings.downloadProgress(received: size(entry.received), total: size(entry.total)))
                    .type(theme.type.caption).foregroundStyle(theme.colors.textSecondary)
            }
        case .paused:
            Text(MavioStrings.downloadPaused).type(theme.type.caption).foregroundStyle(theme.colors.textSecondary)
        case .failed:
            Text(MavioStrings.downloadFailed).type(theme.type.caption).foregroundStyle(theme.colors.danger)
        case .done:
            Text(size(entry.total)).type(theme.type.caption).foregroundStyle(theme.colors.textSecondary)
        }
    }
}
