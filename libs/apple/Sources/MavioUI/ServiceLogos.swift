import SwiftUI

extension Image {
    /// The logo of a site a title's ratings and links come from, by its
    /// name in Services.xcassets: "imdb", "tmdb", "tmdb-wide", "tvdb", "rt",
    /// "rt-fresh", "rt-rotten", "rt-aud-fresh", "rt-aud-rotten", "trakt",
    /// "simkl" or "letterboxd".
    public static func service(_ name: String) -> Image {
        Image(name, bundle: ServiceLogos.bundle)
    }
}
