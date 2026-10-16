//! Compiled grammars and their matching. A pattern is written as words with
//! `(a|b)` for alternatives, `[a]` for what may be left out, `{name}` for a
//! slot of any words, `{name:type}` for a typed slot, `<list>` for one of a
//! grammar's lists and, in device templates, `@` for the pattern itself.
//! Matching walks a pattern's tree over a text's tokens, trying every way a
//! slot can end, and scores each way: words of a pattern count most, typed
//! slots next, fillers less and free slots least, so that the most specific
//! reading wins.

use std::collections::{BTreeMap, HashMap};

use crate::{
    data::{GrammarFile, SetValue},
    lexicon::{self, Lex, Lexicon},
    text::{keys, script_flag},
};

/// What a typed slot holds.
#[derive(Debug, Clone, Copy, PartialEq)]
pub enum SlotType {
    /// Any words.
    Free,
    Number,
    /// A whole number: a season, an episode.
    Count,
    /// A length of time.
    Duration,
    /// A time in a video.
    Position,
    Ordinal,
    Language,
    Percent,
    Speed,
}

impl SlotType {
    fn parse(name: &str) -> Result<Self, String> {
        Ok(match name {
            "number" => Self::Number,
            "count" => Self::Count,
            "duration" => Self::Duration,
            "position" => Self::Position,
            "ordinal" => Self::Ordinal,
            "language" => Self::Language,
            "percent" => Self::Percent,
            "speed" => Self::Speed,
            _ => return Err(format!("unknown slot type {name}")),
        })
    }

    /// The score of each token the slot takes.
    fn weight(self) -> f64 {
        if self == Self::Free { 0.5 } else { 3.0 }
    }
}

/// A slot's value.
#[derive(Debug, Clone, PartialEq)]
pub enum Value {
    /// The words alone.
    Text,
    Number(f64),
    Seconds(f64),
    Language(String),
    Ordinal(i32),
    Keyword(String),
}

#[derive(Debug, Clone)]
enum Node {
    Lit(String),
    Seq(Vec<Node>),
    Alt(Vec<Vec<Node>>),
    Opt(Vec<Node>),
    Slot(String, SlotType),
    Hole,
}

/// What a dialog must hold for a rule to apply.
#[derive(Debug, Clone, Copy, PartialEq)]
pub enum When {
    Confirmation,
    Results,
    Playing,
}

/// What the dialog holds.
#[derive(Debug, Clone, Copy, Default)]
pub struct State {
    pub confirmation: bool,
    pub results: i32,
    pub playing: bool,
}

impl State {
    fn holds(&self, w: When) -> bool {
        match w {
            When::Confirmation => self.confirmation,
            When::Results => self.results > 0,
            When::Playing => self.playing,
        }
    }
}

#[derive(Debug)]
struct Rule {
    intent: String,
    when: Option<When>,
    set: Vec<(String, Value)>,
    patterns: Vec<Vec<Node>>,
}

/// A language's compiled grammar.
#[derive(Debug)]
pub struct Grammar {
    pub language: String,
    pub scripts: u8,
    lexicon: Lexicon,
    prefix: Vec<Vec<String>>,
    suffix: Vec<Vec<String>>,
    rules: Vec<Rule>,
    pub replies: BTreeMap<String, String>,
    pub examples: Vec<String>,
}

struct Parser<'a> {
    chars: Vec<char>,
    i: usize,
    lists: &'a BTreeMap<String, Vec<String>>,
    depth: usize,
}

const SPECIAL: &[char] = &['(', ')', '|', '[', ']', '{', '}', '<', '>', '@'];

