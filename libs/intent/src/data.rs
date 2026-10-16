// The grammar files' contents (grammars/<language>.yaml), read by build.rs,
// which checks them and writes them as JSON for the crate to load. This file
// is included by build.rs as well as being a module of the crate.

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

/// One language's grammar.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct GrammarFile {
    /// Its language, BCP 47, as short as it can be: "en", "zh".
    pub language: String,
    /// The scripts its patterns are written in: latin, han, kana, hangul,
    /// cyrillic. Its patterns are tried on texts with one of them.
    pub scripts: Vec<String>,
    /// Phrases that may come before or after any pattern: "please", "请".
    #[serde(default)]
    pub fillers: Fillers,
    /// Named lists of phrases, used in patterns as `<name>`.
    #[serde(default)]
    pub lists: BTreeMap<String, Vec<String>>,
    /// Words of numbers.
    pub numbers: Numbers,
    /// Words of units of time.
    pub units: Units,
    /// Words of ordinals.
    pub ordinals: Ordinals,
    /// Words after a number that make it a percentage.
    #[serde(default)]
    pub percent: Vec<String>,
    /// Words after a number that make it a speed: "x", "倍".
    #[serde(default)]
    pub speed: Vec<String>,
    /// The names of languages in this one, by ISO 639-1 code.
    pub languages: BTreeMap<String, Vec<String>>,
    /// How a device is named around a pattern of a rule with `device`,
    /// `@` standing for the pattern: "@ on [the] {device}".
    #[serde(default)]
    pub device: Vec<String>,
    /// What requests mean.
    pub intents: Vec<Rule>,
    /// The sentences of answers, by key, with `{name}` arguments.
    pub replies: BTreeMap<String, String>,
    /// Examples of requests, shown when nothing matched.
    pub examples: Vec<String>,
}

/// Phrases allowed around patterns.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Fillers {
    #[serde(default)]
    pub prefix: Vec<String>,
    #[serde(default)]
    pub suffix: Vec<String>,
}

/// Words of numbers. Every key is a phrase, matched after normalization.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Numbers {
    /// Words with their values: "one" 1, "twenty" 20, "两" 2.
    pub words: BTreeMap<String, f64>,
    /// Words multiplying what comes before: "hundred" 100, "十" 10.
    #[serde(default)]
    pub multipliers: BTreeMap<String, f64>,
    /// Words of a half: "half", "半".
    #[serde(default)]
    pub half: Vec<String>,
    /// Words joining the parts of a number: "and", "et".
    #[serde(default)]
    pub and: Vec<String>,
    /// Words of a decimal point: "point".
    #[serde(default)]
    pub point: Vec<String>,
    /// Words meaning one before a unit: "a", "an", "un".
    #[serde(default)]
    pub articles: Vec<String>,
    /// Words between a number and its unit: "个", "つ".
    #[serde(default)]
    pub classifiers: Vec<String>,
    /// Compound words of numbers below 100, such as German's
    /// "einundzwanzig": `{unit}` and `{ten}` are replaced by each key of
    /// `units` and `tens`.
    #[serde(default)]
    pub compound: Option<Compound>,
}

/// Compound words of numbers.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Compound {
    pub pattern: String,
    pub units: BTreeMap<String, f64>,
    pub tens: BTreeMap<String, f64>,
}

/// Words of units of time.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Units {
    pub second: Vec<String>,
    pub minute: Vec<String>,
    pub hour: Vec<String>,
}

/// Words of ordinals.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Ordinals {
    /// Words with their positions; -1 is the last.
    pub words: BTreeMap<String, i32>,
    /// Words before a number making it an ordinal: "第", "number".
    #[serde(default)]
    pub prefix: Vec<String>,
    /// Words after a number making it an ordinal: "th", "番目", "번째".
    #[serde(default)]
    pub suffix: Vec<String>,
}

/// A rule: patterns meaning one intent.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Rule {
    pub intent: String,
    /// What the dialog must hold for the rule to apply: confirmation,
    /// results or playing.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub when: Option<String>,
    /// Whether the patterns may name a device, as the grammar's `device`
    /// says.
    #[serde(default)]
    pub device: bool,
    /// Slots the rule sets when its patterns do not.
    #[serde(default)]
    pub set: BTreeMap<String, SetValue>,
    pub patterns: Vec<String>,
}

/// A slot's value a rule sets.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum SetValue {
    Number(f64),
    Text(String),
}
