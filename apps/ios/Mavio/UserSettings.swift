import MavioKit
import MavioUI
import SwiftUI

/// Subtitles and Audio, as the web UI's: the user's languages and when
/// subtitles show, read from the first server signed in to and saved to
/// each of them.
struct SubtitlesAudioView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var saved: PlaybackPreferences?
    @State private var audio = ""
    @State private var subtitles = ""
    @State private var mode = SubtitleMode.default
    @State private var failure: String?
    @State private var saving = false
    @FocusState private var editing: Int?

    /// The modes in the web UI's order.
    private static let modes: [(SubtitleMode, String)] = [
        (.default, MavioStrings.settingsSubtitleModeDefault),
        (.always, MavioStrings.settingsSubtitleModeAlways),
        (.foreign, MavioStrings.settingsSubtitleModeForeign),
        (.smart, MavioStrings.settingsSubtitleModeSmart),
        (.forced, MavioStrings.settingsSubtitleModeForced),
        (.never, MavioStrings.settingsSubtitleModeNone),
    ]

    var body: some View {
        List {
            if saved != nil {
                Section {
                    field(MavioStrings.settingsAudioLanguages, text: $audio).focused($editing, equals: 0)
                    field(MavioStrings.settingsSubtitleLanguages, text: $subtitles).focused($editing, equals: 1)
                } header: {
                    Text(MavioStrings.settingsSubtitlesAudioMessage)
                } footer: {
                    Text(MavioStrings.settingsLanguagesHint)
                }
                Section {
                    Picker(MavioStrings.settingsSubtitleMode, selection: $mode) {
                        ForEach(Self.modes, id: \.0) { Text($0.1).tag($0.0) }
                    }
                    .pickerStyle(.navigationLink)
                }
            } else if failure == nil {
                ProgressView().frame(maxWidth: .infinity)
                    .listRowBackground(Color.clear)
            }
            if let failure {
                Text(failure)
                    .type(theme.type.callout)
                    .foregroundStyle(theme.colors.danger)
            }
        }
        .scrollContentBackground(.hidden)
        .scrollDismissesKeyboard(.interactively)
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsSubtitlesAudio)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            if saving {
                ProgressView()
            } else if saved != nil {
                Button(MavioStrings.actionSave, action: save)
                    .disabled(!changed || Self.languages(audio) == nil || Self.languages(subtitles) == nil)
            }
        }
        // Coming back from the mode's list keeps what was typed.
        .task { if saved == nil { await load() } }
    }

    private func field(_ label: String, text: Binding<String>) -> some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s4) {
            Text(label)
                .type(theme.type.footnote)
                .foregroundStyle(theme.colors.textSecondary)
            TextField(label, text: text, prompt: Text("chi, eng"))
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .foregroundStyle(
                    Self.languages(text.wrappedValue) == nil ? theme.colors.danger : theme.colors.textPrimary)
        }
        .padding(.vertical, 2)
    }

    private var changed: Bool {
        guard let saved else { return false }
        return Self.languages(audio) != saved.audioLanguages
            || Self.languages(subtitles) != saved.subtitleLanguages || mode != saved.subtitleMode
    }

    private func load() async {
        do {
            let prefs = try await model.client.playbackPreferences()
            show(prefs)
        } catch is CancellationError {
        } catch {
            failure = (error as? CoreError)?.reason ?? error.localizedDescription
        }
    }

    private func show(_ prefs: PlaybackPreferences) {
        saved = prefs
        audio = prefs.audioLanguages.joined(separator: ", ")
        subtitles = prefs.subtitleLanguages.joined(separator: ", ")
        mode = prefs.subtitleMode
    }

    private func save() {
        guard var prefs = saved, let a = Self.languages(audio), let s = Self.languages(subtitles) else { return }
        prefs.audioLanguages = a
        prefs.subtitleLanguages = s
        prefs.subtitleMode = mode
        editing = nil
        saving = true
        failure = nil
        Task {
            defer { saving = false }
            do {
                try await model.client.setPlaybackPreferences(preferences: prefs)
                show(prefs)
            } catch {
                failure = (error as? CoreError)?.reason ?? error.localizedDescription
            }
        }
    }

    /// Three-letter codes from a field: lowercase, separated by commas or
    /// spaces; nil when one is not three letters.
    static func languages(_ s: String) -> [String]? {
        let list = s.lowercased()
            .split(whereSeparator: { $0.isWhitespace || ",，、".contains($0) })
            .map(String.init)
        return list.allSatisfy { $0.count == 3 && $0.allSatisfy { ("a"..."z").contains($0) } } ? list : nil
    }
}

