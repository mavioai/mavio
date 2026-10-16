//! Runs the cases of testdata/cases/<language>.yaml: each is a request with
//! the intent and the slots it must be read as.

use std::{collections::BTreeMap, fs};

use mavio_intent::proto::{
    __buffa::oneof::slot, DialogState, ExamplesRequest, IntentMatch, ParseIntentRequest,
};
use serde::Deserialize;

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Case {
    text: String,
    #[serde(default)]
    languages: Vec<String>,
    #[serde(default)]
    confirmation: bool,
    #[serde(default)]
    results: i32,
    /// The intent, or none when nothing must match.
    intent: Option<String>,
    #[serde(default)]
    slots: BTreeMap<String, serde_norway::Value>,
    #[serde(default)]
    partial: bool,
}

fn check(lang: &str, c: &Case) -> Result<(), String> {
    let languages = if c.languages.is_empty() {
        vec![lang.to_string()]
    } else {
        c.languages.clone()
    };
    let mut req = ParseIntentRequest {
        text: Some(c.text.clone()),
        languages,
        ..Default::default()
    };
    if c.confirmation || c.results > 0 {
        req.state = DialogState {
            confirmation: Some(c.confirmation),
            results: Some(c.results),
            ..Default::default()
        }
        .into();
    }
    let res = mavio_intent::parse(&req);
    let readings: Vec<_> = res
        .intents
        .iter()
        .map(|i| {
            (
                i.name.clone().unwrap_or_default(),
                i.slots
                    .iter()
                    .map(|s| {
                        (
                            s.name.clone().unwrap_or_default(),
                            s.text.clone().unwrap_or_default(),
                        )
                    })
                    .collect::<Vec<_>>(),
            )
        })
        .collect();
    let got = res.intents.first();
    let Some(want) = &c.intent else {
        return match got {
            None => Ok(()),
            Some(_) => Err(format!("got = {readings:?}, want = none")),
        };
    };
    let Some(got) = got else {
        return Err(format!("got = none, want = {want}"));
    };
    if got.name.as_deref() != Some(want) {
        return Err(format!("got = {readings:?}, want = {want}"));
    }
    let partial = got.r#match.as_ref().and_then(|m| m.as_known()) == Some(IntentMatch::Partial);
    if partial != c.partial {
        return Err(format!("partial: got = {partial}, want = {}", c.partial));
    }
    for (name, v) in &c.slots {
        let Some(s) = got.slots.iter().find(|s| s.name.as_deref() == Some(name)) else {
            return Err(format!(
                "slot {name}: got = none, want = {v:?}; readings = {readings:?}"
            ));
        };
        let ok = match (&s.value, v) {
            (
                Some(slot::Value::Number(n) | slot::Value::Seconds(n)),
                serde_norway::Value::Number(w),
            ) => (n - w.as_f64().unwrap_or(f64::NAN)).abs() < 1e-9,
            (Some(slot::Value::Ordinal(n)), serde_norway::Value::Number(w)) => {
                Some(*n as i64) == w.as_i64()
            }
            (
                Some(slot::Value::Language(l) | slot::Value::Keyword(l)),
                serde_norway::Value::String(w),
            ) => l == w,
            (None, serde_norway::Value::String(w)) => s.text.as_deref() == Some(w),
            _ => false,
        };
        if !ok {
            return Err(format!(
                "slot {name}: got = {:?} {:?}, want = {v:?}",
                s.text, s.value
            ));
        }
    }
    Ok(())
}

#[test]
fn cases() {
    let mut failures = Vec::new();
    let mut total = 0;
    for lang in mavio_intent::languages().languages {
        let path = format!("testdata/cases/{lang}.yaml");
        let text = fs::read_to_string(&path).unwrap_or_else(|_| panic!("{path} is missing"));
        let cases: Vec<Case> =
            serde_norway::from_str(&text).unwrap_or_else(|e| panic!("{path}: {e}"));
        for c in &cases {
            total += 1;
            if let Err(e) = check(&lang, c) {
                failures.push(format!("{lang}: {:?}: {e}", c.text));
            }
        }
    }
    assert!(
        failures.is_empty(),
        "{} of {total} cases failed:\n{}",
        failures.len(),
        failures.join("\n")
    );
}

#[test]
fn examples_are_understood() {
    let mut failures = Vec::new();
    for lang in mavio_intent::languages().languages {
        let ex = mavio_intent::examples(&ExamplesRequest {
            language: Some(lang.clone()),
            ..Default::default()
        });
        for e in ex.examples {
            let res = mavio_intent::parse(&ParseIntentRequest {
                text: Some(e.clone()),
                languages: vec![lang.clone()],
                ..Default::default()
            });
            let full = res
                .intents
                .first()
                .and_then(|i| i.r#match.as_ref())
                .and_then(|m| m.as_known())
                == Some(IntentMatch::Full);
            if !full {
                failures.push(format!("{lang}: {e:?}"));
            }
        }
    }
    assert!(
        failures.is_empty(),
        "examples not understood:\n{}",
        failures.join("\n")
    );
}
