import Foundation

/// The logos of the sites a title's ratings and links come from, in
/// Resources/Services.xcassets: Seerr's, under its license
/// (MavioUI/Resources/ServiceLogos-NOTICE.txt).
public enum ServiceLogos {
    /// The bundle holding the logos, which each toolkit loads its image
    /// from by name: "imdb", "tmdb", "tmdb-wide", "tvdb", "rt", "rt-fresh",
    /// "rt-rotten", "rt-aud-fresh", "rt-aud-rotten", "trakt", "simkl" or
    /// "letterboxd".
    public static var bundle: Bundle { .module }
}
