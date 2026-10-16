//! Normalization and tokens: a text becomes tokens whose keys are what
//! patterns match, each keeping where it was in the text, so that a slot's
//! words are given as they were said.

use unicode_normalization::{UnicodeNormalization, char::is_combining_mark};

use crate::hans;

/// A token of a text.
#[derive(Debug, Clone, PartialEq)]
pub struct Token {
    /// What patterns match: lowercase, of one width, without the accents
    /// of Latin letters, Chinese in its simplified form.
    pub key: String,
    /// Where it was in the text, in bytes.
    pub start: usize,
    pub end: usize,
}

/// Scripts of a text, as flags.
pub const LATIN: u8 = 1;
/// Chinese characters, which Japanese uses too.
pub const HAN: u8 = 2;
/// Japanese kana.
pub const KANA: u8 = 4;
/// Korean.
pub const HANGUL: u8 = 8;
/// Russian and the other Cyrillic languages.
pub const CYRILLIC: u8 = 16;

/// The flag of a script's name in a grammar.
pub fn script_flag(name: &str) -> u8 {
    match name {
        "latin" => LATIN,
        "han" => HAN,
        "kana" => KANA,
        "hangul" => HANGUL,
        "cyrillic" => CYRILLIC,
        _ => 0,
    }
}

fn script(c: char) -> u8 {
    match c as u32 {
        0x3040..=0x30ff | 0x31f0..=0x31ff => KANA,
        0x3400..=0x4dbf | 0x4e00..=0x9fff | 0xf900..=0xfaff | 0x20000..=0x3134f => HAN,
        0xac00..=0xd7af | 0x1100..=0x11ff | 0x3130..=0x318f => HANGUL,
        0x0400..=0x052f => CYRILLIC,
        _ if c.is_alphabetic()
            && (c.is_ascii() || (c as u32) < 0x0250 || (0x1e00..=0x1eff).contains(&(c as u32))) =>
        {
            LATIN
        }
        _ => 0,
    }
}

/// Whether a character is a token by itself: the scripts written without
/// spaces between words, and Korean, whose words carry particles.
fn single(c: char) -> bool {
    script(c) & (HAN | KANA | HANGUL) != 0 || c == 'ー'
}

/// fold appends the normalized form of a character to out.
fn fold(c: char, out: &mut String) {
    match c {
        'ß' => out.push_str("ss"),
        '’' | '‘' | '`' | '´' => out.push('\''),
        _ => {
            for l in c.to_lowercase() {
                if l.is_ascii() || script(l) & (HAN | KANA | HANGUL) != 0 {
                    out.push(hans::simplify(l));
                    continue;
                }
                // The accents of letters go: "subtítulos" matches "subtitulos".
                for d in l.nfd() {
                    if !is_combining_mark(d) {
                        out.push(d);
                    }
                }
            }
        }
    }
}

/// A character of the normalized text with where its original was.
struct Folded {
    c: char,
    start: usize,
    end: usize,
}

fn normalize(text: &str) -> Vec<Folded> {
    let mut out = Vec::new();
    let mut chars = text.char_indices().peekable();
    let mut unit = String::new();
    let mut folded = String::new();
    while let Some((start, c)) = chars.next() {
        // A character with the marks that follow it is normalized as one, so
        // that a decomposed kana and its voicing mark compose.
        unit.clear();
        unit.push(c);
        let mut end = start + c.len_utf8();
        while let Some(&(i, m)) = chars.peek() {
            // Half-width kana's voicing marks are letters, yet compose too.
            if !is_combining_mark(m) && m != '\u{ff9e}' && m != '\u{ff9f}' {
                break;
            }
            unit.push(m);
            end = i + m.len_utf8();
            chars.next();
        }
        folded.clear();
        for n in unit.nfkc() {
            fold(n, &mut folded);
        }
        for f in folded.chars() {
            out.push(Folded { c: f, start, end });
        }
    }
    out
}

