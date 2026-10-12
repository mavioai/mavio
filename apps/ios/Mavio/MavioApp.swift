import MavioKit
import MavioUI
import SwiftUI

/// Mavio for iPhone and iPad.
@main
struct MavioApp: App {
    @UIApplicationDelegateAdaptor private var delegate: AppDelegate

    var body: some Scene {
        WindowGroup {
            if let model = delegate.model {
                RootView().environment(model)
            } else {
                StatusView(icon: "error", title: MavioStrings.stateError, message: delegate.failure)
            }
        }
    }
}

/// Owns the app's model, which a launch in the background for the
/// downloads' events needs too.
final class AppDelegate: NSObject, UIApplicationDelegate {
    let model: AppModel?
    let failure: String?

    override init() {
        do {
            model = try MainActor.assumeIsolated { try AppModel() }
            failure = nil
        } catch {
            model = nil
            failure = error.localizedDescription
        }
        super.init()
    }

    func application(
        _ application: UIApplication, handleEventsForBackgroundURLSession identifier: String,
        completionHandler: @escaping () -> Void
    ) {
        let completion = UncheckedSendable(completionHandler)
        model?.downloads.backgroundEventsHandled { completion.value() }
    }
}

/// Carries a value the compiler cannot prove sendable across a boundary it
/// crosses once.
private struct UncheckedSendable<T>: @unchecked Sendable {
    let value: T
    init(_ value: T) { self.value = value }
}
