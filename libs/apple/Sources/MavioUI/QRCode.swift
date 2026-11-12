import SwiftUI

/// A QR code, drawn dark on light whatever the theme, for a phone to scan,
/// with a quiet zone of white around it.
public struct QRCodeView: View {
    private let modules: QRModules
    private let quiet: CGFloat
    private let radius: CGFloat
    private let label: String

    /// A QR code's modules in a white frame `quiet` wide, rounded by `radius`,
    /// read out as `label`.
    public init(_ modules: QRModules, quiet: CGFloat = 12, radius: CGFloat = 8, label: String = "QR") {
        self.modules = modules
        self.quiet = quiet
        self.radius = radius
        self.label = label
    }

    /// The view's content.
    public var body: some View {
        let dark = modules.dark
        let width = dark.map(\.count).max() ?? 0
        Canvas { context, size in
            let module = min(size.width / CGFloat(max(width, 1)), size.height / CGFloat(max(dark.count, 1)))
            context.fill(Path(CGRect(origin: .zero, size: size)), with: .color(.white))
            var path = Path()
            for (y, row) in dark.enumerated() {
                for (x, on) in row.enumerated() where on {
                    path.addRect(CGRect(x: CGFloat(x) * module, y: CGFloat(y) * module, width: module, height: module))
                }
            }
            context.fill(path, with: .color(.black))
        }
        .aspectRatio(CGFloat(width) / CGFloat(max(dark.count, 1)), contentMode: .fit)
        .padding(quiet)
        .background(.white)
        .clipShape(.rect(cornerRadius: radius))
        .accessibilityElement()
        .accessibilityLabel(Text(verbatim: label))
    }
}
