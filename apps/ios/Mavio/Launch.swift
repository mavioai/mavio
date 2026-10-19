import MavioKit
import MavioUI
import SwiftUI

/// The launch screen taken over: the mark where the system's launch screen
/// showed it, kept until Home can be drawn whole, its rows and first images
/// at hand, then growing as it fades into Home, as Plex and YouTube open.
struct LaunchView: View {
    /// Whether the app is ready, which starts the mark's leaving.
    let ready: Bool
    /// Called once the mark has gone.
    let done: () -> Void
    @State private var leaving = false

    var body: some View {
        ZStack {
            Color(uiColor: .systemBackground).opacity(leaving ? 0 : 1)
            Image("LaunchMark")
                .scaleEffect(leaving ? 1.6 : 1)
                .opacity(leaving ? 0 : 1)
        }
        .ignoresSafeArea()
        .allowsHitTesting(!leaving)
        .onChange(of: ready, initial: true) { _, ready in
            guard ready, !leaving else { return }
            withAnimation(.easeIn(duration: MavioDuration.slow)) {
                leaving = true
            } completion: {
                done()
            }
        }
    }

    /// Waits until Home can be drawn without its placeholders: its rows, the
    /// hero's backdrop and logo, and the first cards' artwork, or the time
    /// the launch may take, whichever comes first. Signed out, the sign-in
    /// needs nothing.
    static func prepare(_ model: AppModel, within limit: Duration = .seconds(2)) async {
        if case .signedOut = model.signInState { return }
        let clock = ContinuousClock()
        let deadline = clock.now + limit
        while model.home.isEmpty || model.home.contains(where: \.isLoading), clock.now < deadline {
            try? await Task.sleep(for: .milliseconds(30))
        }
        let urls = firstImages(model.home)
        guard !urls.isEmpty else { return }
        // Whichever comes first: every image loaded, or the deadline.
        await withTaskGroup(of: Bool.self) { group in
            for url in urls {
                group.addTask {
                    await ImageCaching.load(url)
                    return false
                }
            }
            group.addTask {
                try? await Task.sleep(until: deadline, clock: clock)
                return true
            }
            var loaded = 0
            for await late in group {
                if late { break }
                loaded += 1
                if loaded == urls.count { break }
            }
            group.cancelAll()
        }
    }

    /// The images Home's first screen shows: the hero's, and the artwork of
    /// the first rows' first cards.
    private static func firstImages(_ rows: [Row]) -> [URL] {
        var urls: [String] = []
        if let hero = HeroEntry.from(rows).first?.card {
            urls += [hero.backdrop?.url, hero.logo?.url].compactMap(\.self)
        }
        for row in rows.prefix(2) {
            guard case let .items(cards) = row.content else { continue }
            for card in cards.prefix(6) {
                urls += [card.poster?.url, card.thumb?.url].compactMap(\.self)
            }
        }
        return urls.compactMap(URL.init(string:))
    }
}

extension Row {
    /// Whether the row waits for its server's first answer.
    fileprivate var isLoading: Bool {
        if case .loading = content { true } else { false }
    }
}
