// swift-tools-version: 6.2
// The Swift packages the iPhone and iPad, Apple TV and Mac apps share
// (docs/clients.md §4.2). MavioCore is the shared core's Swift API, which
// scripts/build-core.sh builds into build/ with its XCFramework; Libass is
// libass, which scripts/build-ass.sh builds into build/.
import PackageDescription

let package = Package(
    name: "MavioApple",
    defaultLocalization: "en",
    platforms: [.iOS(.v18), .tvOS(.v18), .macOS(.v15)],
    products: [
        .library(name: "MavioKit", targets: ["MavioKit"]),
        .library(name: "MavioPlayer", targets: ["MavioPlayer"]),
        .library(name: "MavioUI", targets: ["MavioUI"]),
    ],
    dependencies: [
        .package(path: "../design"),
        .package(url: "https://github.com/superuser404notfound/AetherEngine", exact: "7.34.0"),
    ],
    targets: [
        .binaryTarget(name: "MavioCoreFFI", path: "build/MavioCoreFFI.xcframework"),
        .binaryTarget(name: "Libass", path: "build/Libass.xcframework"),
        .target(
            name: "MavioCore",
            dependencies: ["MavioCoreFFI"],
            path: "build/MavioCore",
            // UniFFI generates Swift 5 code.
            swiftSettings: [.swiftLanguageMode(.v5)]
        ),
        .target(name: "MavioKit", dependencies: ["MavioCore"]),
        .target(
            name: "MavioPlayer",
            dependencies: ["Libass", .product(name: "AetherEngine", package: "AetherEngine")]
        ),
        .target(name: "MavioUI", dependencies: ["MavioKit", .product(name: "MavioDesign", package: "design")]),
        .testTarget(name: "MavioKitTests", dependencies: ["MavioKit"]),
        .testTarget(name: "MavioPlayerTests", dependencies: ["MavioPlayer"]),
    ]
)
