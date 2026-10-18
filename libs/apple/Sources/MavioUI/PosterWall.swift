import SwiftUI

/// The screen behind sign-in, as the web UI's: a wall of posters drawn from
/// gradients, tilted, its columns drifting slowly up and down. On a phone
/// it fills the top of the screen and fades into the background below.
public struct PosterWall: View {
    @Environment(\.colorScheme) private var scheme
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.theme) private var theme
    @State private var drifted = false

    /// The colors of the posters, top to bottom: warm, cool and dusk, as films are.
    private static let tones: [(UInt32, UInt32)] = [
        (0xFF9F0A, 0x6B2400), (0x2A6F97, 0x0A1A2E), (0x8E5CF7, 0x24104D), (0xF0412F, 0x3A0A0A),
        (0x30B07A, 0x0A2E20), (0xFFD34E, 0x7A3E00), (0x5AC8FA, 0x10325A), (0xFF6482, 0x420D26),
        (0xC7A17A, 0x33241A), (0x64D2FF, 0x1A2A66), (0xE07A5F, 0x2E1A3A),
    ]
    private static let columns = 6
    private static let rows = 9

    /// Creates the wall.
    public init() {}

    /// The view's content.
    public var body: some View {
        GeometryReader { geo in
            // The wall is half again as large as the screen, so that its
            // tilted edges stay outside it.
            let width = geo.size.width * 1.5
            let gap = width * 0.01
            HStack(alignment: .top, spacing: gap) {
                ForEach(0..<Self.columns, id: \.self) { c in
                    VStack(spacing: gap) {
                        ForEach(0..<Self.rows, id: \.self) { r in
                            poster(Self.tones[(c * 4 + r * 7) % Self.tones.count])
                        }
                    }
                    .offset(y: c % 2 == 1 ? -width * 0.06 : 0)
                    .offset(y: drifted ? (c % 2 == 0 ? -1 : 1) * width * 0.12 : 0)
                }
            }
            .frame(width: width)
            .rotationEffect(.degrees(-8))
            .scaleEffect(1.15)
            .opacity(scheme == .dark ? 0.7 : 0.6)
            .frame(width: geo.size.width, height: geo.size.height, alignment: .top)
            .offset(y: -geo.size.width * 0.25)
        }
        .clipped()
        .overlay {
            LinearGradient(
                stops: [
                    .init(color: theme.colors.background.opacity(0), location: 0.25),
                    .init(color: theme.colors.background, location: 0.96),
                ], startPoint: .top, endPoint: .bottom)
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
        .onAppear {
            guard !reduceMotion else { return }
            withAnimation(.easeInOut(duration: 90).repeatForever(autoreverses: true)) { drifted = true }
        }
    }

    private func poster(_ tone: (UInt32, UInt32)) -> some View {
        RoundedRectangle(cornerRadius: 8)
            .fill(
                LinearGradient(
                    colors: [rgb(tone.0), rgb(tone.1)], startPoint: UnitPoint(x: 0.37, y: 0),
                    endPoint: UnitPoint(x: 0.63, y: 1))
            )
            .overlay {
                RoundedRectangle(cornerRadius: 8).fill(
                    RadialGradient(
                        colors: [.white.opacity(0.28), .white.opacity(0)], center: UnitPoint(x: 0.25, y: 0.12),
                        startRadius: 0, endRadius: 90))
            }
            .aspectRatio(2 / 3, contentMode: .fit)
    }
}

/// A color of 0xRRGGBB.
private func rgb(_ hex: UInt32) -> Color {
    Color(
        .sRGB, red: Double(hex >> 16 & 0xFF) / 255, green: Double(hex >> 8 & 0xFF) / 255,
        blue: Double(hex & 0xFF) / 255)
}
