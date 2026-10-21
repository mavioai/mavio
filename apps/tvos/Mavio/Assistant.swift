import MavioKit
import MavioUI
import SwiftUI
import UIKit

// MARK: The app around the assistant

/// The player shown, as the assistant controls it: the player screen sets
/// what it does.
@MainActor
final class PlayerControl: AssistantPlayer {
    var assistantServerId = ""
    var playing: () -> Mavio_Assistant_V1_AskPlaying = { .init() }
    var control: (Mavio_Assistant_V1_ControlCommand, Double) -> Bool = { _, _ in false }
    var streams: (Int32?, Int32?) -> Bool = { _, _ in false }
    var close: () -> Void = {}

    var assistantPlaying: Mavio_Assistant_V1_AskPlaying { playing() }

    func assistantControl(_ command: Mavio_Assistant_V1_ControlCommand, value: Double) -> Bool {
        control(command, value)
    }

    func assistantStreams(audio: Int32?, subtitle: Int32?) -> Bool { streams(audio, subtitle) }
}

extension Router: AssistantHost {
    var assistantScreen: AssistantScreen {
        if let p = playing { return AssistantScreen("player", serverId: p.serverId, itemId: p.itemId) }
        if settings { return AssistantScreen("settings") }
        guard let route = paths[tab]?.last else {
            switch tab {
            case .home: return AssistantScreen("home")
            case .libraries: return AssistantScreen("libraries")
            case .discover: return AssistantScreen("discover")
            case .search: return AssistantScreen("search")
            }
        }
        switch route {
        case .item(let serverId, let itemId), .credits(let serverId, let itemId, _):
            return AssistantScreen("item", serverId: serverId, itemId: itemId)
        case .library(let library):
            return AssistantScreen("library", serverId: library.serverId, libraryId: library.id)
        case .person(let serverId, _): return AssistantScreen("person", serverId: serverId)
        case .catalog, .title, .titleCredits, .discoveredPerson, .discoveredCollection:
            return AssistantScreen("discover")
        }
    }

    var assistantPlayer: (any AssistantPlayer)? { player }

    func assistantPlay(_ action: Mavio_Assistant_V1_PlayAction, serverId: String) -> Bool {
        let ids = action.itemIds
        guard !ids.isEmpty else { return false }
        let i = Int(action.startIndex)
        let start: TimeInterval? =
            action.hasStartPosition
            ? TimeInterval(action.startPosition.seconds) + TimeInterval(action.startPosition.nanos) / 1e9 : nil
        play(PlayRequest(serverId: serverId, itemId: ids[ids.indices.contains(i) ? i : 0], title: "", start: start))
        return true
    }

    func assistantNavigate(_ action: Mavio_Assistant_V1_NavigateAction, serverId: String) -> Bool {
        switch action.target {
        case .home:
            leave()
            tab = .home
            paths[.home] = []
        case .back:
            if playing != nil {
                closePlayer()
            } else if settings {
                settings = false
            } else {
                guard paths[tab]?.isEmpty == false else { return false }
                paths[tab]?.removeLast()
            }
        case .item:
            guard !action.id.isEmpty else { return false }
            leave()
            paths[tab, default: []].append(.item(serverId: serverId, itemId: action.id))
        case .library:
            guard let model else { return false }
            leave()
            let id = action.id
            Task { [weak self] in
                guard let list = try? await model.client.libraries(serverId: serverId).libraries,
                    let library = list.first(where: { $0.id == id }), let self
                else { return }
                paths[tab, default: []].append(.library(library))
            }
        case .search:
            leave()
            searchQuery = action.query
            tab = .search
            paths[.search] = []
        case .settings:
            closePlayer()
            settings = true
        default:
            return false
        }
        return true
    }

    /// Closes the player and the settings, for a page of the tabs.
    private func leave() {
        closePlayer()
        settings = false
    }

    private func closePlayer() {
        if let player { player.close() } else { playing = nil }
    }
}

// MARK: The button

