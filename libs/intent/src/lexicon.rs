//! A language's words of numbers, units, ordinals and languages, and the
//! parsers of typed slots over them: a span of tokens is split into the
//! lexicon's phrases, longest first, and read as a number, a duration, a
//! position, an ordinal, a speed, a percentage or a language.

use std::collections::HashMap;

use crate::{data::GrammarFile, text::keys};

/// What a phrase of the lexicon is.
#[derive(Debug, Clone, PartialEq)]
pub enum Lex {
    /// A number's word.
    Num(f64),
    /// A word multiplying what comes before it.
    Mult(f64),
    And,
    Point,
    Half,
    /// A word meaning one before a unit.
    Article,
    Classifier,
    /// A unit of time, in seconds.
    Unit(f64),
    Ord(i32),
    OrdPrefix,
    OrdSuffix,
    Percent,
    Speed,
    Lang(String),
    /// A number in digits.
    Digits(f64),
    /// A time in digits with colons, in seconds.
    Clock(f64),
}

/// A language's lexicon.
#[derive(Debug, Default)]
pub struct Lexicon {
    /// Phrases by their first token.
    phrases: HashMap<String, Vec<(Vec<String>, Lex)>>,
}

impl Lexicon {
    /// new builds a grammar's lexicon.
    pub fn new(g: &GrammarFile) -> Self {
        let mut l = Lexicon::default();
        let n = &g.numbers;
        for (w, v) in &n.words {
            l.add(w, Lex::Num(*v));
        }
        if let Some(c) = &n.compound {
            for (u, uv) in &c.units {
                for (t, tv) in &c.tens {
                    l.add(
                        &c.pattern.replace("{unit}", u).replace("{ten}", t),
                        Lex::Num(uv + tv),
                    );
                }
            }
        }
        for (w, v) in &n.multipliers {
            l.add(w, Lex::Mult(*v));
        }
        for w in &n.half {
            l.add(w, Lex::Half);
        }
        for w in &n.and {
            l.add(w, Lex::And);
        }
        for w in &n.point {
            l.add(w, Lex::Point);
        }
        for w in &n.articles {
            l.add(w, Lex::Article);
        }
        for w in &n.classifiers {
            l.add(w, Lex::Classifier);
        }
        for (words, s) in [
            (&g.units.second, 1.0),
            (&g.units.minute, 60.0),
            (&g.units.hour, 3600.0),
        ] {
            for w in words {
                l.add(w, Lex::Unit(s));
            }
        }
        for (w, v) in &g.ordinals.words {
            l.add(w, Lex::Ord(*v));
        }
        for w in &g.ordinals.prefix {
            l.add(w, Lex::OrdPrefix);
        }
        for w in &g.ordinals.suffix {
            l.add(w, Lex::OrdSuffix);
        }
        for w in &g.percent {
            l.add(w, Lex::Percent);
        }
        for w in &g.speed {
            l.add(w, Lex::Speed);
        }
        for (code, names) in &g.languages {
            for w in names {
                l.add(w, Lex::Lang(code.clone()));
            }
        }
        for list in l.phrases.values_mut() {
            // Longest first, so that splitting prefers them.
            list.sort_by_key(|(k, _)| std::cmp::Reverse(k.len()));
        }
        l
    }

    fn add(&mut self, phrase: &str, lex: Lex) {
        let k = keys(phrase);
        if let Some(first) = k.first() {
            self.phrases
                .entry(first.clone())
                .or_default()
                .push((k, lex));
        }
    }

    /// split divides tokens into phrases of the lexicon, each with what it
    /// can be, preferring the fewest phrases; None when some token is not in
    /// the lexicon.
    pub fn split(&self, tokens: &[&str]) -> Option<Vec<Vec<Lex>>> {
        let n = tokens.len();
        // best[i]: the fewest phrases covering tokens[i..], with the length
        // of the first.
        let mut best: Vec<Option<(usize, usize)>> = vec![None; n + 1];
        best[n] = Some((0, 0));
        for i in (0..n).rev() {
            let mut lens: Vec<usize> = Vec::new();
            if digits(tokens[i]).is_some() {
                lens.push(1);
            }
            if let Some(list) = self.phrases.get(tokens[i]) {
                for (k, _) in list {
                    if k.len() <= n - i && k.iter().zip(&tokens[i..]).all(|(a, b)| a == b) {
                        lens.push(k.len());
                    }
                }
            }
            for len in lens {
                if let Some((count, _)) = best[i + len]
                    && best[i].is_none_or(|(c, _)| count + 1 < c)
                {
                    best[i] = Some((count + 1, len));
                }
            }
        }
        best[0]?;
        let mut out = Vec::new();
        let mut i = 0;
        while i < n {
            let (_, len) = best[i]?;
            let mut alts = Vec::new();
            if len == 1
                && let Some(d) = digits(tokens[i])
            {
                alts.push(d);
            }
            if let Some(list) = self.phrases.get(tokens[i]) {
                for (k, lex) in list {
                    if k.len() == len && k.iter().zip(&tokens[i..]).all(|(a, b)| a == b) {
                        alts.push(lex.clone());
                    }
                }
            }
            out.push(alts);
            i += len;
        }
        Some(out)
    }
}

