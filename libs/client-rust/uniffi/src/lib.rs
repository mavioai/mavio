//! The Swift and Kotlin API of the shared core, exported with UniFFI
//! (`docs/clients.md` §5.3). The `uniffi-bindgen` crate generates their
//! sources from the built library.

uniffi::setup_scaffolding!();

/// The version of the core.
#[uniffi::export]
pub fn core_version() -> String {
    mavio_core::VERSION.to_owned()
}
