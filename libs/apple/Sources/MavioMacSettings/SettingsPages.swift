#if os(macOS)
    import AppKit
    import MavioMac

    /// Signing in, the settings beside their own sidebar, and the remote
    /// control of a session.
    @MainActor
    public enum SettingsPages {
        /// Signing in, or adding a server when adding.
        public static func signIn(context: MacContext, adding: Bool, done: @escaping @MainActor () -> Void)
            -> NSViewController
        {
            SignInPage(context: context, adding: adding, done: done)
        }

        /// The settings, on the account's page when account; close returns
        /// to Home. On the desktop they show beside their own sidebar; in
        /// television mode as a list whose sections open in its place, in a
        /// stack that back pops before it closes them (docs/tv.md §6.6).
        public static func settings(
            context: MacContext, account: Bool, close: @escaping @MainActor () -> Void
        ) -> NSViewController {
            guard context.theme.isTV else {
                return SettingsSplitController(context: context, account: account, close: close)
            }
            let stack = NavigationController(root: SettingsListPage(context: context))
            if account { stack.push(AccountPage(context: context), animated: false) }
            return stack
        }

        /// The remote control of a session of server, for a window of its
        /// own: a small window the pointer drives, in the desktop's theme.
        public static func remote(context: MacContext, server: ServerRecord, sessionId: String) -> NSViewController {
            RemotePage(context: context.with(theme: .desktop), server: server, sessionId: sessionId)
        }

        /// The PIN pad asking name's PIN, giving it to submit; cancel closes
        /// it. The caller presents it as a sheet, and tells it it is busy or
        /// that the PIN was wrong.
        public static func pinPad(
            context: MacContext, name: String, submit: @escaping @MainActor (String) -> Void,
            cancel: @escaping @MainActor () -> Void
        ) -> PinPadController {
            PinPadController(theme: context.theme, name: name, submit: submit, cancel: cancel)
        }

        /// The users the user signed in to server can switch to.
        public static func switchableUsers(_ model: AppModel, _ server: ServerRecord) async -> [SwitchableUser] {
            await model.switchableUsers(server)
        }
    }
#endif
