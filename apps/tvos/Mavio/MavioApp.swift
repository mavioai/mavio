import MavioKit
import MavioUI
import SwiftUI

/// Mavio for Apple TV.
@main
struct MavioApp: App {
    @State private var launch = Launch()

    var body: some Scene {
        WindowGroup {
            if let model = launch.model, let assistant = launch.assistant {
                RootView().environment(model).environment(assistant)
            } else {
                StatusView(icon: "error", title: MavioStrings.stateError, message: launch.failure)
            }
        }
    }
}

/// The app's model and its assistant, made once as the app launches.
@MainActor
@Observable
final class Launch {
    let model: AppModel?
    let assistant: Assistant?
    let failure: String?

    init() {
        #if DEBUG
            // A debug build trusts the certificate authority of a development
            // mavio.dev, put in the app's caches, which a simulator's are
            // reachable from the Mac.
            let ca = URL.cachesDirectory.appending(path: "dev-ca.pem")
            if FileManager.default.fileExists(atPath: ca.path(percentEncoded: false)) {
                setenv("MAVIO_EXTRA_CA_CERTS", ca.path(percentEncoded: false), 1)
            }
        #endif
        ImageCaching.configure()
        do {
            let model = try AppModel(
                directory: URL.cachesDirectory.appending(path: "Mavio", directoryHint: .isDirectory))
            self.model = model
            assistant = Assistant(model: model)
            failure = nil
        } catch {
            model = nil
            assistant = nil
            failure = error.localizedDescription
        }
    }
}
