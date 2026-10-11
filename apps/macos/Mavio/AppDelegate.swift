import AppKit
import MavioKit
import MavioUI
import SwiftUI

/// Mavio for Mac: an AppKit app, with SwiftUI where it fits.
@main
@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private var window: NSWindow?

    static func main() {
        let app = NSApplication.shared
        let delegate = AppDelegate()
        app.delegate = delegate
        app.setActivationPolicy(.regular)
        withExtendedLifetime(delegate) { app.run() }
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.mainMenu = mainMenu()
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 960, height: 600),
            styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
            backing: .buffered,
            defer: false
        )
        window.title = "Mavio"
        window.contentView = NSHostingView(rootView: Welcome())
        window.center()
        window.makeKeyAndOrderFront(nil)
        self.window = window
        NSApp.activate()
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }

    private func mainMenu() -> NSMenu {
        let app = NSMenu()
        app.addItem(withTitle: "Quit Mavio", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        let item = NSMenuItem()
        item.submenu = app
        let menu = NSMenu()
        menu.addItem(item)
        return menu
    }
}

/// The wordmark and the core's version.
private struct Welcome: View {
    var body: some View {
        VStack(spacing: 8) {
            Wordmark()
            Text(verbatim: coreVersion)
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}
