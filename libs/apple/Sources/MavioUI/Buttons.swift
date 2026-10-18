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
        /// The main action over artwork: white with black content.
        case artworkPrimary
        /// An icon in a circle of glass over artwork.
        case artworkCircle
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
                .modifier(Fill(kind: kind, color: background, shape: shape))
                .contentShape(shape)
                .opacity(enabled ? (configuration.isPressed ? 0.8 : 1) : 0.6)
        }

        @ViewBuilder private var label: some View {
            switch kind {
            case .circle, .artworkCircle:
                configuration.label.frame(width: 44, height: 44)
            case .primary, .secondary, .artworkPrimary:
                configuration.label
                    .padding(.horizontal, large ? MavioSpacing.s20 : MavioSpacing.s16)
                    .frame(minHeight: large ? 48 : 36)
            }
        }

        private var foreground: Color {
            switch kind {
            case .artworkPrimary: .black
            case .artworkCircle: .white
            case _ where !enabled: theme.colors.textDisabled
            case .primary: theme.colors.onButton
            default: theme.colors.textPrimary
            }
        }

        private var background: Color {
            switch kind {
            case .primary: theme.colors.button
            case .artworkPrimary: .white
            case .artworkCircle: .white.opacity(0.2)
            default: theme.colors.fill
            }
        }

        private var shape: AnyShape {
            switch kind {
            case .circle, .artworkCircle: AnyShape(Circle())
            case .primary, .secondary, .artworkPrimary:
                AnyShape(RoundedRectangle(cornerRadius: large ? 10 : MavioRadius.md))
            }
        }
    }
}

/// A button's fill: glass over artwork, else its color.
private struct Fill: ViewModifier {
    let kind: MavioButtonStyle.Kind
    let color: Color
    let shape: AnyShape

    func body(content: Content) -> some View {
        if kind == .artworkCircle {
            content.glass(in: shape)
        } else {
            content.background(color, in: shape)
        }
    }
}

extension View {
    /// Glass in shape over what is behind: Liquid Glass where the system
    /// has it, else a thin material.
    @ViewBuilder public func glass(in shape: some Shape) -> some View {
        if #available(iOS 26, tvOS 26, macOS 26, *) {
            glassEffect(.regular.interactive(), in: shape)
        } else {
            background(.ultraThinMaterial, in: shape)
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
    /// An icon in a circle of glass over artwork.
    public static var artworkCircle: MavioButtonStyle { MavioButtonStyle(.artworkCircle) }
}
