//! The languages the app is shown in, which every call asks the servers
//! for in its `Accept-Language` header, as the web UI's do: the servers
//! translate what they name in them, such as the plugins.

use std::sync::RwLock;

use http::HeaderValue;
use http::header::ACCEPT_LANGUAGE;

/// The header's value, none until the app sets its languages.
static ACCEPT: RwLock<Option<HeaderValue>> = RwLock::new(None);

/// Sets the languages the app is shown in, most wanted first, as BCP 47
/// tags such as `zh-Hans`; none asks for none.
pub fn set_languages(tags: &[String]) {
    let value = header(tags);
    if let Ok(mut accept) = ACCEPT.write() {
        *accept = value;
    }
}

/// The header asking for tags, each less wanted than the one before.
fn header(tags: &[String]) -> Option<HeaderValue> {
    let tags: Vec<&str> = tags
        .iter()
        .map(|t| t.trim())
        .filter(|t| {
            !t.is_empty()
                && t.chars()
                    .all(|c| c.is_ascii_alphanumeric() || c == '-' || c == '_')
        })
        .take(9)
        .collect();
    let value = tags
        .iter()
        .enumerate()
        .map(|(i, t)| {
            let tag = t.replace('_', "-");
            if i == 0 {
                tag
            } else {
                format!("{tag};q=0.{}", 9 - i)
            }
        })
        .collect::<Vec<_>>()
        .join(", ");
    (!value.is_empty())
        .then(|| HeaderValue::from_str(&value).ok())
        .flatten()
}

/// Asks for the app's languages in request, unless it asks already.
pub(crate) fn ask<B>(request: &mut http::Request<B>) {
    if request.headers().contains_key(ACCEPT_LANGUAGE) {
        return;
    }
    if let Some(value) = ACCEPT.read().ok().and_then(|a| a.clone()) {
        request.headers_mut().insert(ACCEPT_LANGUAGE, value);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn header_weighs_each_language_less() {
        let tags = |t: &[&str]| t.iter().map(|s| s.to_string()).collect::<Vec<_>>();
        let cases = [
            (tags(&["zh-Hans"]), Some("zh-Hans")),
            (tags(&["zh_Hans_CN", "en"]), Some("zh-Hans-CN, en;q=0.8")),
            (tags(&["", " fr ", "de"]), Some("fr, de;q=0.8")),
            (tags(&["a\nb"]), None),
            (tags(&[]), None),
        ];
        for (input, want) in cases {
            let got = header(&input);
            assert_eq!(
                got.as_ref().and_then(|v| v.to_str().ok()),
                want,
                "header({input:?})"
            );
        }
    }
}
