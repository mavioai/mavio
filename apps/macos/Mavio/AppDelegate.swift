import AppKit
import MavioMacApp

/// Mavio for Mac: an AppKit app. MacApp owns the windows: the main window
/// with the sidebar and its places, or television mode's tabs, and a window
/// of its own for each playback, which the menu bar and the keys control.
@main
@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate, NSMenuItemValidation {
    private var app: MacApp?

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
        let model: AppModel
        do {
            #if DEBUG
                // A debug build is signed anew each time, which the Keychain
                // would ask about at each launch: its secrets are in a file,
                // and the Keychain is never read.
                let secrets = URL.applicationSupportDirectory.appending(path: "Mavio/dev-secrets.json")
                model = try AppModel(tokens: FileTokens(url: secrets))
            #else
                model = try AppModel()
            #endif
        } catch {
            let alert = NSAlert()
            alert.alertStyle = .critical
            alert.messageText = MavioStrings.stateError
            alert.informativeText = error.localizedDescription
            alert.runModal()
            NSApp.terminate(nil)
            return
        }
        let app = MacApp(model: model)
        self.app = app
        NSApp.mainMenu = MainMenu.make(target: self)
        Appearance.apply(tv: app.isTVMode)
        app.showMainWindow()
        model.start()
        NSApp.activate()
    }

    func applicationWillTerminate(_ notification: Notification) {
        app?.model.stop()
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows: Bool) -> Bool {
        if !hasVisibleWindows { app?.showMainWindow() }
        return true
    }

    func application(_ application: NSApplication, open urls: [URL]) {
        for url in urls { app?.open(url) }
    }

    // MARK: The menu bar's actions

    @objc func showMainWindow() { app?.showMainWindow() }

    @objc func openSettings() { app?.openSettings() }

    @objc func showSearch() { app?.showSearch() }

    @objc func goBack() { app?.goBack() }

    @objc func toggleTVMode() { app?.toggleTVMode() }

    func validateMenuItem(_ item: NSMenuItem) -> Bool {
        if item.action == #selector(toggleTVMode) {
            item.title = app?.isTVMode == true ? MavioStrings.menuStandardMode : MavioStrings.menuTvMode
        }
        return app != nil
    }
}
