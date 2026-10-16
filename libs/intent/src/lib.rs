//! The assistant's fast path: grammars of requests in several languages,
//! which tell what a request means without AI, and the sentences of the
//! answers. The web UI and the server run the crate as one WASM module
//! (`wasm.rs`); requests and answers are the messages of `mavio.intent.v1`.

pub mod data;
pub mod grammar;
mod hans;
pub mod lexicon;
pub mod text;
#[cfg(target_arch = "wasm32")]
mod wasm;

use std::sync::OnceLock;

use grammar::{Found, Grammar, State, Value};

/// The messages of `mavio.intent.v1`.
#[allow(missing_docs, clippy::all)]
pub mod proto {
    include!("../../proto/gen/rust/proto/mavio.intent.v1.mod.rs");
}

use proto::{
    __buffa::oneof::slot, ExamplesRequest, ExamplesResponse, Intent, IntentMatch,
    LanguagesResponse, ParseIntentRequest, ParseIntentResponse, ReplyRequest, ReplyResponse, Slot,
};

/// The most readings a request is given.
const READINGS: usize = 3;

/// grammars returns the compiled grammars, in the order of their languages.
pub fn grammars() -> &'static [Grammar] {
    static GRAMMARS: OnceLock<Vec<Grammar>> = OnceLock::new();
    GRAMMARS.get_or_init(|| {
        let files: Vec<data::GrammarFile> =
            serde_json::from_str(include_str!(concat!(env!("OUT_DIR"), "/grammars.json")))
                .expect("grammars");
        files
            .iter()
            .map(|f| Grammar::compile(f).unwrap_or_else(|e| panic!("{e}")))
            .collect()
    })
}

/// Whether a language is Chinese written in Traditional characters.
fn traditional(tag: &str) -> bool {
    let t = tag.to_ascii_lowercase();
    t.starts_with("zh")
        && ["hant", "tw", "hk", "mo"]
            .iter()
            .any(|s| t.split(['-', '_']).skip(1).any(|p| p == *s))
}

/// The language of a BCP 47 tag without its region and script.
fn base(tag: &str) -> String {
    tag.split(['-', '_'])
        .next()
        .unwrap_or_default()
        .to_ascii_lowercase()
}

/// find returns the grammar of a language, or of English.
fn find(tag: &str) -> &'static Grammar {
    let b = base(tag);
    let gs = grammars();
    gs.iter()
        .find(|g| g.language == b)
        .or_else(|| gs.iter().find(|g| g.language == "en"))
        .unwrap_or(&gs[0])
}

/// The tag answers in a grammar are given in: the user's own tag when the
/// grammar is its language's.
fn tag_of(g: &Grammar, tags: &[String]) -> String {
    tags.iter()
        .find(|t| base(t) == g.language)
        .cloned()
        .unwrap_or_else(|| g.language.clone())
}

struct Reading {
    found: Found,
    grammar: usize,
    rank: usize,
    full: bool,
}

/// parse tells what a request means: its readings, the best first.
pub fn parse(req: &ParseIntentRequest) -> ParseIntentResponse {
    let text = req.text.as_deref().unwrap_or_default();
    let toks = text::tokenize(text);
    let keys: Vec<&str> = toks.iter().map(|t| t.key.as_str()).collect();
    let state = req
        .state
        .as_option()
        .map_or_else(State::default, |s| State {
            confirmation: s.confirmation.unwrap_or_default(),
            results: s.results.unwrap_or_default(),
            playing: s.playing.unwrap_or_default(),
        });
    let tags = &req.languages;
    let gs = grammars();
    let rank = |g: &Grammar| {
        tags.iter()
            .position(|t| base(t) == g.language)
            .unwrap_or(tags.len())
    };
    let scripts = text::scripts(text);
    let eligible: Vec<usize> = (0..gs.len())
        .filter(|&i| {
            if scripts == 0 {
                rank(&gs[i]) < tags.len() || gs[i].language == "en"
            } else {
                gs[i].scripts & scripts != 0
            }
        })
        .collect();
    let mut readings = Vec::new();
    for partial in [false, true] {
        for &gi in &eligible {
            for found in gs[gi].matches(&keys, &state, partial) {
                readings.push(Reading {
                    found,
                    grammar: gi,
                    rank: rank(&gs[gi]),
                    full: !partial,
                });
            }
        }
        if !readings.is_empty() {
            break;
        }
    }
    readings.sort_by(|a, b| {
        let cov = |r: &Reading| (r.found.end - r.found.start) as f64;
        b.full
            .cmp(&a.full)
            .then(if a.full {
                std::cmp::Ordering::Equal
            } else {
                cov(b).total_cmp(&cov(a))
            })
            .then(b.found.score.total_cmp(&a.found.score))
            .then(a.rank.cmp(&b.rank))
            .then(a.grammar.cmp(&b.grammar))
    });
    let mut intents: Vec<Intent> = Vec::new();
    for r in readings {
        let g = &gs[r.grammar];
        let intent = to_intent(g, &r, text, &toks, keys.len(), tags);
        if intents
            .iter()
            .any(|i| i.name == intent.name && i.slots == intent.slots)
        {
            continue;
        }
        intents.push(intent);
        if intents.len() == READINGS {
            break;
        }
    }
    ParseIntentResponse {
        intents,
        ..Default::default()
    }
}