/// digits reads a token of digits: a number, or a time with colons.
fn digits(t: &str) -> Option<Lex> {
    if !t.starts_with(|c: char| c.is_ascii_digit()) {
        return None;
    }
    if t.contains(':') {
        let parts: Vec<f64> = t
            .split(':')
            .map(|p| p.parse().ok())
            .collect::<Option<_>>()?;
        let s = match parts[..] {
            [m, s] if s < 60.0 => m * 60.0 + s,
            [h, m, s] if m < 60.0 && s < 60.0 => h * 3600.0 + m * 60.0 + s,
            _ => return None,
        };
        return Some(Lex::Clock(s));
    }
    t.parse().ok().map(Lex::Digits)
}

fn numeric(l: &Lex) -> bool {
    matches!(
        l,
        Lex::Num(_) | Lex::Mult(_) | Lex::And | Lex::Point | Lex::Half | Lex::Digits(_)
    )
}

/// number reads phrases as a number: digits, or words such as "twenty
/// three", "二十三", "quatre vingt dix", "两百零五", "one point five".
pub fn number(parts: &[Vec<Lex>]) -> Option<f64> {
    let lexes: Vec<&Lex> = parts
        .iter()
        .map(|alts| alts.iter().find(|l| numeric(l)))
        .collect::<Option<_>>()?;
    let mut total = 0.0;
    let mut section = 0.0;
    let mut current: Option<f64> = None;
    let mut fraction: Option<f64> = None;
    let mut half = false;
    let mut words = 0;
    for (i, l) in lexes.iter().enumerate() {
        if half {
            return None;
        }
        match **l {
            Lex::Digits(d) => {
                if current.is_some() || words > 0 || fraction.is_some() {
                    return None;
                }
                current = Some(d);
            }
            Lex::Num(v) => {
                words += 1;
                if let Some(f) = fraction {
                    if v >= 10.0 {
                        return None;
                    }
                    let place = fraction_places(f) + 1;
                    fraction = Some(f + v / 10f64.powi(place));
                    continue;
                }
                current = match current {
                    None => Some(v),
                    // "twenty three", "dix sept", "열 다섯", "quatre vingt dix".
                    Some(c)
                        if c >= 10.0
                            && c % 10.0 == 0.0
                            && c % 100.0 != 0.0
                            && v < 20.0
                            && (v < 10.0 || c % 20.0 == 0.0) =>
                    {
                        Some(c + v)
                    }
                    // "一百零五", "two hundred five": a digit after a section.
                    Some(0.0) => Some(v),
                    Some(_) => return None,
                };
            }
            Lex::Mult(m) => {
                words += 1;
                let c = current.take().unwrap_or(if section == 0.0 && total == 0.0 {
                    1.0
                } else {
                    0.0
                });
                if m >= 1000.0 {
                    total += (section + c).max(1.0) * m;
                    section = 0.0;
                } else if c == 0.0 {
                    // "一百零": the zero does not multiply.
                    return None;
                } else {
                    section += c * m;
                }
            }
            Lex::And => {
                if i == 0 || i == lexes.len() - 1 {
                    return None;
                }
            }
            Lex::Point => {
                if current.is_none() || fraction.is_some() {
                    return None;
                }
                fraction = Some(0.0);
            }
            Lex::Half => {
                half = true;
            }
            _ => return None,
        }
    }
    let whole = total + section + current.unwrap_or(0.0);
    let value = whole + fraction.unwrap_or(0.0) + if half { 0.5 } else { 0.0 };
    if lexes.is_empty() || (!half && current.is_none() && section == 0.0 && total == 0.0) {
        return None;
    }
    Some(value)
}

fn fraction_places(f: f64) -> i32 {
    let mut p = 0;
    let mut x = f;
    while (x - x.round()).abs() > 1e-9 && p < 6 {
        x *= 10.0;
        p += 1;
    }
    p
}

