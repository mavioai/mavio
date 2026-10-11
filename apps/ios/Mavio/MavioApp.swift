import MavioKit
import MavioUI
import SwiftUI

/// Mavio for iPhone and iPad.
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
