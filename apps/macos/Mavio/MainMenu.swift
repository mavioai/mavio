import AppKit
import MavioUI

/// The menu bar: the app's menu with the settings, File, Edit for the
/// fields, View with the sidebar, search and full screen, Window and Help.
@MainActor
enum MainMenu {
    static func make(target: AppDelegate) -> NSMenu {
        let bar = NSMenu()
        bar.addItem(submenu(app(target)))
        bar.addItem(submenu(file()))
        bar.addItem(submenu(edit()))
        bar.addItem(submenu(view(target)))
        let window = window(target)
        bar.addItem(submenu(window))
        NSApp.windowsMenu = window
        let help = NSMenu(title: MavioStrings.menuHelp)
        bar.addItem(submenu(help))
        NSApp.helpMenu = help
        return bar
    }

    private static func submenu(_ menu: NSMenu) -> NSMenuItem {
        let item = NSMenuItem(title: menu.title, action: nil, keyEquivalent: "")
        item.submenu = menu
        return item
    }

    private static func app(_ target: AppDelegate) -> NSMenu {
        let menu = NSMenu(title: "Mavio")
        menu.addItem(
            withTitle: MavioStrings.menuAbout, action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)),
            keyEquivalent: "")
        menu.addItem(.separator())
        let settings = menu.addItem(
            withTitle: MavioStrings.navSettings + "…", action: #selector(AppDelegate.openSettings), keyEquivalent: ",")
        settings.target = target
        menu.addItem(.separator())
        let services = NSMenu(title: MavioStrings.menuServices)
        menu.addItem(withTitle: MavioStrings.menuServices, action: nil, keyEquivalent: "").submenu = services
        NSApp.servicesMenu = services
        menu.addItem(.separator())
        menu.addItem(withTitle: MavioStrings.menuHide, action: #selector(NSApplication.hide(_:)), keyEquivalent: "h")
        menu.addItem(
            withTitle: MavioStrings.menuHideOthers, action: #selector(NSApplication.hideOtherApplications(_:)),
            keyEquivalent: "h"
        ).keyEquivalentModifierMask = [.command, .option]
        menu.addItem(
            withTitle: MavioStrings.menuShowAll, action: #selector(NSApplication.unhideAllApplications(_:)),
            keyEquivalent: "")
        menu.addItem(.separator())
        menu.addItem(
            withTitle: MavioStrings.menuQuit, action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        return menu
    }

    private static func file() -> NSMenu {
        let menu = NSMenu(title: MavioStrings.menuFile)
        menu.addItem(
            withTitle: MavioStrings.menuClose, action: #selector(NSWindow.performClose(_:)), keyEquivalent: "w")
        return menu
    }

    private static func edit() -> NSMenu {
        let menu = NSMenu(title: MavioStrings.menuEdit)
        menu.addItem(withTitle: MavioStrings.menuUndo, action: Selector(("undo:")), keyEquivalent: "z")
        menu.addItem(withTitle: MavioStrings.menuRedo, action: Selector(("redo:")), keyEquivalent: "Z")
        menu.addItem(.separator())
        menu.addItem(withTitle: MavioStrings.menuCut, action: #selector(NSText.cut(_:)), keyEquivalent: "x")
        menu.addItem(withTitle: MavioStrings.menuCopy, action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        menu.addItem(withTitle: MavioStrings.menuPaste, action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        menu.addItem(withTitle: MavioStrings.menuSelectAll, action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")
        return menu
    }

    private static func view(_ target: AppDelegate) -> NSMenu {
        let menu = NSMenu(title: MavioStrings.menuView)
        let search = menu.addItem(
            withTitle: MavioStrings.navSearch, action: #selector(AppDelegate.showSearch), keyEquivalent: "f")
        search.target = target
        menu.addItem(.separator())
        menu.addItem(
            withTitle: MavioStrings.menuToggleSidebar, action: #selector(NSSplitViewController.toggleSidebar(_:)),
            keyEquivalent: "s"
        ).keyEquivalentModifierMask = [.command, .control]
        menu.addItem(
            withTitle: MavioStrings.menuFullScreen, action: #selector(NSWindow.toggleFullScreen(_:)),
            keyEquivalent: "f"
        ).keyEquivalentModifierMask = [.command, .control]
        return menu
    }

    private static func window(_ target: AppDelegate) -> NSMenu {
        let menu = NSMenu(title: MavioStrings.menuWindow)
        menu.addItem(
            withTitle: MavioStrings.menuMinimize, action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m"
        )
        menu.addItem(withTitle: MavioStrings.menuZoom, action: #selector(NSWindow.performZoom(_:)), keyEquivalent: "")
        menu.addItem(.separator())
        let main = menu.addItem(withTitle: "Mavio", action: #selector(AppDelegate.showMainWindow), keyEquivalent: "0")
        main.target = target
        menu.addItem(.separator())
        menu.addItem(
            withTitle: MavioStrings.menuBringAllToFront, action: #selector(NSApplication.arrangeInFront(_:)),
            keyEquivalent: "")
        return menu
    }
}