impl Parser<'_> {
    fn seq(&mut self, stops: &[char]) -> Result<Vec<Node>, String> {
        let mut nodes = Vec::new();
        while let Some(&c) = self.chars.get(self.i) {
            if stops.contains(&c) {
                return Ok(nodes);
            }
            match c {
                '(' => {
                    self.i += 1;
                    let mut alts = Vec::new();
                    loop {
                        alts.push(self.seq(&[')', '|'])?);
                        match self.chars.get(self.i) {
                            Some('|') => self.i += 1,
                            Some(')') => {
                                self.i += 1;
                                break;
                            }
                            _ => return Err("unclosed (".into()),
                        }
                    }
                    nodes.push(Node::Alt(alts));
                }
                '[' => {
                    // "[a|b]" is "[(a|b)]".
                    self.i += 1;
                    let mut alts = Vec::new();
                    loop {
                        alts.push(self.seq(&[']', '|'])?);
                        match self.chars.get(self.i) {
                            Some('|') => self.i += 1,
                            Some(']') => {
                                self.i += 1;
                                break;
                            }
                            _ => return Err("unclosed [".into()),
                        }
                    }
                    nodes.push(Node::Opt(if alts.len() == 1 {
                        alts.remove(0)
                    } else {
                        vec![Node::Alt(alts)]
                    }));
                }
                '{' => {
                    let body = self.until('}')?;
                    let (name, ty) = match body.split_once(':') {
                        Some((n, t)) => (n.trim().to_string(), SlotType::parse(t.trim())?),
                        None => (body.trim().to_string(), SlotType::Free),
                    };
                    nodes.push(Node::Slot(name, ty));
                }
                '<' => {
                    let name = self.until('>')?;
                    if self.depth > 4 {
                        return Err(format!("lists nest too deep at <{name}>"));
                    }
                    let items = self
                        .lists
                        .get(name.trim())
                        .ok_or_else(|| format!("unknown list <{name}>"))?;
                    let alts = items
                        .iter()
                        .map(|item| {
                            Parser {
                                chars: item.chars().collect(),
                                i: 0,
                                lists: self.lists,
                                depth: self.depth + 1,
                            }
                            .all()
                        })
                        .collect::<Result<_, _>>()?;
                    nodes.push(Node::Alt(alts));
                }
                '@' => {
                    self.i += 1;
                    nodes.push(Node::Hole);
                }
                ')' | '|' | ']' | '}' | '>' => return Err(format!("unexpected {c}")),
                _ => {
                    let start = self.i;
                    while self.chars.get(self.i).is_some_and(|c| !SPECIAL.contains(c)) {
                        self.i += 1;
                    }
                    let text: String = self.chars[start..self.i].iter().collect();
                    nodes.extend(keys(&text).into_iter().map(Node::Lit));
                }
            }
        }
        Ok(nodes)
    }

    fn until(&mut self, end: char) -> Result<String, String> {
        let start = self.i + 1;
        let len = self.chars[start..]
            .iter()
            .position(|&c| c == end)
            .ok_or_else(|| format!("unclosed {}", self.chars[self.i]))?;
        self.i = start + len + 1;
        Ok(self.chars[start..start + len].iter().collect())
    }

    fn all(mut self) -> Result<Vec<Node>, String> {
        let nodes = self.seq(&[])?;
        if self.i < self.chars.len() {
            return Err(format!("unexpected {}", self.chars[self.i]));
        }
        Ok(nodes)
    }
}

fn fill(nodes: &[Node], with: &[Node]) -> Vec<Node> {
    nodes
        .iter()
        .map(|n| match n {
            Node::Hole => Node::Seq(with.to_vec()),
            Node::Seq(v) => Node::Seq(fill(v, with)),
            Node::Opt(v) => Node::Opt(fill(v, with)),
            Node::Alt(a) => Node::Alt(a.iter().map(|v| fill(v, with)).collect()),
            n => n.clone(),
        })
        .collect()
}

