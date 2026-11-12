import CoreImage
import CoreImage.CIFilterBuiltins
import Foundation

/// The modules of a QR code: rows of them, true where dark.
public struct QRModules: Equatable, Sendable {
    /// The rows, each of the same count of modules.
    public let dark: [[Bool]]

    /// Modules as they are drawn, such as rclone's in its help.
    public init(dark: [[Bool]]) {
        self.dark = dark
    }

    /// The QR code of text, encoded by CoreImage's `CIQRCodeGenerator` at
    /// level M without its quiet zone, or nil when it cannot be encoded.
    public init?(encoding text: String) {
        let filter = CIFilter.qrCodeGenerator()
        filter.message = Data(text.utf8)
        filter.correctionLevel = "M"
        guard let image = filter.outputImage else { return nil }
        let extent = image.extent.integral
        let width = Int(extent.width), height = Int(extent.height)
        guard width > 0, height > 0 else { return nil }
        // A pixel to a module, as the generator draws it.
        var pixels = [UInt8](repeating: 255, count: width * height)
        CIContext(options: [.useSoftwareRenderer: true]).render(
            image, toBitmap: &pixels, rowBytes: width, bounds: extent, format: .L8, colorSpace: nil)
        let rows = (0..<height).map { y in (0..<width).map { x in pixels[y * width + x] < 128 } }
        // Without the light margin the generator leaves around the code.
        guard let top = rows.firstIndex(where: { $0.contains(true) }),
            let bottom = rows.lastIndex(where: { $0.contains(true) }),
            let left = (0..<width).first(where: { x in rows.contains { $0[x] } }),
            let right = (0..<width).last(where: { x in rows.contains { $0[x] } })
        else { return nil }
        self.dark = rows[top...bottom].map { Array($0[left...right]) }
    }
}
