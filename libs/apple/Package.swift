// swift-tools-version: 6.2
// The Swift packages the iPhone and iPad, Apple TV and Mac apps share
// (docs/clients.md §4.2). MavioCore is the shared core's Swift API, which
// scripts/build-core.sh builds into build/ with its XCFramework; Libass is
// libass, which mavio-deps releases and tools/deps fetches into build/.
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
        .package(url: "https://github.com/mavioai/AetherEngine", branch: "v7.39.0-mavio"),
        .package(url: "https://github.com/kean/Nuke", exact: "13.2.0"),
        .package(url: "https://github.com/pointfreeco/swift-snapshot-testing", exact: "1.19.6"),
        .package(url: "https://github.com/apple/swift-protobuf", exact: "1.38.1"),
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
        // The messages of the server's API, which scripts/gen-proto.sh
        // generates.
        .target(
            name: "MavioProto",
            dependencies: [.product(name: "SwiftProtobuf", package: "swift-protobuf")],
            path: "gen/MavioProto"
        ),
        .target(name: "MavioKit", dependencies: ["MavioCore", "MavioProto"]),
        .target(
            name: "MavioPlayer",
            dependencies: [
                // MavioCoreFFI exports libdovi, which AetherEngine calls.
                "Libass", "MavioCoreFFI", .product(name: "AetherEngine", package: "AetherEngine"),
                .product(name: "Nuke", package: "Nuke"),
            ]
        ),
        .target(
            name: "MavioUI",
            dependencies: [
                "MavioKit", .product(name: "MavioDesign", package: "design"),
                .product(name: "NukeUI", package: "Nuke"),
            ]
        ),
        .testTarget(name: "MavioKitTests", dependencies: ["MavioKit"]),
        .testTarget(name: "MavioPlayerTests", dependencies: ["MavioPlayer"]),
        // The snapshot tests (docs/clients.md §6.8), which draw on the iOS
        // simulator: scripts/snapshot.sh.
        .testTarget(
            name: "MavioUITests",
            dependencies: ["MavioUI", .product(name: "SnapshotTesting", package: "swift-snapshot-testing")]
        ),
    ]
)
