// swift-tools-version: 6.2
// The design system's Swift package for the Apple apps: the tokens, icons
// and strings build/build.js generates into gen/swift.
import PackageDescription

let package = Package(
    name: "MavioDesign",
    defaultLocalization: "en",
    platforms: [.iOS(.v18), .tvOS(.v18), .macOS(.v15)],
    products: [.library(name: "MavioDesign", targets: ["MavioDesign"])],
    targets: [
        .target(
            name: "MavioDesign",
            path: "gen/swift",
            resources: [.process("Icons.xcassets"), .process("Localizable.xcstrings")]
        )
    ]
)
