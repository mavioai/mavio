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
        #if os(macOS)
            Form {
                if let title = pageTitle {
                    Section {
                    } header: {
                        VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                            Text(title).type(theme.type.title1).foregroundStyle(theme.colors.textPrimary)
                            if let message {
                                Text(message).type(theme.type.body).foregroundStyle(theme.colors.textSecondary)
                            }
                        }
                    }
                } else if let message {
                    Section {
                    } footer: {
                        Text(message)
                    }
                }
                content
            }
            .formStyle(CardFormStyle())
        #else
            List {
                if let message {
                    Section {
                    } footer: {
                        Text(message)
                    }
                }
                content
            }
        #endif
    }
}

extension View {
    /// A navigation link as a row of settings opening a page: on the
    /// desktop's cards, the whole row with a chevron at its end, as a list
    /// shows it elsewhere.
    public func rowLink() -> some View {
        #if os(macOS)
            buttonStyle(RowLinkStyle())
        #else
            self
        #endif
    }
}

extension EnvironmentValues {
    /// The title a page of settings shows large at its top, as the web UI's
    /// in a wide window, where the window's bar does not show it.
    @Entry public var pageTitle: String?
}

#if os(macOS)
    /// The web UI's groups in a wide window (apps/web's Group and Row): each
    /// section's heading, its rows on a rounded card with lines between,
    /// and its footer, at most 900 points wide in the middle of the page;
    /// a row's label at its start and its switch, choice or value at its
    /// end. The grouped style of the Mac's forms keeps them much narrower.
    struct CardFormStyle: FormStyle {
        @Environment(\.theme) private var theme

        func makeBody(configuration: Configuration) -> some View {
            ScrollView {
                VStack(alignment: .leading, spacing: MavioSpacing.s32) {
                    ForEach(sections: configuration.content) { section in
                        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                            if !section.header.isEmpty {
                                section.header
                                    .type(theme.type.headline)
                                    .textCase(nil)
                                    .padding(.horizontal, MavioSpacing.s4)
                            }
                            if !section.content.isEmpty {
                                card(section.content)
                            }
                            if !section.footer.isEmpty {
                                section.footer
                                    .type(theme.type.footnote)
                                    .foregroundStyle(theme.colors.textSecondary)
                                    .padding(.horizontal, MavioSpacing.s4)
                            }
                        }
                    }
                }
                .frame(maxWidth: 900, alignment: .leading)
                .padding(.horizontal, MavioSpacing.s48)
                .padding(.vertical, MavioSpacing.s32)
                .frame(maxWidth: .infinity)
            }
            // A button reads as a link, as the web UI's; a row opening a
            // page says so with its chevron.
            .buttonStyle(.borderless)
            .toggleStyle(RowToggleStyle())
            .labeledContentStyle(RowLabeledContentStyle())
            .pickerStyle(.menu)
        }

        private func card(_ rows: SubviewsCollection) -> some View {
            VStack(alignment: .leading, spacing: 0) {
                ForEach(rows) { row in
                    row
                        .frame(maxWidth: .infinity, minHeight: 32, alignment: .leading)
                        .padding(.horizontal, MavioSpacing.s16)
                        .padding(.vertical, 10)
                    if row.id != rows.last?.id {
                        Divider().padding(.leading, MavioSpacing.s16)
                    }
                }
            }
            .background(theme.colors.surface, in: .rect(cornerRadius: 14))
        }
    }

    /// A row's label and the line under it: the first of its texts, then
    /// the others smaller and fainter.
    private struct RowLabel<Label: View>: View {
        let label: Label
        @Environment(\.theme) private var theme

        var body: some View {
            VStack(alignment: .leading, spacing: 2) {
                Group(subviews: label) { parts in
                    ForEach(parts) { part in
                        if part.id == parts.first?.id {
                            part.type(theme.type.body)
                        } else {
                            part.type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                        }
                    }
                }
            }
        }
    }

    /// A row opening a page: the whole row, with a chevron at its end.
    private struct RowLinkStyle: ButtonStyle {
        @Environment(\.theme) private var theme

        func makeBody(configuration: Configuration) -> some View {
            HStack(spacing: MavioSpacing.s12) {
                configuration.label.foregroundStyle(theme.colors.textPrimary)
                Spacer(minLength: 0)
                Icon("forward", size: 16).foregroundStyle(theme.colors.textTertiary)
            }
            .contentShape(.rect)
            .opacity(configuration.isPressed ? 0.6 : 1)
        }
    }

    /// A switch at the end of its row.
    private struct RowToggleStyle: ToggleStyle {
        func makeBody(configuration: Configuration) -> some View {
            HStack(spacing: MavioSpacing.s12) {
                RowLabel(label: configuration.label)
                Spacer(minLength: 0)
                Toggle("", isOn: configuration.$isOn).labelsHidden().toggleStyle(.switch)
            }
        }
    }

    /// A value at the end of its row.
    private struct RowLabeledContentStyle: LabeledContentStyle {
        @Environment(\.theme) private var theme

        func makeBody(configuration: Configuration) -> some View {
            HStack(spacing: MavioSpacing.s12) {
                RowLabel(label: configuration.label)
                Spacer(minLength: 0)
                configuration.content
                    .foregroundStyle(theme.colors.textSecondary)
                    .multilineTextAlignment(.trailing)
            }
        }
    }
#endif
