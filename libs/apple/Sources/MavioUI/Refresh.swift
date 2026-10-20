import SwiftUI

private struct PullToRefresh: ViewModifier {
    let action: @MainActor () async -> Void
    @State private var pulls = 0

    func body(content: Content) -> some View {
        #if os(tvOS)
            // A television has nothing to pull: pages read themselves again.
            content
        #else
            content
                .refreshable {
                    pulls += 1
                    await action()
                }
                .sensoryFeedback(.impact(weight: .medium), trigger: pulls)
        #endif
    }
}

extension View {
    /// Reads the page again when it is pulled down, the spinner showing
    /// until action ends, with a tap of the haptics; every page that can be
    /// read again takes it rather than `refreshable`.
    public func pullToRefresh(_ action: @escaping @MainActor () async -> Void) -> some View {
        modifier(PullToRefresh(action: action))
    }
}
