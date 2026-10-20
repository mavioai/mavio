import MavioKit
import MavioUI
import SwiftUI

/// Where this device keeps its settings.
enum DeviceSetting {
    static let autoplayNext = "player.autoplayNext"
}

/// The quality the player asks the servers for.
enum PlaybackQuality: String, CaseIterable {
    case original, auto
    case p1080 = "1080p"
    case p720 = "720p"
    case p480 = "480p"

    /// Where this device keeps it.
    static let key = "player.quality"

    /// The bitrate it limits the stream to: Auto sends more from a server on
    /// the local network than over the internet.
    func bitrate(local: Bool) -> Int64 {
        switch self {
        case .original: 200_000_000
        case .auto: local ? 100_000_000 : 40_000_000
        case .p1080: 10_000_000
        case .p720: 4_000_000
        case .p480: 1_500_000
        }
    }

    var label: String {
        switch self {
        case .original: MavioStrings.playerQualityOriginal
        case .auto: MavioStrings.playerQualityAuto
        default: rawValue
        }
    }
}

/// The settings (docs/tv.md §6.6): the account and its servers, and this
/// device's playback.
struct SettingsView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @AppStorage(PlaybackQuality.key) private var quality = PlaybackQuality.auto
    @AppStorage(DeviceSetting.autoplayNext) private var autoplayNext = true

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                Text(MavioStrings.navSettings).type(theme.type.title1).foregroundStyle(theme.colors.textPrimary)
                    .padding(.bottom, MavioSpacing.s24)
                Group {
                    Heading(MavioStrings.settingsAccount)
                    if let account = model.account {
                        Text(account.name).type(theme.type.body).foregroundStyle(theme.colors.textSecondary)
                    }
                    ForEach(model.servers, id: \.id) { server in
                        Menu {
                            Button(MavioStrings.authSignOut, role: .destructive) {
                                Task { try? await model.remove(server.id) }
                            }
                        } label: {
                            HStack(spacing: MavioSpacing.s16) {
                                Icon("server", size: 32)
                                VStack(alignment: .leading, spacing: 2) {
                                    Text(server.name).type(theme.type.headline)
                                    Text("\(server.userName) · \(server.address)").type(theme.type.footnote)
                                        .opacity(0.7)
                                }
                                .lineLimit(1)
                                Spacer()
                                if !model.signedIn.contains(server.id) {
                                    Text(MavioStrings.authSignedOut).type(theme.type.footnote)
                                }
                            }
                            .padding(.vertical, MavioSpacing.s12)
                        }
                        .buttonStyle(RowButtonStyle())
                    }
                    Button {
                        Task { try? await model.signOut() }
                    } label: {
                        Label {
                            Text(MavioStrings.authSignOut)
                        } icon: {
                            Icon("signOut", size: 28)
                        }
                    }
                    .buttonStyle(RowButtonStyle())
                }
                Group {
                    Heading(MavioStrings.settingsPlayback)
                    Picker(selection: $quality) {
                        ForEach(PlaybackQuality.allCases, id: \.self) { Text($0.label).tag($0) }
                    } label: {
                        Text(MavioStrings.settingsMaxQuality)
                    }
                    Toggle(MavioStrings.settingsAutoplayNext, isOn: $autoplayNext)
                }
                Text(
                    verbatim:
                        "Mavio \(Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") ?? "") · \(coreVersion)"
                )
                .type(theme.type.caption).foregroundStyle(theme.colors.textTertiary)
                .padding(.top, MavioSpacing.s32)
            }
            .frame(maxWidth: 1100, alignment: .leading)
            .frame(maxWidth: .infinity)
            .padding(.vertical, MavioSpacing.s64)
        }
        .toolbar(.hidden, for: .navigationBar)
    }
}

/// A group's heading.
private struct Heading: View {
    let text: String
    @Environment(\.theme) private var theme

    init(_ text: String) { self.text = text }

    var body: some View {
        Text(text).type(theme.type.title3).foregroundStyle(theme.colors.textSecondary)
            .padding(.top, MavioSpacing.s32)
    }
}
