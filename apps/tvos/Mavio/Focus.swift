import MavioDesign
import MavioUI
import SwiftUI

/// The motion of focus: `duration.fast` with `easing.standard`.
let focusAnimation = Animation.timingCurve(0.2, 0, 0, 1, duration: MavioDuration.fast)

/// A card that grows with focus: scaled by `layout.focusScale`, its artwork
/// ringed in `color.accent` 1.5 pt apart and over `color.tvFocusShadow`
/// (docs/tv.md §4).
struct CardButtonStyle: ButtonStyle {
    /// The artwork's aspect, which the ring follows.
    var wide = false

    func makeBody(configuration: Configuration) -> some View {
        CardFocus(label: configuration.label, wide: wide, pressed: configuration.isPressed)
    }
}

private struct CardFocus<Label: View>: View {
    let label: Label
    let wide: Bool
    let pressed: Bool
    @Environment(\.isFocused) private var focused
    @Environment(\.theme) private var theme

    var body: some View {
        label
            .environment(\.cardFocused, focused)
            .scaleEffect(focused ? (pressed ? 1 : theme.layout.focusScale) : 1)
            .animation(focusAnimation, value: focused)
            .animation(focusAnimation, value: pressed)
    }
}

extension EnvironmentValues {
    /// Whether the card the view lies in has focus.
    @Entry var cardFocused = false
}

/// The ring and shadow of a focused card's artwork.
struct FocusRing: ViewModifier {
    @Environment(\.cardFocused) private var focused
    @Environment(\.theme) private var theme

    func body(content: Content) -> some View {
        content
            .clipShape(.rect(cornerRadius: MavioRadius.md))
            .shadow(color: focused ? theme.colors.tvFocusShadow : .clear, radius: 16, y: 12)
            .overlay {
                if focused {
                    RoundedRectangle(cornerRadius: MavioRadius.md + 1.5 + 1.5)
                        .strokeBorder(theme.colors.accent, lineWidth: 3)
                        .padding(-4.5)
                }
            }
    }
}

extension View {
    /// Rings the artwork of a focused card.
    func focusRing() -> some View {
        modifier(FocusRing())
    }
}

/// A button of glass that turns white with black content with focus, 64 pt
/// high (§4).
struct TVButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        ButtonFocus(label: configuration.label, pressed: configuration.isPressed, round: false)
    }
}

/// A round button, as the menu button, white with focus.
struct RoundButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        ButtonFocus(label: configuration.label, pressed: configuration.isPressed, round: true)
    }
}

private struct ButtonFocus<Label: View>: View {
    let label: Label
    let pressed: Bool
    let round: Bool
    @Environment(\.isFocused) private var focused
    @Environment(\.theme) private var theme

    var body: some View {
        label
            .type(theme.type.headline)
            .foregroundStyle(focused ? .black : .white)
            .padding(.horizontal, round ? 0 : MavioSpacing.s32)
            .frame(minWidth: round ? nil : 64, minHeight: round ? nil : 64)
            .background {
                if focused {
                    Capsule().fill(.white).shadow(color: theme.colors.tvFocusShadow, radius: 16, y: 12)
                } else {
                    Capsule().fill(.white.opacity(0.16)).background(.ultraThinMaterial, in: .capsule)
                }
            }
            .scaleEffect(focused && !pressed ? theme.layout.focusScale : 1)
            .animation(focusAnimation, value: focused)
    }
}

/// A tab of the navigation bar: filled with `color.fillSelected` while
/// current, white with focus.
struct TabButtonStyle: ButtonStyle {
    let selected: Bool

    func makeBody(configuration: Configuration) -> some View {
        TabFocus(label: configuration.label, selected: selected)
    }
}

private struct TabFocus<Label: View>: View {
    let label: Label
    let selected: Bool
    @Environment(\.isFocused) private var focused
    @Environment(\.theme) private var theme

    var body: some View {
        label
            .type(theme.type.headline)
            .foregroundStyle(focused ? .black : theme.colors.textPrimary)
            .padding(.horizontal, MavioSpacing.s24)
            .frame(height: 56)
            .background {
                if focused {
                    Capsule().fill(.white)
                } else if selected {
                    Capsule().fill(theme.colors.fillSelected)
                }
            }
            .animation(focusAnimation, value: focused)
    }
}

/// A row of a list, as the settings' and the sign-in's: white with black
/// content and a shadow with focus, its corners rounded 14 pt.
struct RowButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        RowFocus(label: configuration.label)
    }
}

private struct RowFocus<Label: View>: View {
    let label: Label
    @Environment(\.isFocused) private var focused
    @Environment(\.theme) private var theme

    var body: some View {
        label
            .type(theme.type.body)
            .foregroundStyle(focused ? .black : theme.colors.textPrimary)
            .padding(.horizontal, MavioSpacing.s24)
            .frame(maxWidth: .infinity, minHeight: 72, alignment: .leading)
            .background {
                RoundedRectangle(cornerRadius: 14)
                    .fill(focused ? .white : theme.colors.fill)
                    .shadow(color: focused ? theme.colors.tvFocusShadow : .clear, radius: 16, y: 12)
            }
            .scaleEffect(focused ? 1.02 : 1)
            .animation(focusAnimation, value: focused)
    }
}

/// A person of the cast or crew: white with black content, its corners
/// rounded, over a shadow with focus (§4).
struct PersonButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        PersonFocus(label: configuration.label, pressed: configuration.isPressed)
    }
}

private struct PersonFocus<Label: View>: View {
    let label: Label
    let pressed: Bool
    @Environment(\.isFocused) private var focused
    @Environment(\.theme) private var theme

    var body: some View {
        label
            .environment(\.cardFocused, focused)
            .padding(MavioSpacing.s16)
            .background {
                RoundedRectangle(cornerRadius: MavioRadius.lg)
                    .fill(focused ? .white : .clear)
                    .shadow(color: focused ? theme.colors.tvFocusShadow : .clear, radius: 16, y: 12)
            }
            .scaleEffect(focused && !pressed ? theme.layout.focusScale : 1)
            .animation(focusAnimation, value: focused)
    }
}

/// A small pill, as a genre, keyword or fact opening the titles alike: of
/// `color.fill`, white with black content with focus.
struct ChipButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        ChipFocus(label: configuration.label)
    }
}

private struct ChipFocus<Label: View>: View {
    let label: Label
    @Environment(\.isFocused) private var focused
    @Environment(\.theme) private var theme

    var body: some View {
        label
            .type(theme.type.callout)
            .foregroundStyle(focused ? .black : theme.colors.textPrimary)
            .padding(.horizontal, MavioSpacing.s16)
            .padding(.vertical, MavioSpacing.s8)
            .background {
                Capsule().fill(focused ? .white : theme.colors.fill)
                    .shadow(color: focused ? theme.colors.tvFocusShadow : .clear, radius: 16, y: 12)
            }
            .scaleEffect(focused ? theme.layout.focusScale : 1)
            .animation(focusAnimation, value: focused)
    }
}
