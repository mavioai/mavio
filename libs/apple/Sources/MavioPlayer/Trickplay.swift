import CoreGraphics
import Foundation
import Nuke

/// Thumbnail sheets the server made of a video for seeking: a thumbnail
/// every interval, row by row, columns across and rows down a sheet, count
/// in all; sheet n is at `{url}/{n}.jpg`.
public struct TrickplaySheets: Sendable, Equatable {
    /// A thumbnail's width, in pixels.
    public let width: Int
    /// A thumbnail's height, in pixels.
    public let height: Int
    /// The thumbnails across a sheet.
    public let columns: Int
    /// The thumbnails down a sheet.
    public let rows: Int
    /// The thumbnails in all.
    public let count: Int
    /// The time between thumbnails.
    public let interval: TimeInterval
    /// The URL the sheets are under.
    public let url: String

    /// Sheets of thumbnails width by height, columns by rows a sheet,
    /// count in all, one every interval, under url.
    public init(width: Int, height: Int, columns: Int, rows: Int, count: Int, interval: TimeInterval, url: String) {
        (self.width, self.height, self.columns, self.rows) = (width, height, columns, rows)
        (self.count, self.interval, self.url) = (count, interval, url)
    }

    /// The sheets of a width at least wide, else the widest.
    public static func choose(_ sheets: [TrickplaySheets], wide: Int) -> TrickplaySheets? {
        sheets.filter { $0.width >= wide }.min { $0.width < $1.width } ?? sheets.max { $0.width < $1.width }
    }

    /// The thumbnail of a position, cut from its sheet, which the image
    /// pipeline keeps; nil while the sheet cannot be fetched.
    func thumbnail(at seconds: TimeInterval) async -> CGImage? {
        guard columns > 0, rows > 0, count > 0, interval > 0 else { return nil }
        let index = min(max(Int(seconds / interval), 0), count - 1)
        let perSheet = columns * rows
        guard let sheetURL = URL(string: "\(url)/\(index / perSheet).jpg"),
            let image = try? await ImagePipeline.shared.image(for: sheetURL),
            let sheet = cgImage(image)
        else { return nil }
        // The sheet's own pixels, should it have been scaled.
        let tileWidth = sheet.width / columns
        let tileHeight = min(sheet.height / rows, tileWidth * max(height, 1) / max(width, 1))
        let tile = index % perSheet
        return sheet.cropping(
            to: CGRect(
                x: tile % columns * tileWidth, y: tile / columns * (sheet.height / rows), width: tileWidth,
                height: tileHeight))
    }
}

private func cgImage(_ image: PlatformImage) -> CGImage? {
    #if canImport(UIKit)
        image.cgImage
    #else
        image.cgImage(forProposedRect: nil, context: nil, hints: nil)
    #endif
}