/// duration reads phrases as a length of time in seconds: "30 seconds",
/// "a minute and a half", "一分半", "一个半小时", "1 hour 20 minutes", "90s",
/// or a bare number of seconds.
pub fn duration(parts: &[Vec<Lex>]) -> Option<f64> {
    if parts.iter().all(|p| p.iter().any(numeric)) {
        return number(parts);
    }
    let has = |i: usize, f: &dyn Fn(&Lex) -> bool| parts.get(i).is_some_and(|p| p.iter().any(f));
    let unit_of = |i: usize| {
        parts.get(i).and_then(|p| {
            p.iter()
                .find_map(|l| if let Lex::Unit(s) = l { Some(*s) } else { None })
        })
    };
    let mut total = 0.0;
    let mut i = 0;
    let mut last_unit: Option<f64> = None;
    let mut components = 0;
    while i < parts.len() {
        if has(i, &|l| *l == Lex::And) && components > 0 {
            i += 1;
            continue;
        }
        // "and a half" after a component: half of its unit.
        if has(i, &|l| *l == Lex::Half)
            && unit_of(i + 1).is_none()
            && !has(i + 1, &|l| *l == Lex::Article)
        {
            total += 0.5 * last_unit?;
            i += 1;
            continue;
        }
        if has(i, &|l| *l == Lex::Article) && has(i + 1, &|l| *l == Lex::Half) {
            i += 1;
            continue;
        }
        // "half an hour", "半小时".
        if has(i, &|l| *l == Lex::Half) {
            let mut j = i + 1;
            if has(j, &|l| *l == Lex::Article) {
                j += 1;
            }
            let u = unit_of(j)?;
            total += 0.5 * u;
            last_unit = Some(u);
            components += 1;
            i = j + 1;
            continue;
        }
        // A number, then its unit.
        let start = i;
        let mut value = None;
        if has(i, &|l| *l == Lex::Article) {
            value = Some(1.0);
            i += 1;
        } else {
            let mut j = i;
            let digit = |p: &Vec<Lex>| {
                p.iter()
                    .any(|l| numeric(l) && *l != Lex::Half && *l != Lex::And)
            };
            // "treinta y cinco": "and" inside a number, between its words.
            while j < parts.len()
                && (digit(&parts[j])
                    || (j > start
                        && parts[j].contains(&Lex::And)
                        && parts.get(j + 1).is_some_and(digit)))
            {
                j += 1;
            }
            if j > start {
                value = Some(number(&parts[start..j])?);
                i = j;
            }
        }
        let mut v = value?;
        if has(i, &|l| *l == Lex::Classifier) {
            i += 1;
        }
        // "一个半小时": a half before the unit.
        if has(i, &|l| *l == Lex::Half) && unit_of(i + 1).is_some() {
            v += 0.5;
            i += 1;
        }
        match unit_of(i) {
            Some(u) => {
                total += v * u;
                last_unit = Some(u);
                i += 1;
            }
            // "一分三十": a number after minutes is seconds, after hours minutes.
            None if i == parts.len() && last_unit.is_some_and(|u| u > 1.0) => {
                total += v * last_unit? / 60.0;
            }
            None => return None,
        }
        components += 1;
    }
    (components > 0).then_some(total)
}

/// position reads phrases as a time in a video, in seconds: "1:20:00",
/// "1 hour 20 minutes", "一小时二十分".
pub fn position(parts: &[Vec<Lex>]) -> Option<f64> {
    if let [p] = parts
        && let Some(Lex::Clock(s)) = p.iter().find(|l| matches!(l, Lex::Clock(_)))
    {
        return Some(*s);
    }
    if parts.iter().all(|p| p.iter().any(numeric)) {
        return None;
    }
    duration(parts)
}

/// ordinal reads phrases as a position in a list: "second", "第二", "2nd",
/// "the last", "2番目", "두 번째".
pub fn ordinal(parts: &[Vec<Lex>]) -> Option<i32> {
    if let [p] = parts
        && let Some(Lex::Ord(o)) = p.iter().find(|l| matches!(l, Lex::Ord(_)))
    {
        return Some(*o);
    }
    let is = |p: &Vec<Lex>, l: Lex| p.contains(&l);
    let (first, rest) = parts.split_first()?;
    let body = if is(first, Lex::OrdPrefix) {
        rest
    } else if parts.last().is_some_and(|p| is(p, Lex::OrdSuffix)) {
        &parts[..parts.len() - 1]
    } else {
        return None;
    };
    // "第一个": a classifier may follow.
    let body = match body.split_last() {
        Some((last, init)) if is(last, Lex::Classifier) => init,
        _ => body,
    };
    let n = number(body)?;
    (n >= 1.0 && n.fract() == 0.0 && n < 1000.0).then_some(n as i32)
}

/// count reads phrases as a whole number: a season's or an episode's.
pub fn count(parts: &[Vec<Lex>]) -> Option<f64> {
    let n = number(parts)?;
    (n >= 0.0 && n.fract() == 0.0).then_some(n)
}

