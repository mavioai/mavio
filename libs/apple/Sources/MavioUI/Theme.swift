import MavioDesign
import SwiftUI

/// The tokens a view draws with: the theme's colors, the form factor's
/// layout and the type scale, read from the environment.
public struct Theme: Sendable {
    /// The colors.
    public var colors: MavioColors
    /// The sizes of the form factor.
    public var layout: MavioLayout
    /// The type scale.
    public var type: MavioTypography

    /// The theme of a color scheme and form factor.
    public init(colors: MavioColors = .dark, layout: MavioLayout = .phone, type: MavioTypography = .standard) {
        self.colors = colors
        self.layout = layout
        self.type = type
    }
}

extension EnvironmentValues {
    /// The tokens views draw with.
    @Entry public var theme = Theme()
}

extension MavioTypeStyle {
    /// The system font in this style, scaled with Dynamic Type relative to
    /// the text style of a similar size.
    public var font: Font {
        .system(size: size, weight: weight)
    }
}

extension View {
    /// Sets the font and tracking of a type style.
    public func type(_ style: MavioTypeStyle) -> some View {
        font(style.font).tracking(style.tracking)
    }
}

/// An icon of the design system, drawn in the foreground color.
public struct Icon: View {
    private let name: String
    private let size: CGFloat

    /// The icon of a semantic name, as icons.json names it; fill for its
    /// active state.
    public init(_ name: String, fill: Bool = false, size: CGFloat = 20) {
        self.name = fill ? name + "Fill" : name
        self.size = size
    }

    /// The template image.
    public var body: some View {
        Image(name, bundle: MavioStrings.bundle)
            .renderingMode(.template)
            .resizable()
            .scaledToFit()
            .frame(width: size, height: size)
            .accessibilityHidden(true)
    }
}
