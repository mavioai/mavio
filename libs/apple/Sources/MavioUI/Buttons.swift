import SwiftUI

/// The buttons of the web UI: the primary filled with `color.button`, the
/// secondary with `color.fill`, and round icon buttons; the accent is
/// left to progress, selection, links and switches.
public struct MavioButtonStyle: ButtonStyle {
    /// The kinds of button.
    public enum Kind: Sendable {
        /// The screen's main action, filled with `color.button`.
        case primary
        /// Another action, filled with `color.fill`.
        case secondary
        /// An icon in a circle filled with `color.fill`.
        case circle
    }

    private let kind: Kind
    private let large: Bool

    /// A button of a kind; large is the 48 pt height of Play on a phone.
    public init(_ kind: Kind, large: Bool = false) {
        self.kind = kind
        self.large = large
    }

    /// The button.
    public func makeBody(configuration: Configuration) -> some View {
        Styled(configuration: configuration, kind: kind, large: large)
    }

    private struct Styled: View {
        let configuration: Configuration
        let kind: Kind
        let large: Bool
        @Environment(\.theme) private var theme
        @Environment(\.isEnabled) private var enabled

        var body: some View {
            label
                .type(theme.type.headline)
                .fontWeight(.semibold)
                .foregroundStyle(foreground)
                .background(background, in: shape)
                .contentShape(shape)
                .opacity(enabled ? (configuration.isPressed ? 0.8 : 1) : 0.6)
        }

        @ViewBuilder private var label: some View {
            switch kind {
            case .circle:
                configuration.label.frame(width: 44, height: 44)
            case .primary, .secondary:
                configuration.label
                    .padding(.horizontal, large ? MavioSpacing.s20 : MavioSpacing.s16)
                    .frame(minHeight: large ? 48 : 36)
            }
        }

        private var foreground: Color {
            guard enabled else { return theme.colors.textDisabled }
            return kind == .primary ? theme.colors.onButton : theme.colors.textPrimary
        }

        private var background: Color {
            kind == .primary ? theme.colors.button : theme.colors.fill
        }

        private var shape: AnyShape {
            switch kind {
            case .circle: AnyShape(Circle())
            case .primary, .secondary: AnyShape(RoundedRectangle(cornerRadius: large ? 10 : MavioRadius.md))
            }
        }
    }
}

extension ButtonStyle where Self == MavioButtonStyle {
    /// The screen's main action, filled with `color.button`.
    public static var primary: MavioButtonStyle { MavioButtonStyle(.primary) }
    /// Another action, filled with `color.fill`.
    public static var secondary: MavioButtonStyle { MavioButtonStyle(.secondary) }
    /// An icon in a circle filled with `color.fill`.
    public static var circle: MavioButtonStyle { MavioButtonStyle(.circle) }
}