fn word_char(c: char) -> bool {
    c.is_alphanumeric() && !single(c)
}

/// tokenize splits a text into tokens: a character at a time in Chinese,
/// Japanese and Korean, words elsewhere, letters and digits apart ("30s" is
/// "30" and "s"), numbers with their decimal points and times with their
/// colons whole ("1.5", "1:20:00"), and "%" alone.
pub fn tokenize(text: &str) -> Vec<Token> {
    let chars = normalize(text);
    let mut tokens = Vec::new();
    let mut i = 0;
    while i < chars.len() {
        let c = chars[i].c;
        if single(c) {
            tokens.push(Token {
                key: c.to_string(),
                start: chars[i].start,
                end: chars[i].end,
            });
            i += 1;
        } else if c.is_ascii_digit() {
            let mut j = i;
            let mut key = String::new();
            while j < chars.len() {
                let d = chars[j].c;
                let joins = matches!(d, '.' | ',' | ':')
                    && j > i
                    && chars.get(j + 1).is_some_and(|n| n.c.is_ascii_digit())
                    && chars[j - 1].c.is_ascii_digit();
                if d.is_ascii_digit() {
                    key.push(d);
                } else if joins {
                    key.push(if d == ',' { '.' } else { d });
                } else {
                    break;
                }
                j += 1;
            }
            tokens.push(Token {
                key,
                start: chars[i].start,
                end: chars[j - 1].end,
            });
            i = j;
        } else if word_char(c) {
            let mut j = i;
            let mut key = String::new();
            while j < chars.len() && word_char(chars[j].c) && !chars[j].c.is_ascii_digit() {
                key.push(chars[j].c);
                j += 1;
            }
            tokens.push(Token {
                key,
                start: chars[i].start,
                end: chars[j - 1].end,
            });
            i = j;
        } else if c == '%' {
            tokens.push(Token {
                key: "%".into(),
                start: chars[i].start,
                end: chars[i].end,
            });
            i += 1;
        } else {
            i += 1;
        }
    }
    tokens
}

/// The scripts a text is written in.
pub fn scripts(text: &str) -> u8 {
    text.chars().fold(0, |s, c| s | script(c))
}

/// keys tokenizes a phrase of a grammar into its keys.
pub fn keys(phrase: &str) -> Vec<String> {
    tokenize(phrase).into_iter().map(|t| t.key).collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn k(text: &str) -> Vec<String> {
        keys(text)
    }

    #[test]
    fn tokens() {
        let cases: &[(&str, &[&str])] = &[
            ("Back 30 seconds!", &["back", "30", "seconds"]),
            ("后退30秒", &["后", "退", "30", "秒"]),
            ("後退三十秒", &["后", "退", "三", "十", "秒"]),
            ("ＰＬＡＹ　Ｘ", &["play", "x"]),
            ("go to 1:20:00", &["go", "to", "1:20:00"]),
            ("1.5x speed", &["1.5", "x", "speed"]),
            ("s02e03", &["s", "02", "e", "03"]),
            ("Subtítulos en español", &["subtitulos", "en", "espanol"]),
            ("l’anglais", &["l", "anglais"]),
            ("Straße", &["strasse"]),
            ("자막 켜줘", &["자", "막", "켜", "줘"]),
            ("ﾃﾞｨｽﾞﾆｰ", &["デ", "ィ", "ズ", "ニ", "ー"]),
            ("50%", &["50", "%"]),
            ("1,5 Minuten", &["1.5", "minuten"]),
        ];
        for (text, want) in cases {
            assert_eq!(k(text), *want, "tokenize({text:?})");
        }
    }

    #[test]
    fn spans() {
        let text = "Play Le Bureau des Légendes";
        let t = tokenize(text);
        assert_eq!(&text[t[1].start..t[4].end], "Le Bureau des Légendes");
    }
}
