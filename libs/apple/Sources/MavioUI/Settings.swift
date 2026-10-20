import MavioDesign
import SwiftUI

/// A section's row: its icon on a colored tile, and its title.
public struct EntryLabel: View {
    let title: String
    let icon: String
    let color: Color
    @Environment(\.theme) private var theme

    /// A row of a title, with an icon on a tile of a color.
    public init(_ title: String, icon: String, color: Color) {
        (self.title, self.icon, self.color) = (title, icon, color)
    }

    /// The view's content.
    public var body: some View {
        Label {
            Text(title)
        } icon: {
            Icon(icon, fill: true, size: 17)
                .foregroundStyle(.white)
                .frame(width: 28, height: 28)
                .background(color, in: .rect(cornerRadius: 7))
        }
    }
}

/// A person's initial in a circle.
public struct Avatar: View {
    let name: String
    let size: CGFloat

    /// A name's initial in a circle of a size.
    public init(name: String, size: CGFloat) { (self.name, self.size) = (name, size) }
    @Environment(\.theme) private var theme

    /// The view's content.
    public var body: some View {
        Text(name.first.map { String($0).uppercased() } ?? "")
            .font(.system(size: size * 0.42, weight: .semibold))
            .foregroundStyle(theme.colors.textPrimary)
            .frame(width: size, height: size)
            .background(theme.colors.fill, in: .circle)
    }
}

extension Color {
    /// A color of its sRGB hex value, as the web UI writes the tiles'.
    public init(hex: UInt32) {
        self.init(
            .sRGB, red: Double(hex >> 16 & 0xFF) / 255, green: Double(hex >> 8 & 0xFF) / 255,
            blue: Double(hex & 0xFF) / 255)
    }
}
