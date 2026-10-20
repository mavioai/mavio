import AppKit
import MavioKit
import MavioScreens
import MavioUI
import SwiftUI

/// Mavio for Mac: an AppKit app, with SwiftUI where it fits. It owns the
/// windows: the main window with the sidebar and its places, and a window
/// of its own for each playback, which the menu bar and the keys control.
@main
@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate, NSWindowDelegate {
    private var model: AppModel?
    private var failure: String?
    private var window: NSWindow?
    private let router = Router()
    private let shell = Shell()
    /// The windows playing, the frontmost last.
    private var players: [NSWindow] = []
    /// What each window of the player's plays.
    private var slots: [ObjectIdentifier: PlayerSlot] = [:]

    static func main() {
        let app = NSApplication.shared
        let delegate = AppDelegate()
        app.delegate = delegate
        app.setActivationPolicy(.regular)
        withExtendedLifetime(delegate) { app.run() }
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        #if DEBUG
            // A debug build trusts the certificate authority of a development
            // mavio.dev, put in the app's Application Support.
            let ca = URL.applicationSupportDirectory.appending(path: "Mavio/dev-ca.pem")
            if FileManager.default.fileExists(atPath: ca.path(percentEncoded: false)) {
                setenv("MAVIO_EXTRA_CA_CERTS", ca.path(percentEncoded: false), 1)
            }
        #endif
        ImageCaching.configure()
        do {
            model = try AppModel()
        } catch {
            failure = error.localizedDescription
        }
        NSApp.mainMenu = MainMenu.make(target: self)
        Appearance.apply()
        showMainWindow()
        model?.start()
        NSApp.activate()
    }

    func applicationWillTerminate(_ notification: Notification) {
        model?.stop()
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows: Bool) -> Bool {
        if !hasVisibleWindows { showMainWindow() }
        return true
    }

    func application(_ application: NSApplication, open urls: [URL]) {
        for url in urls { shell.open(url) }
        showMainWindow()
    }

    // MARK: Windows

    /// Shows the main window, making it the first time.
    @objc func showMainWindow() {
        if let window {
            window.makeKeyAndOrderFront(nil)
            return
        }
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1280, height: 800),
            styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
            backing: .buffered, defer: false)
        window.title = "Mavio"
        window.contentMinSize = NSSize(width: 1024, height: 680)
        window.titlebarAppearsTransparent = true
        window.isReleasedWhenClosed = false
        window.setFrameAutosaveName("Main")
        window.tabbingMode = .disallowed
        let content: AnyView =
            if let model {
                AnyView(
                    RootView(play: { [weak self] in self?.play($0) })
                        .environment(model)
                        .environment(router)
                        .environment(shell))
            } else {
                AnyView(StatusView(icon: "error", title: MavioStrings.stateError, message: failure))
            }
        window.contentView = NSHostingView(rootView: content)
        if window.frameAutosaveName.isEmpty || !window.setFrameUsingName("Main") { window.center() }
        window.makeKeyAndOrderFront(nil)
        self.window = window
    }

    /// Plays request in the player's window, in place of what plays there,
    /// or, as the setting asks, in a new window for each video so that
    /// several play at once (docs/clients.md §4.4). The first window of the
    /// player's keeps where it was left; the others open beside it.
    func play(_ request: PlayRequest) {
        guard let model else { return }
        let each = UserDefaults.standard.bool(forKey: DeviceSetting.playerWindows)
        if !each, let player = players.last, let slot = slots[ObjectIdentifier(player)] {
            slot.request = request
            player.title = request.title
            player.makeKeyAndOrderFront(nil)
            return
        }
        let player = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1280, height: 720),
            styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
            backing: .buffered, defer: false)
        player.title = request.title
        player.titlebarAppearsTransparent = true
        player.titleVisibility = .hidden
        player.backgroundColor = .black
        player.isReleasedWhenClosed = false
        player.isMovableByWindowBackground = true
        player.tabbingMode = .disallowed
        player.collectionBehavior.insert(.fullScreenPrimary)
        player.delegate = self
        player.contentMinSize = NSSize(width: 480, height: 270)
        let slot = PlayerSlot(request: request)
        player.contentView = NSHostingView(
            rootView: PlayerSlotView(slot: slot) { [weak player] in player?.close() }.environment(model))
        if let previous = players.last {
            player.setFrame(previous.frame, display: false)
            player.setFrameTopLeftPoint(
                player.cascadeTopLeft(from: NSPoint(x: previous.frame.minX, y: previous.frame.maxY)))
        } else if !player.setFrameUsingName("Player") {
            player.center()
        }
        if players.isEmpty { player.setFrameAutosaveName("Player") }
        player.makeKeyAndOrderFront(nil)
        players.append(player)
        slots[ObjectIdentifier(player)] = slot
    }

    func windowWillClose(_ notification: Notification) {
        guard let closing = notification.object as? NSWindow else { return }
        players.removeAll { $0 === closing }
        // The player's view stops its playback as it leaves the window.
        slots.removeValue(forKey: ObjectIdentifier(closing))?.request = nil
        closing.contentView = nil
    }

    @objc func openSettings() {
        showMainWindow()
        shell.place = .settings
    }

    @objc func showSearch() {
        showMainWindow()
        shell.place = .search
    }
}

extension Appearance {
    /// Gives the app the appearance this device keeps.
    @MainActor static func apply() {
        let raw = UserDefaults.standard.string(forKey: DeviceSetting.appearance) ?? ""
        switch Appearance(rawValue: raw) ?? .system {
        case .system: NSApp.appearance = nil
        case .light: NSApp.appearance = NSAppearance(named: .aqua)
        case .dark: NSApp.appearance = NSAppearance(named: .darkAqua)
        }
    }
}
