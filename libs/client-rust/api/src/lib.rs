//! Messages and Connect clients of the Mavio API, generated from `libs/proto`
//! into `libs/proto/gen/rust` by `pnpm nx run proto:generate`.

/// The messages of every `mavio.*.v1` package (buffa).
#[rustfmt::skip]
#[allow(missing_docs, clippy::all)]
#[path = "../../../proto/gen/rust/proto/mod.rs"]
pub mod proto;

/// The Connect clients of the services (connect-rust), which use the
/// messages of [`proto`].
#[rustfmt::skip]
#[allow(missing_docs, clippy::all)]
#[path = "../../../proto/gen/rust/connect/mod.rs"]
pub mod connect;
