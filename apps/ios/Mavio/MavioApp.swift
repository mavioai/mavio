import MavioKit
import MavioScreens
import MavioUI
import SwiftUI

/// Mavio for iPhone and iPad.
@main
struct MavioApp: App {
    @UIApplicationDelegateAdaptor private var delegate: AppDelegate
    @AppStorage(DeviceSetting.appearance) private var appearance = Appearance.system

    var body: some Scene {
        WindowGroup {
            if let model = delegate.model, let assistant = delegate.assistant {
                RootView().environment(model).environment(assistant)
                    .preferredColorScheme(appearance.scheme)
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
    /// The assistant over the app (docs/assistant.md §9.1).
    let assistant: Assistant?
    let failure: String?

    override init() {
        #if DEBUG
            // A debug build trusts the certificate authority of a development
            // mavio.dev, put in the app's Documents.
            let ca = URL.documentsDirectory.appending(path: "dev-ca.pem")
            if FileManager.default.fileExists(atPath: ca.path(percentEncoded: false)) {
                setenv("MAVIO_EXTRA_CA_CERTS", ca.path(percentEncoded: false), 1)
            }
        #endif
        ImageCaching.configure()
        do {
            let model = try MainActor.assumeIsolated { try AppModel() }
            self.model = model
            assistant = MainActor.assumeIsolated { Assistant(model: model) }
            failure = nil
        } catch {
            model = nil
            assistant = nil
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