/// The users of a standalone server as avatars, to switch to with their
/// PIN, as the web UI's account settings show them.
struct SwitchUserSection: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var users: [SwitchableUser] = []
    @State private var asking: SwitchableUser?
    @State private var busy = false
    @State private var failure: String?

    var body: some View {
        Group {
            // Switching needs someone else to switch to.
            if users.count > 1 {
                Section(MavioStrings.authSwitchUser) {
                    ScrollView(.horizontal) {
                        HStack(alignment: .top, spacing: MavioSpacing.s20) {
                            ForEach(users, id: \.id) { user in
                                Button {
                                    if user.hasPin { asking = user } else { Task { await switchTo(user, pin: nil) } }
                                } label: {
                                    member(user)
                                }
                                .buttonStyle(.plain)
                                .disabled(user.current || busy)
                            }
                        }
                        .padding(.vertical, MavioSpacing.s8)
                    }
                    .scrollIndicators(.hidden)
                    if let failure, asking == nil {
                        Text(failure).type(theme.type.callout).foregroundStyle(theme.colors.danger)
                    }
                }
            }
        }
        .task(id: server.userId) {
            users = (try? await model.client.switchableUsers(serverId: server.id)) ?? []
        }
        .sheet(
            isPresented: Binding(get: { asking != nil }, set: { if !$0 { asking = nil } })
        ) {
            if let asking {
                PinPad(name: asking.name, busy: busy, error: failure) { pin in
                    Task { await switchTo(asking, pin: pin) }
                } cancel: {
                    self.asking = nil
                    failure = nil
                }
                .presentationDetents([.height(560)])
                .presentationBackground(theme.colors.background)
            }
        }
    }

    private func member(_ user: SwitchableUser) -> some View {
        VStack(spacing: 6) {
            Avatar(name: user.name, size: 56)
                .padding(3)
                .overlay {
                    if user.current { Circle().strokeBorder(theme.colors.accent, lineWidth: 2) }
                }
                .overlay(alignment: .bottomTrailing) {
                    if user.hasPin {
                        Icon("lock", fill: true, size: 12)
                            .padding(4)
                            .background(theme.colors.surfaceRaised, in: .circle)
                    }
                }
            Text(user.name)
                .type(theme.type.footnote)
                .lineLimit(1)
        }
        .frame(width: 68)
        .contentShape(.rect)
    }

    private func switchTo(_ user: SwitchableUser, pin: String?) async {
        busy = true
        failure = nil
        defer { busy = false }
        do {
            try await model.switchUser(server.id, to: user.id, pin: pin)
            asking = nil
        } catch {
            failure = (error as? CoreError)?.reason ?? error.localizedDescription
        }
    }
}

/// Asks for a user's four-digit PIN on a phone's keypad, as the web UI's;
/// the fourth digit sends it, and a wrong one clears the dots.
private struct PinPad: View {
    let name: String
    let busy: Bool
    let error: String?
    let enter: (String) -> Void
    let cancel: () -> Void
    @Environment(\.theme) private var theme
    @State private var pin = ""

    var body: some View {
        VStack(spacing: MavioSpacing.s20) {
            Avatar(name: name, size: 56)
            Text(MavioStrings.authEnterPin(name: name))
                .type(theme.type.title3)
                .multilineTextAlignment(.center)
            HStack(spacing: MavioSpacing.s16) {
                ForEach(0..<4, id: \.self) { i in
                    Circle()
                        .strokeBorder(theme.colors.textPrimary, lineWidth: 2)
                        .background(Circle().fill(i < pin.count ? theme.colors.textPrimary : .clear))
                        .frame(width: 12, height: 12)
                }
            }
            .accessibilityElement()
            .accessibilityLabel(MavioStrings.authPin)
            .accessibilityValue("\(pin.count)/4")
            Text(error ?? " ")
                .type(theme.type.footnote)
                .foregroundStyle(theme.colors.danger)
                .opacity(error == nil ? 0 : 1)
            Grid(horizontalSpacing: MavioSpacing.s20, verticalSpacing: MavioSpacing.s12) {
                ForEach([["1", "2", "3"], ["4", "5", "6"], ["7", "8", "9"], ["", "0", "⌫"]], id: \.self) { row in
                    GridRow {
                        ForEach(row, id: \.self) { key($0) }
                    }
                }
            }
        }
        .padding(MavioSpacing.s24)
        .sensoryFeedback(.error, trigger: error) { _, new in new != nil }
        .onChange(of: error) { _, new in if new != nil { pin = "" } }
    }

    @ViewBuilder private func key(_ k: String) -> some View {
        let label: AnyView =
            switch k {
            // Cancel takes the keypad's empty corner, as on a phone's.
            case "": AnyView(Text(MavioStrings.actionCancel).type(theme.type.callout))
            case "⌫": AnyView(Icon("backspace", size: 24).accessibilityLabel(MavioStrings.searchBackspace))
            default: AnyView(Text(k).font(.system(size: 28, weight: .regular, design: .rounded)))
            }
        Button {
            press(k)
        } label: {
            label
                .frame(width: 72, height: 72)
                .background(k == "⌫" ? .clear : theme.colors.fill, in: .circle)
                .contentShape(.circle)
        }
        .buttonStyle(.plain)
        .disabled(busy && k != "")
    }

    private func press(_ k: String) {
        switch k {
        case "": cancel()
        case "⌫": pin = String(pin.dropLast())
        default:
            guard pin.count < 4 else { return }
            pin += k
            if pin.count == 4 { enter(pin) }
        }
    }
}
