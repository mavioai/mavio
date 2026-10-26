import MavioUI
import SwiftUI

/// What the scrubber is over while it is dragged: the picture there, when
/// the player has one, over the chapter's name and the time, in glass above
/// the thumb.
struct ScrubPreview: View {
    let image: CGImage?
    let time: String
    var chapter: String?

    /// How wide the picture is.
    static let width: CGFloat = 160

    var body: some View {
        VStack(spacing: MavioSpacing.s4) {
            if let image {
                Image(decorative: image, scale: 1)
                    .resizable()
                    .aspectRatio(contentMode: .fill)
                    .frame(width: Self.width, height: Self.width * 9 / 16)
                    .clipShape(.rect(cornerRadius: 8))
            }
            Text(chapter.flatMap { $0.isEmpty ? nil : "\($0) · \(time)" } ?? time)
                .lineLimit(1)
                .frame(maxWidth: Self.width)
                .type(MavioTypography.standard.footnote).fontWeight(.semibold)
                .monospacedDigit()
                .padding(.horizontal, MavioSpacing.s8)
                .frame(height: 24)
        }
        .padding(image == nil ? 0 : 4)
        .foregroundStyle(.white)
        .glass(in: .rect(cornerRadius: 12))
        .environment(\.colorScheme, .dark)
        .allowsHitTesting(false)
    }
}
