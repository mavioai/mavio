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

/// A page of settings: a list of sections on a phone and a television, and
/// on the desktop a form of grouped cards, as the web UI's sections in a
/// wide window, its switches and choices at the end of their rows.
public struct SettingsList<Content: View>: View {
    let message: String?
    let content: Content
    @Environment(\.pageTitle) private var pageTitle
    @Environment(\.theme) private var theme

    /// A page of the sections content holds, and the message that says
    /// what the page is for, as the web UI's under a page's title.
    public init(message: String? = nil, @ViewBuilder content: () -> Content) {
        self.message = message
        self.content = content()
    }

    /// The view's content.
    public var body: some View {
        List {
            if let message {
                Section {
                } footer: {
                    Text(message)
                }
            }
            content
        }
    }
}

extension View {
    /// A navigation link as a row of settings opening a page: on the
    /// desktop's cards, the whole row with a chevron at its end, as a list
    /// shows it elsewhere.
    public func rowLink() -> some View {
        self
    }
}

extension EnvironmentValues {
    /// The title a page of settings shows large at its top, as the web UI's
    /// in a wide window, where the window's bar does not show it.
    @Entry public var pageTitle: String?
}