/// The assistant button of the navigation bar (docs/tv.md §3): a
/// microphone in a round button, shown while the server's assistant is on.
/// Pressed, it starts a conversation with the microphone Apple TV has, an
/// iPhone or iPad connected to it or a headset's, and pressed again ends
/// it; without one, it opens the system's dictation (docs/assistant.md
/// §9.3).
struct AssistantButton: View {
    @Environment(Assistant.self) private var assistant
    @Environment(Router.self) private var router

    var body: some View {
        if assistant.server != nil {
            Button {
                if assistant.listening || VoiceSession.microphoneAvailable {
                    assistant.tap()
                } else {
                    router.dictating = true
                }
            } label: {
                Icon("microphone", fill: assistant.listening, size: 32).frame(width: 72, height: 72)
            }
            .buttonStyle(RoundButtonStyle())
            .accessibilityLabel(assistant.listening ? MavioStrings.assistantStopListening : MavioStrings.assistantTalk)
        }
    }
}

// MARK: Dictation

/// The system's keyboard, opened at once over the page with no field of
/// its own, where the Siri Remote's microphone dictates; what it returns is
/// asked as one request, shown with the orb and cards as a voice session's.
struct Dictation: UIViewRepresentable {
    @Binding var active: Bool
    let done: (String) -> Void

    func makeUIView(context: Context) -> UITextField {
        let field = UITextField()
        field.alpha = 0
        field.placeholder = MavioStrings.assistantPlaceholder
        field.returnKeyType = .send
        field.delegate = context.coordinator
        return field
    }

    func updateUIView(_ field: UITextField, context: Context) {
        context.coordinator.parent = self
        guard active, !field.isFirstResponder else { return }
        field.text = ""
        DispatchQueue.main.async { field.becomeFirstResponder() }
    }

    func makeCoordinator() -> Coordinator { Coordinator(parent: self) }

    final class Coordinator: NSObject, UITextFieldDelegate {
        var parent: Dictation

        init(parent: Dictation) {
            self.parent = parent
        }

        func textFieldDidEndEditing(_ field: UITextField) {
            let text = (field.text ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
            field.text = ""
            parent.active = false
            if !text.isEmpty { parent.done(text) }
        }
    }
}

// MARK: The orb and cards

/// The assistant over the page or the player (docs/assistant.md §9.1;
/// libs/design/components.md, Assistant), as the web UI's on a television:
/// the orb at the top right, a pill of its state beside it, and the cards
/// below it. Results to choose from and a confirmation take focus as they
/// come; Back dismisses the assistant.
struct AssistantOverlay: View {
    @Environment(Assistant.self) private var assistant
    @Environment(\.theme) private var theme
    @FocusState private var focus: Target?

    private enum Target: Hashable {
        case item(String), cancel, confirm
    }

    /// The cards' width.
    private static let cardWidth: CGFloat = 720

