/// SwiftUI components each app composes in its own way, drawn with the
/// design tokens.
@_exported import MavioDesign
import SwiftUI

/// The Mavio wordmark.
public struct Wordmark: View {
    /// Creates the wordmark.
    public init() {}

    /// The wordmark's text in the accent color.
    public var body: some View {
        Text(verbatim: "Mavio")
            .font(.largeTitle.bold())
            .foregroundStyle(MavioColors.dark.accent)
    }
}
