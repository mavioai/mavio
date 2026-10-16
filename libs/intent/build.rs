// Reads the grammars (grammars/*.yaml) and writes them as one JSON array
// into OUT_DIR, which the crate includes and loads on first use; YAML is read
// only here, so that the WASM module carries no YAML parser.

#![allow(dead_code)]

use std::{env, fs, path::Path};

include!("src/data.rs");

fn main() {
    println!("cargo:rerun-if-changed=grammars");
    let mut paths: Vec<_> = fs::read_dir("grammars")
        .expect("grammars")
        .map(|e| e.expect("grammars").path())
        .filter(|p| p.extension().is_some_and(|e| e == "yaml"))
        .collect();
    paths.sort();
    let mut grammars = Vec::new();
    for path in paths {
        println!("cargo:rerun-if-changed={}", path.display());
        let text = fs::read_to_string(&path).expect("grammar");
        let g: GrammarFile =
            serde_norway::from_str(&text).unwrap_or_else(|e| panic!("{}: {e}", path.display()));
        let stem = path
            .file_stem()
            .and_then(|s| s.to_str())
            .unwrap_or_default();
        assert_eq!(
            g.language,
            stem,
            "{}: language must be the file's name",
            path.display()
        );
        for rule in &g.intents {
            if rule.device {
                assert!(
                    !g.device.is_empty(),
                    "{}: {} names devices but the grammar has no device patterns",
                    path.display(),
                    rule.intent
                );
            }
            if let Some(w) = &rule.when {
                assert!(
                    matches!(w.as_str(), "confirmation" | "results" | "playing"),
                    "{}: unknown when {w}",
                    path.display()
                );
            }
        }
        grammars.push(g);
    }
    let out = Path::new(&env::var("OUT_DIR").expect("OUT_DIR")).join("grammars.json");
    fs::write(out, serde_json::to_string(&grammars).expect("json")).expect("write");
}
