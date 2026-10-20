import MavioKit
import MavioUI
import SwiftUI

/// Appearance, as the web UI's: the theme as three pictures, and the
/// language, which iOS sets for each app in the system's Settings.
public struct AppearanceView: View {
    @AppStorage(DeviceSetting.appearance) private var appearance = Appearance.system
    @Environment(\.theme) private var theme
    @Environment(\.openURL) private var openURL

    /// Appearance of this device.
    public init() {}

    /// The view's content.
    public var body: some View {
        List {
            Section {
                HStack(alignment: .top, spacing: MavioSpacing.s16) {
                    ForEach(Appearance.allCases, id: \.self) { option in
                        Button {
                            appearance = option
                        } label: {
                            VStack(spacing: MavioSpacing.s8) {
                                ThemePreview(appearance: option, on: option == appearance)
                                Text(option.label)
                                    .type(theme.type.callout)
                                    .fontWeight(option == appearance ? .semibold : .regular)
                                    .foregroundStyle(
                                        option == appearance ? theme.colors.textPrimary : theme.colors.textSecondary)
                            }
                        }
                        .buttonStyle(.plain)
                        .accessibilityAddTraits(option == appearance ? .isSelected : [])
                    }
                }
                .padding(.vertical, MavioSpacing.s8)
            } footer: {
                Text(MavioStrings.settingsAppearanceMessage)
            }
            Section {
                Button {
                    if let url = URL(string: Self.languageSettings) { openURL(url) }
                } label: {
                    LabeledContent(MavioStrings.settingsLanguage) {
                        HStack(spacing: MavioSpacing.s4) {
                            Text(Self.language)
                            Icon("external", size: 16)
                        }
                    }
                }
                .tint(theme.colors.textPrimary)
            }
        }
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsAppearance)
        .inlineTitle()
    }

    /// Where the system sets the app's language: its page of the Settings
    /// app, or the Mac's Language & Region.
    private static var languageSettings: String {
        #if os(macOS)
            "x-apple.systempreferences:com.apple.Localization-Settings.extension"
        #else
            UIApplication.openSettingsURLString
        #endif
    }

    /// The language the app shows, in itself.
    private static var language: String {
        let code = Bundle.main.preferredLocalizations.first ?? "en"
        return Locale(identifier: code).localizedString(forIdentifier: code)?.capitalized(
            with: Locale(identifier: code))
            ?? code
    }
}

/// A small window in a theme, as the web UI draws it; System shows both
/// halves.
private struct ThemePreview: View {
    let appearance: Appearance
    let on: Bool
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(spacing: 0) {
            switch appearance {
            case .system:
                pane(dark: false)
                pane(dark: true)
            case .light: pane(dark: false)
            case .dark: pane(dark: true)
            }
        }
        .aspectRatio(16 / 10, contentMode: .fit)
        .clipShape(.rect(cornerRadius: 12))
        .overlay {
            RoundedRectangle(cornerRadius: 12)
                .strokeBorder(on ? theme.colors.accent : theme.colors.border, lineWidth: on ? 2 : 1)
        }
    }

    private func pane(dark: Bool) -> some View {
        GeometryReader { geo in
            HStack(spacing: 6) {
                RoundedRectangle(cornerRadius: 4)
                    .fill(Color(hex: dark ? 0x2C2C2E : 0xFFFFFF))
                    .frame(width: (geo.size.width - 16) / 4)
                VStack(alignment: .leading, spacing: 6) {
                    RoundedRectangle(cornerRadius: 4)
                        .fill(
                            LinearGradient(
                                colors: [Color(hex: 0xFFD34E), Color(hex: 0xFF9A2E), Color(hex: 0xF0412F)],
                                startPoint: .topLeading, endPoint: .bottomTrailing)
                        )
                        .frame(height: (geo.size.height - 16) / 2)
                    Capsule()
                        .fill(Color(hex: dark ? 0x3A3A3C : 0xD1D1D6))
                        .frame(width: (geo.size.width - 16) * 0.5, height: 6)
                }
                .frame(maxHeight: .infinity, alignment: .top)
            }
            .padding(8)
        }
        .background(Color(hex: dark ? 0x1C1C1E : 0xF2F2F7))
    }
}

/// Playback, as the web UI's: the highest quality streamed to this device,
/// which the player's panel also sets, whether the next episode plays
/// when one ends, and on the Mac whether each video has a window of its own.
public struct PlaybackSettingsView: View {
    @AppStorage(PlaybackQuality.key) private var quality = PlaybackQuality.auto
    @AppStorage(DeviceSetting.autoplayNext) private var autoplayNext = true
    @AppStorage(DeviceSetting.playerWindows) private var playerWindows = false
    @Environment(\.theme) private var theme

    /// Playback on this device.
    public init() {}

    /// The view's content.
    public var body: some View {
        List {
            Section {
                Picker(MavioStrings.settingsMaxQuality, selection: $quality) {
                    ForEach(PlaybackQuality.allCases, id: \.self) { Text($0.label).tag($0) }
                }
                .tint(theme.colors.textSecondary)
                Toggle(MavioStrings.settingsAutoplayNext, isOn: $autoplayNext)
                #if os(macOS)
                    Toggle(isOn: $playerWindows) {
                        Text(MavioStrings.settingsPlayerWindows)
                        Text(MavioStrings.settingsPlayerWindowsMessage)
                    }
                #endif
            } footer: {
                Text(MavioStrings.settingsAppearanceMessage)
            }
        }
        .tint(theme.colors.accent)
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsPlayback)
        .inlineTitle()
    }
}