impl Grammar {
    /// compile compiles a grammar's file.
    pub fn compile(g: &GrammarFile) -> Result<Self, String> {
        let parse = |p: &str| {
            Parser {
                chars: p.chars().collect(),
                i: 0,
                lists: &g.lists,
                depth: 0,
            }
            .all()
            .map_err(|e| format!("{}: {p:?}: {e}", g.language))
        };
        let devices: Vec<Vec<Node>> = g
            .device
            .iter()
            .map(|d| parse(d))
            .collect::<Result<_, _>>()?;
        let mut rules = Vec::new();
        for r in &g.intents {
            let mut patterns = Vec::new();
            for p in &r.patterns {
                let nodes = parse(p)?;
                if r.device {
                    for d in &devices {
                        patterns.push(fill(d, &nodes));
                    }
                }
                patterns.push(nodes);
            }
            let when = match r.when.as_deref() {
                None => None,
                Some("confirmation") => Some(When::Confirmation),
                Some("results") => Some(When::Results),
                Some("playing") => Some(When::Playing),
                Some(w) => return Err(format!("{}: unknown when {w}", g.language)),
            };
            let set = r
                .set
                .iter()
                .map(|(k, v)| {
                    (
                        k.clone(),
                        match v {
                            SetValue::Number(n) => Value::Number(*n),
                            SetValue::Text(t) => Value::Keyword(t.clone()),
                        },
                    )
                })
                .collect();
            rules.push(Rule {
                intent: r.intent.clone(),
                when,
                set,
                patterns,
            });
        }
        Ok(Grammar {
            language: g.language.clone(),
            scripts: g.scripts.iter().fold(0, |s, n| s | script_flag(n)),
            lexicon: Lexicon::new(g),
            prefix: g
                .fillers
                .prefix
                .iter()
                .map(|p| keys(p))
                .filter(|k| !k.is_empty())
                .collect(),
            suffix: g
                .fillers
                .suffix
                .iter()
                .map(|p| keys(p))
                .filter(|k| !k.is_empty())
                .collect(),
            rules,
            replies: g.replies.clone(),
            examples: g.examples.clone(),
        })
    }

    /// matches finds every reading of a text's tokens by the grammar: whole
    /// readings, or, when partial, readings of a part of the text without free
    /// slots.
    pub fn matches(&self, tokens: &[&str], state: &State, partial: bool) -> Vec<Found> {
        let n = tokens.len();
        let mut out = Vec::new();
        if n == 0 {
            return out;
        }
        let mut w = Walk {
            g: self,
            keys: tokens,
            partial,
            cache: HashMap::new(),
            caps: Vec::new(),
            score: 0.0,
            free: 0,
        };
        let (starts, ends) = if partial {
            (
                (0..n).map(|s| (s, 0.0)).collect(),
                (1..=n).map(|e| (e, 0.0)).collect(),
            )
        } else {
            (
                self.fillers(tokens, &self.prefix, false),
                self.fillers(tokens, &self.suffix, true),
            )
        };
        for (ri, rule) in self.rules.iter().enumerate() {
            if rule.when.is_some_and(|wh| !state.holds(wh)) {
                continue;
            }
            for pattern in &rule.patterns {
                for (&s, &sf) in &starts {
                    let cont = Cont {
                        nodes: pattern,
                        next: None,
                    };
                    w.walk(&cont, s, &mut |w, e| {
                        let Some(&ef) = ends.get(&e) else { return };
                        if e <= s {
                            return;
                        }
                        if partial && ((e - s) * 2 < n || (s == 0 && e == n)) {
                            return;
                        }
                        out.push(Found {
                            rule: ri,
                            start: s,
                            end: e,
                            score: w.score + sf + ef,
                            free: w.free > 0,
                            slots: w.caps.clone(),
                        });
                    });
                }
            }
        }
        out
    }

    /// The intent of a rule, and the slots it sets.
    pub fn rule(&self, i: usize) -> (&str, &[(String, Value)]) {
        (&self.rules[i].intent, &self.rules[i].set)
    }

    /// fillers finds where a pattern may start (or, from the end, where it
    /// may end) after fillers, with the score of the fillers' tokens.
    fn fillers(
        &self,
        tokens: &[&str],
        phrases: &[Vec<String>],
        from_end: bool,
    ) -> HashMap<usize, f64> {
        let n = tokens.len();
        let mut best: HashMap<usize, f64> = HashMap::new();
        let origin = if from_end { n } else { 0 };
        best.insert(origin, 0.0);
        let mut queue = vec![origin];
        while let Some(p) = queue.pop() {
            let score = best[&p];
            for ph in phrases {
                let (lo, hi) = if from_end {
                    if ph.len() > p {
                        continue;
                    }
                    (p - ph.len(), p)
                } else {
                    if p + ph.len() > n {
                        continue;
                    }
                    (p, p + ph.len())
                };
                if ph.iter().zip(&tokens[lo..hi]).all(|(a, b)| a == b) {
                    let next = if from_end { lo } else { hi };
                    let s = score + ph.len() as f64;
                    if best.get(&next).is_none_or(|&b| s > b) {
                        best.insert(next, s);
                        queue.push(next);
                    }
                }
            }
        }
        best
    }
}