    var body: some View {
        ZStack(alignment: .topTrailing) {
            if assistant.open {
                VStack(alignment: .trailing, spacing: MavioSpacing.s8) {
                    HStack(spacing: MavioSpacing.s16) {
                        pill
                        Orb(assistant: assistant)
                    }
                    cards
                }
                .padding(.top, 24)
                .padding(.trailing, theme.layout.margin - 24)
                .focusSection()
                .onExitCommand { assistant.close() }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topTrailing)
        .ignoresSafeArea()
        .opacity(assistant.leaving ? 0 : 1)
        .animation(assistant.leaving ? .easeOut(duration: MavioDuration.slow) : nil, value: assistant.leaving)
        .onChange(of: assistant.confirmation?.id) { _, id in
            if id != nil { focus = .confirm }
        }
        .onChange(of: assistant.items.map(\.id)) { _, ids in
            if let first = ids.first { focus = .item(first) }
        }
        // The assistant stays while focus is in its cards.
        .onChange(of: focus) { _, focus in
            if focus == nil { assistant.letGo() } else { assistant.hold() }
        }
    }

    @ViewBuilder private var pill: some View {
        if let status = assistant.status {
            Text(status)
                .type(theme.type.callout)
                .foregroundStyle(theme.colors.textPrimary)
                .lineLimit(1)
                .padding(.horizontal, MavioSpacing.s20)
                .padding(.vertical, MavioSpacing.s8)
                .background(CardFill(shape: Capsule()))
                .transition(.opacity)
        }
    }

    private var cards: some View {
        let a = assistant
        return ScrollView {
            VStack(spacing: MavioSpacing.s16) {
                if a.off {
                    Card { Text(MavioStrings.assistantOff).foregroundStyle(theme.colors.textSecondary) }
                }
                if !a.heard.isEmpty || !a.answer.isEmpty {
                    Card { Talk(assistant: a) }
                }
                if !a.activity.isEmpty {
                    Card {
                        VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                            ForEach(a.activity) { ToolRow(activity: $0) }
                        }
                    }
                }
                if !a.items.isEmpty {
                    Card(padding: MavioSpacing.s16) {
                        VStack(spacing: MavioSpacing.s8) {
                            ForEach(Array(a.items.enumerated()), id: \.element.id) { i, item in
                                ResultRow(assistant: a, item: item, number: a.choose ? i + 1 : nil)
                                    .focused($focus, equals: .item(item.id))
                            }
                        }
                    }
                }
                if let c = a.confirmation {
                    Card {
                        VStack(alignment: .leading, spacing: MavioSpacing.s20) {
                            Text(c.text)
                            HStack(spacing: MavioSpacing.s16) {
                                Button(MavioStrings.actionCancel) { a.confirm(false) }
                                    .focused($focus, equals: .cancel)
                                Button(MavioStrings.assistantConfirm) { a.confirm(true) }
                                    .focused($focus, equals: .confirm)
                            }
                            .buttonStyle(TVButtonStyle())
                            .focusSection()
                        }
                    }
                }
                if let e = a.error {
                    Card { Text(e).foregroundStyle(theme.colors.danger) }
                }
            }
            // Room for the focused rows' scale and shadow.
            .padding(MavioSpacing.s24)
            .animation(.spring(duration: 0.4, bounce: 0.3), value: a.items.map(\.id))
        }
        .frame(width: Self.cardWidth + 2 * MavioSpacing.s24)
        .frame(maxHeight: 900)
        .fixedSize(horizontal: false, vertical: true)
    }
}

/// The orb, half as large again as a phone's, at a television's distance.
private struct Orb: View {
    let assistant: Assistant
    @State private var renderer = OrbRenderer()

    private static let scale: CGFloat = 1.5

    var body: some View {
        let side = OrbRenderer.canvasSize * Self.scale
        TimelineView(.animation(paused: !assistant.open)) { timeline in
            let now = timeline.date.timeIntervalSinceReferenceDate
            Canvas { context, size in
                renderer.step(
                    now: now, mode: assistant.mode, level: assistant.level(), interrupts: assistant.interrupts,
                    reduceMotion: false)
                context.withCGContext { g in
                    // The renderer draws as AppKit's views do, with y up.
                    g.translateBy(x: 0, y: size.height)
                    g.scaleBy(x: 1, y: -1)
                    renderer.draw(in: g, side: size.width, now: now, light: false, reduceMotion: false)
                }
            }
            .frame(width: side, height: side)
            .offset(x: renderer.offset * Self.scale)
        }
        .frame(width: OrbRenderer.size * Self.scale, height: OrbRenderer.size * Self.scale)
        .scaleEffect(assistant.leaving ? 0.4 : 1)
        .animation(assistant.leaving ? .easeOut(duration: MavioDuration.slow) : nil, value: assistant.leaving)
        .accessibilityHidden(true)
    }
}

/// A card of the assistant: glass tinted with the card's color.
private struct Card<Content: View>: View {
    var padding: CGFloat = MavioSpacing.s24
    @ViewBuilder let content: Content
    @Environment(\.theme) private var theme

    var body: some View {
        content
            .type(theme.type.body)
            .foregroundStyle(theme.colors.textPrimary)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(padding)
            .background(CardFill(shape: RoundedRectangle(cornerRadius: MavioRadius.xl)))
            .shadow(
                color: MavioElevation.level3.color, radius: MavioElevation.level3.blur / 2,
                y: MavioElevation.level3.y)
    }
}

/// The fill of the cards and the pill: a blur tinted with the cards' color,
/// and a hairline border.
private struct CardFill<S: Shape>: View {
    let shape: S
    @Environment(\.theme) private var theme

