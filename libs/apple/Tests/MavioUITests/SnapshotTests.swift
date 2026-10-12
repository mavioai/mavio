// The snapshot tests of the iPhone and iPad (docs/clients.md §6.8): the
// screens of libs/design/snapshots/screens.json drawn with MavioUI on the
// iOS simulator and compared with the references in libs/apple/snapshots.
// scripts/snapshot.sh runs them; on the Mac they are skipped.
#if os(iOS)
    import Foundation
    import MavioCore
    import MavioDesign
    import MavioUI
    import SnapshotTesting
    import SwiftUI
    import UIKit
    import XCTest

    /// The fixed data of the screens, as libs/design/snapshots/screens.json
    /// holds it.
    struct Screens: Decodable {
        struct Size: Decodable {
            let width: Double
            let height: Double
            let scale: Double
        }

        struct Server: Decodable {
            let id: String
        }

        struct Library: Decodable {
            let id: String
            let name: String
        }

        struct Item: Decodable {
            let kind: String
            let name: String
            let year: Int32?
            let runtimeMinutes: Double?
            let series: String?
            let season: Int?
            let episode: Int?
        }

        struct Hub: Decodable {
            struct Entry: Decodable {
                let id: String
                let progressMinutes: Double?
                let played: Bool?
            }

            let kind: String
            let library: String?
            let items: [Entry]
        }

        let sizes: [String: Size]
        let server: Server
        let libraries: [Library]
        let items: [String: Item]
        let home: [Hub]

        static func load() throws -> Screens {
            let libs = URL(filePath: #filePath)
                .deletingLastPathComponent().deletingLastPathComponent()
                .deletingLastPathComponent().deletingLastPathComponent()
            let data = try Data(contentsOf: libs.appending(path: "design/snapshots/screens.json"))
            return try JSONDecoder().decode(Screens.self, from: data)
        }

        /// The card of a hub's entry.
        func card(_ entry: Hub.Entry) -> Card {
            let item = items[entry.id]!
            let runtime = item.runtimeMinutes.map { $0 * 60 }
            let subtitle = item.series.flatMap { items[$0] }.map { series in
                "\(series.name) · S\(item.season ?? 0):E\(item.episode ?? 0)"
            }
            return Card(
                serverId: server.id, itemId: entry.id,
                kind: item.kind == "episode" ? .episode : item.kind == "series" ? .series : .movie,
                title: item.name, subtitle: subtitle, year: item.year, poster: nil, thumb: nil,
                progress: entry.progressMinutes.flatMap { p in runtime.map { Float(p * 60 / $0) } },
                played: entry.played ?? false, favorite: false, runtime: runtime, versions: [])
        }
    }

    /// Home of one server, as the app draws its rows.
    struct HomeScreen: View {
        let screens: Screens
        @Environment(\.theme) private var theme

        var body: some View {
            VStack(alignment: .leading, spacing: 0) {
                ForEach(Array(screens.home.enumerated()), id: \.offset) { _, hub in
                    let cards = hub.items.map(screens.card)
                    if hub.kind == "continueWatching" {
                        Shelf(MavioStrings.homeContinueWatching) {
                            ForEach(cards, id: \.self) { ThumbCard($0, width: theme.layout.thumbnail) }
                        }
                    } else {
                        let library = screens.libraries.first { $0.id == hub.library }?.name ?? ""
                        Shelf(MavioStrings.homeLatest(library: library)) {
                            ForEach(cards, id: \.self) { PosterCard($0, width: theme.layout.poster) }
                        }
                    }
                }
                Spacer(minLength: 0)
            }
            .padding(.vertical, MavioSpacing.s8)
        }
    }

    @MainActor
    final class SnapshotTests: XCTestCase {
        func testHome() async throws {
            let screens = try Screens.load()
            for size in ["phone", "tablet"] {
                try await snapshot(HomeScreen(screens: screens), "home-\(size)", size: screens.sizes[size]!, size)
            }
        }

        /// Draws view at size and compares it with the reference name, or
        /// records the reference when MAVIO_SNAPSHOT_RECORD is set.
        private func snapshot(
            _ view: some View, _ name: String, size: Screens.Size, _ form: String,
            file: StaticString = #filePath, line: UInt = #line
        ) async throws {
            let layout: MavioLayout = form == "tablet" ? .tablet : .phone
            let screen =
                view
                .environment(\.theme, Theme(colors: .dark, layout: layout))
                .frame(width: size.width, height: size.height)
                .background(MavioColors.dark.background)
            let traits = UITraitCollection {
                $0.displayScale = size.scale
                $0.userInterfaceStyle = .dark
                $0.horizontalSizeClass = form == "tablet" ? .regular : .compact
                $0.preferredContentSizeCategory = .large
            }
            let image = await withCheckedContinuation { done in
                Snapshotting<AnyView, UIImage>
                    .image(layout: .fixed(width: size.width, height: size.height), traits: traits)
                    .snapshot(AnyView(screen))
                    .run { done.resume(returning: $0) }
            }
            let apple = URL(filePath: "\(file)")
                .deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            let reference = apple.appending(path: "snapshots/\(name).png")
            // Text antialiases a little differently on other versions of the
            // simulator, at the edges of the glyphs.
            let diffing = Diffing<UIImage>.image(precision: 0.99, perceptualPrecision: 0.98, scale: size.scale)
            let record = ProcessInfo.processInfo.environment["MAVIO_SNAPSHOT_RECORD"] != nil
            guard !record, let data = try? Data(contentsOf: reference) else {
                try FileManager.default.createDirectory(
                    at: reference.deletingLastPathComponent(), withIntermediateDirectories: true)
                try diffing.toData(image).write(to: reference)
                if !record { XCTFail("recorded the missing reference \(name).png", file: file, line: line) }
                return
            }
            if let (message, _) = diffing.diffV2(diffing.fromData(data), image) {
                let actual = apple.appending(path: "build/snapshots/\(name).png")
                try? FileManager.default.createDirectory(
                    at: actual.deletingLastPathComponent(), withIntermediateDirectories: true)
                try? diffing.toData(image).write(to: actual)
                XCTFail("\(name).png differs from its reference: \(message)", file: file, line: line)
            }
        }
    }
#endif
