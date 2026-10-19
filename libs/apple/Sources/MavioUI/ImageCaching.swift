import Foundation
import Nuke

/// The images kept on disk across launches, so that a screen drawn again
/// shows them at once instead of their placeholders.
public enum ImageCaching {
    /// Makes the shared pipeline keep the images it loads on disk, keyed by
    /// their URLs without the servers' signatures, which change with the
    /// session and every period while the image they name stays the same.
    public static func configure() {
        ImagePipeline.shared = ImagePipeline(
            configuration: .withDataCache(name: "dev.mavio.images", sizeLimit: 500 << 20),
            delegate: UnsignedKeys())
    }

    /// Loads an image into the caches, as a screen about to show it would.
    public static func load(_ url: URL) async {
        _ = try? await ImagePipeline.shared.image(for: url)
    }
}

/// Keys an image by its URL with the signed token cut out of its path:
/// `…/signed/<token>/images/<id>` is kept as `…/images/<id>`.
private final class UnsignedKeys: ImagePipeline.Delegate {
    func cacheKey(for request: ImageRequest, pipeline: ImagePipeline) -> String? {
        guard let url = request.url?.absoluteString, let range = url.range(of: "/signed/") else { return nil }
        guard let end = url[range.upperBound...].firstIndex(of: "/") else { return nil }
        return url.replacingCharacters(in: range.lowerBound..<end, with: "")
    }
}