    var body: some View {
        shape.fill(theme.colors.surfaceOverlay.opacity(0.72))
            .background(.ultraThinMaterial, in: shape)
            .overlay(shape.stroke(theme.colors.border, lineWidth: 1))
    }
}

/// What the user said, and the answer, its words lit as they are spoken.
private struct Talk: View {
    let assistant: Assistant
    @State private var clock = SpokenClock()
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s12) {
            if !assistant.heard.isEmpty {
                (Text(assistant.heard).foregroundStyle(theme.colors.textSecondary)
                    + Text(!assistant.userSpeaking && assistant.heardAt != nil ? " ›" : "")
                    .foregroundStyle(theme.colors.textTertiary))
                    .fixedSize(horizontal: false, vertical: true)
            }
            if !assistant.answer.isEmpty {
                TimelineView(.animation(paused: !assistant.answerLive)) { _ in
                    Text(lit()).fixedSize(horizontal: false, vertical: true)
                }
            }
        }
    }

    private func lit() -> AttributedString {
        let live = assistant.answerLive
        let played = clock.advance(live ? assistant.speech()?.played ?? 0 : .infinity, answer: assistant.answer)
        var text = AttributedString()
        for s in SpokenText.segments(assistant.answer) {
            var part = AttributedString(s.text)
            part.foregroundColor = !live || s.at <= played + 120 ? theme.colors.textPrimary : theme.colors.textTertiary
            text += part
        }
        return text
    }
}

/// How much of the answer's speech has played, which goes only forward for
/// the same answer.
private final class SpokenClock {
    private var played = 0.0
    private var answer = ""

    func advance(_ to: Double, answer: String) -> Double {
        // A new answer starts again.
        if !answer.hasPrefix(self.answer) || self.answer.isEmpty { played = 0 }
        self.answer = answer
        played = max(played, to)
        return played
    }
}

/// A tool run or running.
private struct ToolRow: View {
    let activity: Assistant.Activity
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(spacing: MavioSpacing.s16) {
            Icon("search", size: 22)
                .foregroundStyle(theme.colors.accent)
                .frame(width: 40, height: 40)
                .background(theme.colors.accent.opacity(0.16), in: .rect(cornerRadius: MavioRadius.sm))
            Text(activity.summary)
                .type(theme.type.callout)
                .foregroundStyle(theme.colors.textSecondary)
                .frame(maxWidth: .infinity, alignment: .leading)
                .fixedSize(horizontal: false, vertical: true)
            Group {
                if activity.failed {
                    Icon("close", size: 24).foregroundStyle(theme.colors.danger)
                } else if activity.done {
                    Icon("check", size: 24).foregroundStyle(theme.colors.success)
                } else {
                    ProgressView()
                }
            }
            .frame(width: 32, height: 32)
        }
    }
}

/// A result, chosen or opened when pressed.
private struct ResultRow: View {
    let assistant: Assistant
    let item: Mavio_Library_V1_Item
    let number: Int?
    @Environment(\.theme) private var theme

    var body: some View {
        Button {
            assistant.choose(item)
        } label: {
            HStack(spacing: MavioSpacing.s16) {
                if let number {
                    Text("\(number)").fontWeight(.semibold).opacity(0.6).frame(width: 28)
                }
                ArtworkView(assistant.poster(item, maxWidth: 120), title: item.name)
                    .frame(width: 52, height: 78)
                    .clipShape(.rect(cornerRadius: 8))
                VStack(alignment: .leading, spacing: 2) {
                    Text(item.name).fontWeight(.medium)
                    if let s = Assistant.subtitle(item) {
                        Text(s).type(theme.type.caption).opacity(0.7)
                    }
                }
                .lineLimit(1)
            }
            .padding(.vertical, MavioSpacing.s8)
        }
        .buttonStyle(RowButtonStyle())
    }
}
