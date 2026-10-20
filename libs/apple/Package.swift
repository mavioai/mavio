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
        .library(name: "MavioAdmin", targets: ["MavioAdmin"]),
        .library(name: "MavioAppKit", targets: ["MavioAppKit"]),
        .library(name: "MavioMacApp", targets: ["MavioMacApp"]),
        .library(name: "MavioCommon", targets: ["MavioCommon"]),
        .library(name: "MavioKit", targets: ["MavioKit"]),
        .library(name: "MavioPlayer", targets: ["MavioPlayer"]),
        .library(name: "MavioScreens", targets: ["MavioScreens"]),
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
        // What every app's screens share apart from their toolkit: the
        // devices' settings, Discover's state, the labels of the server's
        // values, the settings' sections and the cache of images.
        .target(
            name: "MavioCommon",
            dependencies: [
                "MavioKit", .product(name: "MavioDesign", package: "design"), .product(name: "Nuke", package: "Nuke"),
            ]
        ),
        .target(
            name: "MavioUI",
            dependencies: ["MavioCommon", .product(name: "NukeUI", package: "Nuke")],
            // The logos of the sites a title's ratings and links come from.
            resources: [.process("Resources")]
        ),
        // The settings of a server, which the apps show to its users and
        // administrators.
        .target(name: "MavioAdmin", dependencies: ["MavioUI"]),
        // The screens the iPhone and iPad app and the Mac app share, which
        // each composes in its own shell.
        .target(name: "MavioScreens", dependencies: ["MavioAdmin"]),
        // The Mac app's components in AppKit, drawn with the design tokens:
        // artwork, buttons, cards, shelves and grids, pages, navigation,
        // forms, and the focus of television mode.
        .target(
            name: "MavioAppKit",
            dependencies: ["MavioCommon", "MavioPlayer", .product(name: "Nuke", package: "Nuke")]
        ),
        // What the Mac app's pages share: their context, the page they
        // derive from, and the rows and cards of a server's items.
        .target(name: "MavioMac", dependencies: ["MavioAppKit"]),
        // The Mac app's pages in AppKit, by area: an item's details, the
        // libraries, Home, search and downloads; Discover and requests;
        // signing in and the settings of the user and the device; a
        // server's settings; acquiring media; the AI providers,
        // conversations and the plugins; the player.
        .target(name: "MavioMacMedia", dependencies: ["MavioMac"]),
        .target(name: "MavioMacDiscover", dependencies: ["MavioMac"]),
        .target(name: "MavioMacSettings", dependencies: ["MavioMac"]),
        .target(name: "MavioMacServer", dependencies: ["MavioMac"]),
        .target(name: "MavioMacAcquisition", dependencies: ["MavioMac"]),
        .target(name: "MavioMacPlugins", dependencies: ["MavioMac"]),
        // The Mac app's player window: the controls over the picture.
        .target(name: "MavioMacPlayer", dependencies: ["MavioMac"]),
        // The Mac app's shell: the main window's sidebar and places, the
        // user's menu, television mode, and the page of every route and
        // section.
        .target(
            name: "MavioMacApp",
            dependencies: [
                "MavioMacMedia", "MavioMacDiscover", "MavioMacSettings", "MavioMacServer", "MavioMacAcquisition",
                "MavioMacPlugins", "MavioMacPlayer",
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
