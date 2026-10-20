//! Sets the core's version as the server's: a release's from MAVIO_VERSION
//! (v0.1.0 or 0.1.0), else 0.0.0 and the commit it is built at, else dev.

use std::path::Path;
use std::process::Command;

fn main() {
    println!("cargo::rerun-if-env-changed=MAVIO_VERSION");
    let version = match std::env::var("MAVIO_VERSION") {
        Ok(v) if !v.is_empty() => v.trim_start_matches('v').to_owned(),
        _ => match git(&["rev-parse", "--short=12", "HEAD"]) {
            Some(revision) => {
                // Built again when the commit moves: HEAD, the branch it is on
                // and the packed refs, those that exist.
                for path in ["HEAD", "packed-refs"]
                    .map(str::to_owned)
                    .into_iter()
                    .chain(git(&["symbolic-ref", "-q", "HEAD"]))
                {
                    if let Some(file) =
                        git(&["rev-parse", "--path-format=absolute", "--git-path", &path])
                        && Path::new(&file).exists()
                    {
                        println!("cargo::rerun-if-changed={file}");
                    }
                }
                format!("{}-{revision}", env!("CARGO_PKG_VERSION"))
            }
            None => "dev".to_owned(),
        },
    };
    println!("cargo::rustc-env=MAVIO_CORE_VERSION={version}");
}

/// The trimmed output of git, run in the crate's directory, if it succeeds.
fn git(args: &[&str]) -> Option<String> {
    let out = Command::new("git")
        .args(args)
        .current_dir(env!("CARGO_MANIFEST_DIR"))
        .output()
        .ok()?;
    let text = String::from_utf8(out.stdout).ok()?.trim().to_owned();
    (out.status.success() && !text.is_empty()).then_some(text)
}