/// A slot a reading took.
#[derive(Debug, Clone)]
pub struct Capture {
    pub name: String,
    /// The tokens it took.
    pub start: usize,
    pub end: usize,
    pub value: Value,
}

/// A reading of a text.
#[derive(Debug, Clone)]
pub struct Found {
    pub rule: usize,
    /// The tokens the pattern took.
    pub start: usize,
    pub end: usize,
    pub score: f64,
    /// Whether a free slot took words.
    pub free: bool,
    pub slots: Vec<Capture>,
}

struct Cont<'a> {
    nodes: &'a [Node],
    next: Option<&'a Cont<'a>>,
}

struct Walk<'a> {
    g: &'a Grammar,
    keys: &'a [&'a str],
    partial: bool,
    cache: HashMap<(usize, usize), Option<Vec<Vec<Lex>>>>,
    caps: Vec<Capture>,
    score: f64,
    free: usize,
}

/// The most tokens a typed slot takes.
const TYPED: usize = 10;

impl Walk<'_> {
    fn walk(&mut self, cont: &Cont<'_>, pos: usize, sink: &mut dyn FnMut(&Self, usize)) {
        let Some((node, rest)) = cont.nodes.split_first() else {
            match cont.next {
                Some(next) => self.walk(next, pos, sink),
                None => sink(self, pos),
            }
            return;
        };
        let rest = Cont {
            nodes: rest,
            next: cont.next,
        };
        match node {
            Node::Lit(k) => {
                if self.keys.get(pos).is_some_and(|t| t == k) {
                    self.score += 4.0;
                    self.walk(&rest, pos + 1, sink);
                    self.score -= 4.0;
                }
            }
            Node::Seq(v) => self.walk(
                &Cont {
                    nodes: v,
                    next: Some(&rest),
                },
                pos,
                sink,
            ),
            Node::Alt(alts) => {
                for a in alts {
                    self.walk(
                        &Cont {
                            nodes: a,
                            next: Some(&rest),
                        },
                        pos,
                        sink,
                    );
                }
            }
            Node::Opt(v) => {
                self.walk(
                    &Cont {
                        nodes: v,
                        next: Some(&rest),
                    },
                    pos,
                    sink,
                );
                self.walk(&rest, pos, sink);
            }
            Node::Hole => self.walk(&rest, pos, sink),
            Node::Slot(name, ty) => {
                if *ty == SlotType::Free && self.partial {
                    return;
                }
                let n = self.keys.len();
                let max = if *ty == SlotType::Free {
                    n
                } else {
                    n.min(pos + TYPED)
                };
                for end in pos + 1..=max {
                    let Some(value) = self.value(*ty, pos, end) else {
                        continue;
                    };
                    let gain = ty.weight() * (end - pos) as f64;
                    self.caps.push(Capture {
                        name: name.clone(),
                        start: pos,
                        end,
                        value,
                    });
                    self.score += gain;
                    self.free += usize::from(*ty == SlotType::Free);
                    self.walk(&rest, end, sink);
                    self.free -= usize::from(*ty == SlotType::Free);
                    self.score -= gain;
                    self.caps.pop();
                }
            }
        }
    }

    fn value(&mut self, ty: SlotType, s: usize, e: usize) -> Option<Value> {
        if ty == SlotType::Free {
            return Some(Value::Text);
        }
        let g = self.g;
        let keys = self.keys;
        let parts = self
            .cache
            .entry((s, e))
            .or_insert_with(|| g.lexicon.split(&keys[s..e]))
            .as_deref()?;
        match ty {
            SlotType::Free => Some(Value::Text),
            SlotType::Number => lexicon::number(parts).map(Value::Number),
            SlotType::Count => lexicon::count(parts).map(Value::Number),
            SlotType::Duration => lexicon::duration(parts).map(Value::Seconds),
            SlotType::Position => lexicon::position(parts).map(Value::Seconds),
            SlotType::Ordinal => lexicon::ordinal(parts).map(Value::Ordinal),
            SlotType::Language => lexicon::language(parts).map(Value::Language),
            SlotType::Percent => lexicon::percent(parts).map(Value::Number),
            SlotType::Speed => lexicon::speed(parts).map(Value::Number),
        }
    }
}