fn to_intent(
    g: &Grammar,
    r: &Reading,
    text: &str,
    toks: &[text::Token],
    n: usize,
    tags: &[String],
) -> Intent {
    let (name, set) = g.rule(r.found.rule);
    let mut slots: Vec<Slot> = r
        .found
        .slots
        .iter()
        .map(|c| {
            slot(
                &c.name,
                &text[toks[c.start].start..toks[c.end - 1].end],
                &c.value,
            )
        })
        .collect();
    for (k, v) in set {
        if !slots.iter().any(|s| s.name.as_deref() == Some(k)) {
            slots.push(slot(k, "", v));
        }
    }
    let confidence = if r.full {
        if r.found.free { 0.9 } else { 1.0 }
    } else {
        0.6 * (r.found.end - r.found.start) as f64 / n as f64
    };
    Intent {
        name: Some(name.to_string()),
        slots,
        r#match: Some(
            if r.full {
                IntentMatch::Full
            } else {
                IntentMatch::Partial
            }
            .into(),
        ),
        confidence: Some(confidence),
        language: Some(tag_of(g, tags)),
        ..Default::default()
    }
}

fn slot(name: &str, text: &str, v: &Value) -> Slot {
    let value = match v {
        Value::Text => None,
        Value::Number(n) => Some(slot::Value::Number(*n)),
        Value::Seconds(s) => Some(slot::Value::Seconds(*s)),
        Value::Language(l) => Some(slot::Value::Language(l.clone())),
        Value::Ordinal(o) => Some(slot::Value::Ordinal(*o)),
        Value::Keyword(k) => Some(slot::Value::Keyword(k.clone())),
    };
    Slot {
        name: Some(name.to_string()),
        text: Some(text.to_string()),
        value,
        ..Default::default()
    }
}

/// reply writes an answer's sentence in a language, or in English when the
/// language has none.
pub fn reply(req: &ReplyRequest) -> ReplyResponse {
    let tag = req.language.as_deref().unwrap_or("en");
    let key = req.key.as_deref().unwrap_or_default();
    let mut g = find(tag);
    let template = match g.replies.get(key) {
        Some(t) => t,
        None => {
            g = find("en");
            g.replies.get(key).map_or("", String::as_str)
        }
    };
    let mut out = template.to_string();
    for (k, v) in &req.args {
        out = out.replace(&format!("{{{k}}}"), v);
    }
    let (text, language) = localize(g, tag, out);
    ReplyResponse {
        text: Some(text),
        language: Some(language),
        ..Default::default()
    }
}

/// localize writes a grammar's text for a tag: Chinese in Traditional
/// characters for the Traditional tags.
fn localize(g: &Grammar, tag: &str, text: String) -> (String, String) {
    if g.language == "zh" && traditional(tag) {
        return (hans::traditional(&text), tag.to_string());
    }
    let language = if base(tag) == g.language {
        tag.to_string()
    } else {
        g.language.clone()
    };
    (text, language)
}

/// examples returns examples of requests in a language.
pub fn examples(req: &ExamplesRequest) -> ExamplesResponse {
    let tag = req.language.as_deref().unwrap_or("en");
    let g = find(tag);
    let mut language = g.language.clone();
    let examples = g
        .examples
        .iter()
        .map(|e| {
            let (t, l) = localize(g, tag, e.clone());
            language = l;
            t
        })
        .collect();
    ExamplesResponse {
        examples,
        language: Some(language),
        ..Default::default()
    }
}

/// languages returns the languages of the grammars.
pub fn languages() -> LanguagesResponse {
    LanguagesResponse {
        languages: grammars().iter().map(|g| g.language.clone()).collect(),
        ..Default::default()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn reply_in(language: &str, key: &str, args: &[(&str, &str)]) -> (String, String) {
        let r = reply(&ReplyRequest {
            language: Some(language.into()),
            key: Some(key.into()),
            args: args
                .iter()
                .map(|(k, v)| (k.to_string(), v.to_string()))
                .collect(),
            ..Default::default()
        });
        (r.text.unwrap_or_default(), r.language.unwrap_or_default())
    }

    #[test]
    fn replies() {
        let cases = [
            ("en-US", "playing", ("Playing Dune.", "en-US")),
            ("zh-CN", "playing", ("正在播放《Dune》。", "zh-CN")),
            ("zh-TW", "resumed", ("繼續播放。", "zh-TW")),
            ("zh-Hant", "paused", ("已暫停。", "zh-Hant")),
            ("pt-BR", "playing", ("Playing Dune.", "en")),
        ];
        for (language, key, (text, lang)) in cases {
            let got = reply_in(language, key, &[("title", "Dune")]);
            assert_eq!(
                got,
                (text.to_string(), lang.to_string()),
                "reply({language}, {key})"
            );
        }
    }

    #[test]
    fn every_grammar_has_every_reply() {
        let en = &find("en").replies;
        for g in grammars() {
            for k in en.keys() {
                assert!(
                    g.replies.contains_key(k),
                    "{}: reply {k} is missing",
                    g.language
                );
            }
            for k in g.replies.keys() {
                assert!(
                    en.contains_key(k),
                    "{}: reply {k} is not in English",
                    g.language
                );
            }
        }
    }

    #[test]
    fn language_order_breaks_ties() {
        // "pause" is French and German as well as English.
        for (languages, want) in [
            (vec!["fr"], "fr"),
            (vec!["en"], "en"),
            (vec!["de", "fr"], "de"),
        ] {
            let res = parse(&ParseIntentRequest {
                text: Some("pause".into()),
                languages: languages.iter().map(|s| s.to_string()).collect(),
                ..Default::default()
            });
            assert_eq!(
                res.intents[0].language.as_deref(),
                Some(want),
                "{languages:?}"
            );
        }
    }
}
