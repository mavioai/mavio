import MavioKit
import Testing

@Test func reportsCoreVersion() {
    // A release's, else 0.0.0 and the commit it was built at, else dev.
    #expect(MavioKit.coreVersion.wholeMatch(of: /\d+\.\d+\.\d+(-[0-9a-f]{12})?|dev/) != nil)
}
