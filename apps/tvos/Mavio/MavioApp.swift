import MavioKit
import MavioUI
import SwiftUI

/// Mavio for Apple TV.
@main
struct MavioApp: App {
    var body: some Scene {
        WindowGroup {
            VStack(spacing: 8) {
                Wordmark()
                Text(verbatim: coreVersion)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
    }
}
