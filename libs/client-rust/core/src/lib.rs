//! The shared core of the Mavio clients (`docs/clients.md` §5): the servers
//! a client is signed in to, the event stream, the offline store, playback
//! reporting, downloads and the view models of the screens. It draws
//! nothing and decodes nothing; the apps reach it through `mavio-uniffi`.

/// The version of the core, which the apps show and send as their client
/// version.
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn version() {
        assert_eq!(VERSION, "0.0.0", "got = {VERSION}, want = 0.0.0");
    }
}
