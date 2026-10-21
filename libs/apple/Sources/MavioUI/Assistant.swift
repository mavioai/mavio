#if os(iOS)
    import MavioCommon
    import MavioDesign
    import MavioKit
    import SwiftUI

    /// The assistant over the app on a phone or iPad (docs/assistant.md
    /// §9.1; libs/design/components.md, Assistant), as the web UI's: on a
    /// phone the orb at the bottom's center, above the tab bar where it
    /// shows, and the cards above it, the first nearest; on an iPad the orb
    /// at the top right and the cards below it. A touch outside them
    /// dismisses them.
    public struct AssistantOverlay: View {
        private let assistant: Assistant
        private let lift: CGFloat
        @Environment(\.theme) private var theme
        @Environment(\.horizontalSizeClass) private var sizeClass

        /// The overlay of assistant; lift is how far the tab bar at the
        /// bottom of a phone lifts the orb.
        public init(assistant: Assistant, lift: CGFloat = 0) {
            self.assistant = assistant
            self.lift = lift
        }

        /// The cards' width on an iPad.
        static let cardWidth: CGFloat = 380

        /// The view's content.
        public var body: some View {
            let wide = sizeClass == .regular
            ZStack {
                if assistant.open {
                    Color.clear
                        .contentShape(.rect)
                        .ignoresSafeArea()
                        // The touch only dismisses: it does not reach the page.
                        .gesture(
                            DragGesture(minimumDistance: 0).onChanged { _ in
                                if !assistant.leaving { assistant.close() }
                            }
                        )
                        .accessibilityHidden(true)
                    if wide {
                        HStack(spacing: MavioSpacing.s8) {
                            pill
                            AssistantOrb(assistant: assistant)
                        }
                        .padding(.top, 12)
                        .padding(.trailing, 12)
                        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topTrailing)
                        // Just under the orb's light, which fills only the
                        // middle of its box.
                        cards(wide: true)
                            .frame(width: Self.cardWidth)
                            .padding(.top, 80)
                            .padding(.trailing, MavioSpacing.s16 - MavioSpacing.s4)
                            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topTrailing)
                    } else {
                        AssistantOrb(assistant: assistant)
                            .padding(.bottom, lift + MavioSpacing.s16)
                            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottom)
                        // No status pill on a phone; the cards start just over
                        // the orb's light.
                        cards(wide: false)
                            .padding(.horizontal, MavioSpacing.s4)
                            .padding(.top, 56)
                            .padding(.bottom, lift + 80)
                            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottom)
                    }
                }
            }
            .opacity(assistant.leaving ? 0 : 1)
            .animation(.easeOut(duration: MavioDuration.slow), value: assistant.leaving)
            .accessibilityElement(children: .contain)
            .accessibilityLabel(MavioStrings.assistantTitle)
        }

        @ViewBuilder private var pill: some View {
            if let status = assistant.status {
                Text(status)
                    .type(theme.type.footnote)
                    .foregroundStyle(theme.colors.textPrimary)
                    .lineLimit(1)
                    .padding(.horizontal, MavioSpacing.s12)
                    .padding(.vertical, MavioSpacing.s4)
                    .background(AssistantCardFill(shape: Capsule()))
                    .transition(.opacity)
            }
        }

        /// The cards, scrolling where they are taller than the room for
        /// them: on a phone the first at the bottom.
        private func cards(wide: Bool) -> some View {
            let views = cardViews
            return ScrollView {
                VStack(spacing: MavioSpacing.s8) {
                    ForEach(wide ? views : views.reversed()) { card in
                        card.view.transition(
                            .asymmetric(
                                insertion: .opacity.combined(with: .offset(y: wide ? -8 : 8)), removal: .opacity))
                    }
                }
                .padding(MavioSpacing.s4)
                .animation(.spring(duration: 0.4, bounce: 0.3), value: views.map(\.id))
            }
            .scrollBounceBehavior(.basedOnSize)
            .defaultScrollAnchor(wide ? .top : .bottom)
            .fixedSize(horizontal: false, vertical: true)
            .simultaneousGesture(
                DragGesture(minimumDistance: 0)
                    .onChanged { _ in assistant.hold() }
                    .onEnded { _ in assistant.letGo() })
        }

        private struct CardView: Identifiable {
            let id: String
            let view: AnyView
        }

        private var cardViews: [CardView] {
            let a = assistant
            var views: [CardView] = []
            if a.off {
                views.append(
                    CardView(
                        id: "off",
                        view: AnyView(
                            AssistantCard {
                                Text(MavioStrings.assistantOff).foregroundStyle(theme.colors.textSecondary)
                            })))
            }
            if !a.heard.isEmpty || !a.answer.isEmpty {
                views.append(CardView(id: "talk", view: AnyView(AssistantCard { TalkView(assistant: a) })))
            }
            if !a.activity.isEmpty {
                views.append(
                    CardView(
                        id: "tools",
                        view: AnyView(
                            AssistantCard(vertical: 12, horizontal: 14) {
                                VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                                    ForEach(a.activity) { ToolRow(activity: $0) }
                                }
                            })))
            }
            if !a.items.isEmpty {
                views.append(
                    CardView(
                        id: "items",
                        view: AnyView(
                            AssistantCard(vertical: 10, horizontal: 10) {
                                VStack(spacing: 2) {
                                    ForEach(Array(a.items.enumerated()), id: \.element.id) { i, item in
                                        ResultRow(assistant: a, item: item, number: a.choose ? i + 1 : nil)
                                    }
                                }
                            })))
            }
            if let c = a.confirmation {
                views.append(
                    CardView(
                        id: "confirm" + c.id,
                        view: AnyView(
                            AssistantCard {
                                VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                                    Text(c.text)
                                    HStack(spacing: MavioSpacing.s8) {
                                        Button(MavioStrings.actionCancel) { a.confirm(false) }
                                            .buttonStyle(ConfirmButtonStyle(primary: false))
                                        Button(MavioStrings.assistantConfirm) { a.confirm(true) }
                                            .buttonStyle(ConfirmButtonStyle(primary: true))
                                    }
                                }
                            })))
            }
            if let e = a.error {
                views.append(
                    CardView(
                        id: "error",
                        view: AnyView(
                            AssistantCard {
                                Text(e).foregroundStyle(theme.colors.danger).textSelection(.enabled)
                            })))
            }
            return views
        }
    }

    /// The orb, which pressed starts a voice session and pressed again ends
    /// it, or held talks while held.
    struct AssistantOrb: View {
        let assistant: Assistant
        @State private var renderer = OrbRenderer()
        @State private var pressed = false
        @Environment(\.colorScheme) private var scheme
        @Environment(\.accessibilityReduceMotion) private var reduceMotion

        var body: some View {
            let light = scheme == .light
            let side = OrbRenderer.canvasSize
            TimelineView(.animation(paused: !assistant.open)) { timeline in
                let now = timeline.date.timeIntervalSinceReferenceDate
                Canvas { context, size in
                    renderer.step(
                        now: now, mode: assistant.mode, level: assistant.level(), interrupts: assistant.interrupts,
                        reduceMotion: reduceMotion)
                    context.withCGContext { g in
                        // The renderer draws as AppKit's views do, with y up.
                        g.translateBy(x: 0, y: size.height)
                        g.scaleBy(x: 1, y: -1)
                        renderer.draw(in: g, side: size.width, now: now, light: light, reduceMotion: reduceMotion)
                    }
                }
                .frame(width: side, height: side)
                .offset(x: renderer.offset)
            }
            // The light page's shadows of the web UI's orb.
            .shadow(
                color: light ? Color(.sRGB, red: 20 / 255, green: 12 / 255, blue: 80 / 255, opacity: 0.22) : .clear,
                radius: 1.5, y: 2
            )
            .shadow(
                color: light ? Color(.sRGB, red: 90 / 255, green: 70 / 255, blue: 245 / 255, opacity: 0.42) : .clear,
                radius: 9, y: 10
            )
            .frame(width: OrbRenderer.size, height: OrbRenderer.size)
            .contentShape(.circle)
            .scaleEffect(assistant.leaving ? 0.4 : 1)
            .animation(.easeOut(duration: MavioDuration.slow), value: assistant.leaving)
            .gesture(
                DragGesture(minimumDistance: 0)
                    .onChanged { _ in
                        guard !pressed else { return }
                        pressed = true
                        assistant.press()
                    }
                    .onEnded { _ in
                        pressed = false
                        assistant.release()
                    }
            )
            .accessibilityElement()
            .accessibilityAddTraits(.isButton)
            .accessibilityLabel(assistant.listening ? MavioStrings.assistantStopListening : MavioStrings.assistantTalk)
            .accessibilityAction { assistant.tap() }
        }
    }

    /// A card of the assistant: glass tinted with the card's color, as the
    /// web UI's.
    struct AssistantCard<Content: View>: View {
        let vertical: CGFloat
        let horizontal: CGFloat
        @ViewBuilder let content: Content
        @Environment(\.theme) private var theme

        init(vertical: CGFloat = 14, horizontal: CGFloat = 16, @ViewBuilder content: () -> Content) {
            self.vertical = vertical
            self.horizontal = horizontal
            self.content = content()
        }

        var body: some View {
            content
                .type(theme.type.body)
                .foregroundStyle(theme.colors.textPrimary)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.vertical, vertical)
                .padding(.horizontal, horizontal)
                .background(AssistantCardFill(shape: RoundedRectangle(cornerRadius: MavioRadius.xl)))
                .shadow(
                    color: MavioElevation.level3.color, radius: MavioElevation.level3.blur / 2,
                    y: MavioElevation.level3.y)
        }
    }

    /// The fill of the cards and the pill: a blur tinted with the cards'
    /// color, and a hairline border.
    struct AssistantCardFill<S: Shape>: View {
        let shape: S
        @Environment(\.theme) private var theme
        @Environment(\.colorScheme) private var scheme

        var body: some View {
            shape.fill(scheme == .light ? Color.white.opacity(0.74) : theme.colors.surfaceOverlay.opacity(0.72))
                .background(.ultraThinMaterial, in: shape)
                .overlay(shape.stroke(theme.colors.border, lineWidth: 1))
        }
    }

    /// What the user said, and the answer, its words lit as they are
    /// spoken.
    private struct TalkView: View {
        let assistant: Assistant
        @State private var clock = SpokenClock()
        @Environment(\.theme) private var theme

        var body: some View {
            VStack(alignment: .leading, spacing: 10) {
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
                part.foregroundColor =
                    !live || s.at <= played + 120 ? theme.colors.textPrimary : theme.colors.textTertiary
                text += part
            }
            return text
        }
    }

    /// How much of the answer's speech has played, which goes only
    /// forward for the same answer.
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
            HStack(spacing: 10) {
                Icon("search", size: 14)
                    .foregroundStyle(theme.colors.accent)
                    .frame(width: 26, height: 26)
                    .background(theme.colors.accent.opacity(0.16), in: .rect(cornerRadius: MavioRadius.sm))
                Text(activity.summary)
                    .type(theme.type.footnote)
                    .foregroundStyle(theme.colors.textSecondary)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .fixedSize(horizontal: false, vertical: true)
                Group {
                    if activity.failed {
                        Icon("close", size: 16).foregroundStyle(theme.colors.danger)
                    } else if activity.done {
                        Icon("check", size: 16).foregroundStyle(theme.colors.success)
                    } else {
                        ProgressView().controlSize(.mini)
                    }
                }
                .frame(width: 16, height: 16)
            }
        }
    }

    /// A result, chosen or opened when tapped.
    private struct ResultRow: View {
        let assistant: Assistant
        let item: Mavio_Library_V1_Item
        let number: Int?
        @Environment(\.theme) private var theme

        var body: some View {
            Button {
                assistant.choose(item)
            } label: {
                HStack(spacing: 10) {
                    if let number {
                        Text("\(number)")
                            .type(theme.type.footnote).fontWeight(.semibold)
                            .foregroundStyle(theme.colors.textTertiary)
                            .frame(width: 16)
                    }
                    ArtworkView(assistant.poster(item, maxWidth: 80), title: item.name)
                        .frame(width: 34, height: 51)
                        .clipShape(.rect(cornerRadius: 5))
                    VStack(alignment: .leading, spacing: 1) {
                        Text(item.name).type(theme.type.callout).fontWeight(.medium)
                            .foregroundStyle(theme.colors.textPrimary)
                        if let s = Assistant.subtitle(item) {
                            Text(s).type(theme.type.caption).foregroundStyle(theme.colors.textSecondary)
                        }
                    }
                    .lineLimit(1)
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
                .padding(6)
                .contentShape(.rect(cornerRadius: MavioRadius.md))
            }
            .buttonStyle(RowButtonStyle())
        }
    }

    private struct RowButtonStyle: ButtonStyle {
        @Environment(\.theme) private var theme

        func makeBody(configuration: Configuration) -> some View {
            configuration.label.background(
                configuration.isPressed ? theme.colors.fill : .clear, in: .rect(cornerRadius: MavioRadius.md))
        }
    }

    /// The confirmation's buttons, as the web UI's: Cancel on the fill,
    /// Confirm on the accent.
    private struct ConfirmButtonStyle: ButtonStyle {
        let primary: Bool
        @Environment(\.theme) private var theme

        func makeBody(configuration: Configuration) -> some View {
            configuration.label
                .type(theme.type.callout)
                .foregroundStyle(primary ? theme.colors.onAccent : theme.colors.textPrimary)
                .frame(maxWidth: .infinity, minHeight: 36)
                .background(
                    primary
                        ? theme.colors.accent : configuration.isPressed ? theme.colors.fillSelected : theme.colors.fill,
                    in: .rect(cornerRadius: MavioRadius.md)
                )
                .opacity(primary && configuration.isPressed ? 0.9 : 1)
                .contentShape(.rect(cornerRadius: MavioRadius.md))
        }
    }

    /// The assistant button, which tapped starts a voice session and tapped
    /// again ends it, or held talks while held; hidden while no server's
    /// assistant is on.
    public struct AssistantButton: View {
        private let assistant: Assistant
        private let size: CGFloat
        @State private var pressed = false

        /// The button of assistant, its icon size points.
        public init(assistant: Assistant, size: CGFloat = 22) {
            self.assistant = assistant
            self.size = size
        }

        /// The view's content.
        public var body: some View {
            if assistant.server != nil {
                Icon("microphone", fill: assistant.listening, size: size)
                    .frame(width: 44, height: 44)
                    .contentShape(.rect)
                    .opacity(pressed ? 0.6 : 1)
                    .gesture(
                        DragGesture(minimumDistance: 0)
                            .onChanged { _ in
                                guard !pressed else { return }
                                pressed = true
                                assistant.press()
                            }
                            .onEnded { _ in
                                pressed = false
                                assistant.release()
                            }
                    )
                    .accessibilityElement()
                    .accessibilityAddTraits(.isButton)
                    .accessibilityLabel(
                        assistant.listening ? MavioStrings.assistantStopListening : MavioStrings.assistantTalk
                    )
                    .accessibilityAction { assistant.tap() }
            }
        }
    }
#endif
