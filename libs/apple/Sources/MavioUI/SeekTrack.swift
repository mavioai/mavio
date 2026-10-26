import SwiftUI

/// The player's seek track, as the web UI's and the Mac's: one rounded
/// track with what has played filled, gaps cut where chapters start, and
/// the knob when it has one. The track is inset by half the knob on each
/// side, so the knob's center reaches its ends.
public struct SeekTrack: View {
    private let fraction: Double
    private let chapters: [Double]
    private let height: CGFloat
    private let knob: CGFloat
    private let showsKnob: Bool

    /// The gap cut where a chapter starts.
    public static let gap: CGFloat = 2

    /// A track filled to fraction (0…1) with gaps at the chapters' fractions;
    /// knob is the knob's diameter, drawn when showsKnob.
    public init(
        fraction: Double, chapters: [Double] = [], height: CGFloat = 4, knob: CGFloat = 12,
        showsKnob: Bool = true
    ) {
        self.fraction = min(max(fraction, 0), 1)
        self.chapters = chapters.filter { $0 > 0 && $0 < 1 }
        self.height = height
        self.knob = knob
        self.showsKnob = showsKnob
    }

    /// The fraction (0…1) at a point across a track of width, knob as given.
    public static func fraction(at x: CGFloat, width: CGFloat, knob: CGFloat = 12) -> Double {
        Double(min(max((x - knob / 2) / max(width - knob, 1), 0), 1))
    }

    /// The track, its fill and the knob.
    public var body: some View {
        Canvas { context, size in
            let usable = max(size.width - knob, 0)
            let track = CGRect(x: knob / 2, y: (size.height - height) / 2, width: usable, height: height)
            var clip = Path(CGRect(origin: .zero, size: size))
            for chapter in chapters {
                clip.addRect(
                    CGRect(x: knob / 2 + chapter * usable - Self.gap / 2, y: 0, width: Self.gap, height: size.height))
            }
            var cut = context
            cut.clip(to: clip, style: FillStyle(eoFill: true))
            cut.fill(Path(roundedRect: track, cornerRadius: height / 2), with: .color(.white.opacity(0.3)))
            var played = track
            played.size.width = fraction * usable
            cut.fill(Path(roundedRect: played, cornerRadius: height / 2), with: .color(.white))
            guard showsKnob else { return }
            let center = CGPoint(x: knob / 2 + fraction * usable, y: size.height / 2)
            context.fill(
                Path(ellipseIn: CGRect(x: center.x - knob / 2, y: center.y - knob / 2, width: knob, height: knob)),
                with: .color(.white))
        }
        .frame(height: max(height, showsKnob ? knob : 0))
    }
}
