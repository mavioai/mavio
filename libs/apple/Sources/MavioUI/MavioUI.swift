/// SwiftUI components each app composes in its own way, drawn with the
/// design tokens.
@_exported import MavioDesign
import SwiftUI

/// The Mavio wordmark in the logo's gradient (libs/design/brand/wordmark.svg).
public struct Wordmark: View {
    private let height: CGFloat

    /// Creates the wordmark, height tall.
    public init(height: CGFloat = 34) { self.height = height }

    /// The view's content.
    public var body: some View {
        WordmarkShape()
            .fill(
                LinearGradient(
                    stops: [
                        .init(color: Color(red: 1, green: 0.827, blue: 0.306), location: 0),
                        .init(color: Color(red: 1, green: 0.604, blue: 0.180), location: 0.45),
                        .init(color: Color(red: 0.941, green: 0.255, blue: 0.184), location: 1),
                    ], startPoint: .topLeading, endPoint: .bottomTrailing),
                style: FillStyle(eoFill: true)
            )
            .frame(width: height * 244 / 64, height: height)
            .accessibilityLabel(Text(verbatim: "Mavio"))
    }
}

/// The wordmark's outline, on its grid of 244 × 64, scaled to the rectangle.
private struct WordmarkShape: Shape {
    /// The outline in absolute moves and lines, as the SVG's path.
    private static let commands =
        "M0 64V0H14L30 30L46 0H60V64H48V24L34 50H26L12 24V64ZM80 20H114V64H80L70 54V30ZM86 32L82 36V48L86 52H102V32Z"
        + "M124 20H137L146 44.7L155 20H168L152 64H140ZM178 20H190V64H178ZM178 2H190V14H178ZM210 20H234L244 30V54L234 "
        + "64H210L200 54V30ZM216 32L212 36V48L216 52H228L232 48V36L228 32Z"

    func path(in rect: CGRect) -> Path {
        var path = Path()
        var point = CGPoint.zero
        let tokens = Self.commands.split(whereSeparator: \.isWhitespace).flatMap { word -> [Substring] in
            // Commands stick to their first number, as "M0" and "64V0H14".
            var parts: [Substring] = []
            var start = word.startIndex
            for i in word.indices where i != start && word[i].isLetter {
                parts.append(word[start..<i])
                start = i
            }
            parts.append(word[start...])
            return parts
        }
        var i = 0
        func number(_ s: Substring) -> CGFloat { CGFloat(Double(s) ?? 0) }
        while i < tokens.count {
            let t = tokens[i]
            let command = t.first!
            let rest = t.dropFirst()
            switch command {
            case "M", "L":
                point = CGPoint(x: number(rest), y: number(tokens[i + 1]))
                if command == "M" { path.move(to: point) } else { path.addLine(to: point) }
                i += 2
            case "H":
                point.x = number(rest)
                path.addLine(to: point)
                i += 1
            case "V":
                point.y = number(rest)
                path.addLine(to: point)
                i += 1
            default:
                path.closeSubpath()
                i += 1
            }
        }
        return path.applying(CGAffineTransform(scaleX: rect.width / 244, y: rect.height / 64))
    }
}