/// percent reads phrases as a percentage: "50", "50%", "百分之五十" (the
/// grammar's pattern takes "百分之"), "fifty percent".
pub fn percent(parts: &[Vec<Lex>]) -> Option<f64> {
    let body = match parts.split_last() {
        Some((last, init)) if last.contains(&Lex::Percent) => init,
        _ => parts,
    };
    let n = number(body)?;
    (0.0..=100.0).contains(&n).then_some(n)
}

/// speed reads phrases as a playback speed: "1.5", "1.5x", "1.5倍".
pub fn speed(parts: &[Vec<Lex>]) -> Option<f64> {
    let body = match parts.split_last() {
        Some((last, init)) if last.contains(&Lex::Speed) => init,
        _ => parts,
    };
    let n = number(body)?;
    (n > 0.0 && n <= 4.0).then_some(n)
}

/// language reads phrases as a language's ISO code.
pub fn language(parts: &[Vec<Lex>]) -> Option<String> {
    let [p] = parts else { return None };
    p.iter().find_map(|l| {
        if let Lex::Lang(c) = l {
            Some(c.clone())
        } else {
            None
        }
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::data::GrammarFile;

    fn lexicon(lang: &str) -> Lexicon {
        let files: Vec<GrammarFile> =
            serde_json::from_str(include_str!(concat!(env!("OUT_DIR"), "/grammars.json")))
                .expect("grammars");
        Lexicon::new(files.iter().find(|g| g.language == lang).expect("grammar"))
    }

    fn read<T>(lang: &str, text: &str, f: fn(&[Vec<Lex>]) -> Option<T>) -> Option<T> {
        let k = keys(text);
        let k: Vec<&str> = k.iter().map(String::as_str).collect();
        f(&lexicon(lang).split(&k)?)
    }

    #[test]
    fn numbers() {
        let cases: &[(&str, &str, Option<f64>)] = &[
            ("en", "twenty three", Some(23.0)),
            ("en", "two hundred and five", Some(205.0)),
            ("en", "one point five", Some(1.5)),
            ("en", "three thousand two hundred", Some(3200.0)),
            ("en", "twenty twenty", None),
            ("zh", "二十三", Some(23.0)),
            ("zh", "十五", Some(15.0)),
            ("zh", "两百零五", Some(205.0)),
            ("zh", "一万二千", Some(12000.0)),
            ("zh", "一点五", Some(1.5)),
            ("fr", "quatre vingt dix", Some(90.0)),
            ("fr", "soixante et onze", Some(71.0)),
            ("de", "einundzwanzig", Some(21.0)),
            ("ko", "열 다섯", Some(15.0)),
            ("ko", "삼십", Some(30.0)),
            ("es", "ciento veinte", Some(120.0)),
        ];
        for (lang, text, want) in cases {
            assert_eq!(read(lang, text, number), *want, "number({lang}: {text:?})");
        }
    }

    #[test]
    fn durations() {
        let cases: &[(&str, &str, Option<f64>)] = &[
            ("en", "30 seconds", Some(30.0)),
            ("en", "an hour and a half", Some(5400.0)),
            ("en", "half an hour", Some(1800.0)),
            ("en", "1 hour 20 minutes", Some(4800.0)),
            ("en", "90s", Some(90.0)),
            ("en", "45", Some(45.0)),
            ("zh", "一分半", Some(90.0)),
            ("zh", "一个半小时", Some(5400.0)),
            ("zh", "一分三十", Some(90.0)),
            ("zh", "半小时", Some(1800.0)),
            ("de", "eine halbe Stunde", Some(1800.0)),
            ("fr", "deux heures et demie", Some(9000.0)),
            ("es", "treinta y cinco segundos", Some(35.0)),
            ("en", "seconds", None),
        ];
        for (lang, text, want) in cases {
            assert_eq!(
                read(lang, text, duration),
                *want,
                "duration({lang}: {text:?})"
            );
        }
    }

    #[test]
    fn positions_and_ordinals() {
        assert_eq!(read("en", "1:20:00", position), Some(4800.0));
        assert_eq!(read("en", "5:30", position), Some(330.0));
        assert_eq!(read("en", "20", position), None);
        let cases: &[(&str, &str, Option<i32>)] = &[
            ("en", "second", Some(2)),
            ("en", "2nd", Some(2)),
            ("en", "last", Some(-1)),
            ("zh", "第三个", Some(3)),
            ("ja", "2番目", Some(2)),
            ("ko", "두 번째", Some(2)),
            ("en", "two", None),
        ];
        for (lang, text, want) in cases {
            assert_eq!(
                read(lang, text, ordinal),
                *want,
                "ordinal({lang}: {text:?})"
            );
        }
    }
}
