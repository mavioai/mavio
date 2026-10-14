# Generates golden.json: what ftfy's fix_text, as vendored by Bazarr, makes of
# the texts below with the options Bazarr saves subtitles with
# (custom_libs/subliminal_patch/subtitle.py), for the Go tests of package
# subtitle to compare FixText against. It also records the tables ftfy builds
# at import time, which the tests compare the embedded data/ftfy.json against.
# Run it with the Bazarr checkout recorded in golden.json:
#
#   python3 -I golden.py /path/to/bazarr > golden.json
#
# It needs only the Python standard library besides Bazarr's vendored libs.
import hashlib
import json
import os
import subprocess
import sys

BAZARR = os.path.abspath(sys.argv[1])
sys.path[:0] = [os.path.join(BAZARR, "libs")]

import ftfy  # noqa: E402
from ftfy import badness, chardata  # noqa: E402

OPTIONS = {"uncurl_quotes": False, "fix_character_width": False}


def moji(text, encoding):
    """Text written as UTF-8 and read back in another encoding."""
    return text.encode("utf-8").decode(encoding)


# name, input
CASES = [
    # UTF-8 read as Windows-1252 or Latin-1
    ("e_acute", "Ã©"),
    ("apostrophe", "doesnâ€™t"),
    ("curly_quotes_lossy_c1", "â€œquotedâ€\x9d"),
    ("so", "sÃ³"),
    ("schon", "schÃ¶n"),
    ("check_mark", "âœ” No problems"),
    ("shrug", "&macr;\\_(ã\x83\x84)_/&macr;"),
    ("sentence_1252", moji("Le cœur a ses raisons — « déjà vu » à 5 €.", "sloppy-windows-1252")),
    ("sentence_latin1", moji("Größe: ½ Maß, Straße & Ärger", "latin-1")),
    ("doubly_encoded", "The Mona Lisa doesnÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢t have eyebrows."),
    ("doubly_encoded_generated", moji(moji("Ça va très bien, merci.", "sloppy-windows-1252"), "sloppy-windows-1252")),
    ("cesu8", "\xed\xa0\xbd\xed\xb8\x8d is a smiley".encode("latin-1").decode("sloppy-windows-1252")),
    ("java_null", (b"nul\xc0\x80here " + "é".encode()).decode("latin-1")),
    # Latin-1 that meant Windows-1252, and stray C1 controls
    ("latin1_meant_1252", "\x93quoted\x94 and \x85"),
    ("c1_undefined_in_1252", "\x81\x93quoted\x94"),
    ("c1_with_cjk", "\x93引用\x94"),
    # Other single-byte encodings
    ("cyrillic_1251", moji("Привет, как дела? Всё хорошо.", "sloppy-windows-1251")),
    ("central_european_1250", moji("Příliš žluťoučký kůň úpěl ďábelské ódy.", "sloppy-windows-1250")),
    ("greek_1253", moji("Γειά σου Κόσμε, τι κάνεις;", "sloppy-windows-1253")),
    ("turkish_1254", moji("Şişli'de ığdır çağrı ödül.", "sloppy-windows-1254")),
    ("baltic_1257", moji("Ąžuolas ir šešėlis, ūsai.", "sloppy-windows-1257")),
    ("iso_8859_2", moji("Zażółć gęślą jaźń", "iso-8859-2")),
    ("macroman", moji("Café crème brûlée", "macroman")),
    ("cp437", moji("Señor Müller café", "cp437")),
    # Byte A0 turned into a space
    ("a0_voila", "voilÃ le travail"),
    ("a0_a_la_mode", "Ã la mode"),
    ("a0_portuguese", "Ã s vezes Ã quele"),
    ("a0_nbsp", "Price:Â 5Â â‚¬"),
    # Lossy sequences
    ("lossy_replacement", "â€œ like this â€�"),
    ("lossy_question_mark", "cafÃ? and â€?"),
    # Mixed encodings that cannot be fixed as a whole
    ("inconsistent", "Il Ã©tait une fois ★ un garÃ§on"),
    ("inconsistent_cyrillic", "Ã©tÃ© — Привет"),
    # HTML entities
    ("entities", "Tom &amp; Jerry &eacute; &#233; &#x41; &#X42; &foo; &EACUTE; &nTILDE; &NTILDE;"),
    ("entities_numeric_edge", "&#59; &#0; &#13; &#128; &#129; &#1; &#xD800; &#x110000; &#12abc; &#99999999999999999999;"),
    ("entities_upper", "BUNDESSTRA&SZLIG;E P&EACUTE;REZ &AMP; &LT;&GT;"),
    ("entities_named", "&Jscr;ohn &HilbertSpace;ancock &checkmark; &hellip; &not &nbsp;x"),
    ("entities_with_tag", "<i>Tom &amp; Jerry</i> &eacute;"),
    ("entities_sticky_tag", "&eacute; first\n<b>tag</b> &amp;\n&eacute; after"),
    ("entities_flubberific", "Broken text&hellip; it&#x2019;s ﬂubberiﬁc!"),
    # Terminal escapes
    ("terminal_escapes", "\x1b[36;44mI'm blue, da ba dee\x1b[0m \x1b[1A"),
    # Ligatures and digraphs
    ("ligatures", "ﬁnal ﬂuﬃest ﬆ ﬅ Ĳssel ǅ ŉ"),
    # Line breaks
    ("line_breaks", "a\r\nb\rc d e\x85f"),
    # Control characters
    ("control_chars", "a\x00b\x07c\x0bd\x7fe﻿f⁪g￼h￹i\tj\x0ck‍l"),
    # Normalization
    ("nfc", "é Å ñ 가"),
    # Fixes that Bazarr turns off
    ("curly_quotes_kept", "“Hello,” she said. ‘Yes’ ʼ"),
    ("full_width_kept", "ＬＯＵＤ　ＮＯＩＳＥＳ ｳﾙﾄﾗ"),
    # Correct text that must stay unchanged
    ("chinese", "敏捷的棕色狐狸跳过了懒惰的狗。“你好”"),
    ("japanese", "素早い茶色の狐が怠け者の犬を飛び越える。「こんにちは」"),
    ("korean", "빠른 갈색 여우가 게으른 개를 뛰어넘습니다."),
    ("arabic", "الثعلب البني السريع يقفز فوق الكلب الكسول."),
    ("hebrew", "השועל החום המהיר קופץ מעל הכלב העצלן."),
    ("russian", "Съешь же ещё этих мягких французских булок, да выпей чаю."),
    ("greek", "Ξεσκεπάζω την ψυχοφθόρα βδελυγμία."),
    ("emoji", "Party 🎉🏴󠁧󠁢󠁷󠁬󠁳󠁿 👩‍👩‍👧 ❤️"),
    ("thai_hindi", "ภาษาไทย हिन्दी"),
    ("kaomoji", "(╯°□°)╯︵ ┻━┻ ¯\\_(ツ)_/¯"),
    ("latin_accents", "Ça, où, naïve, coöperate, résumé, Ångström, Øre, smörgåsbord"),
    ("symbols", "© 2024 ® ™ ±5 °C 10³ ½ ¼ € £ ¥ § ¶"),
    ("ascii", "Just plain ASCII text, nothing to fix here!"),
    ("empty", ""),
    # Subtitles
    ("srt_mojibake", "1\r\n00:00:01,000 --> 00:00:02,500\r\nÃ‡a va, mon cÅ“ur ?\r\n\r\n"
                     "2\r\n00:00:03,000 --> 00:00:04,000\r\nâ€” Oui, trÃ¨s bien.\r\n"),
    ("srt_tags", "1\n00:00:01,000 --> 00:00:02,500\n<i>Itâ€™s fine &amp; dandy</i>\n\n"
                 "2\n00:00:03,000 --> 00:00:04,000\nTom &amp; Jerry\n"),
    ("srt_cyrillic", "1\n00:00:01,000 --> 00:00:02,000\n" + moji("Что ты делаешь?", "sloppy-windows-1251") + "\n"),
    ("srt_correct", "1\n00:00:01,000 --> 00:00:02,000\n¿Qué pasa? ¡Nada!\n\n2\n00:00:03,000 --> 00:00:04,000\n— Él dijo «sí».\n"),
]

# name, text whose UTF-8 encoding with lone surrogates (WTF-8) is the input.
SURROGATE_CASES = [
    ("surrogate_pair", "smile 😀 ok"),
    ("surrogate_lone", "lone \udca9 and \ud83d end"),
    ("surrogate_reversed", "\udca9\ud83d"),
]

# name, unit, prefix, count: inputs too long to record, built as
# prefix + unit * count, whose output is recorded by its SHA-256.
LONG_CASES = [
    ("long_line_split", "Ã©", "x", 600000),
    ("long_lines", "Ã© ok\n", "", 50000),
]


def entry(name, text):
    return {"name": name, "input": text, "output": ftfy.fix_text(text, **OPTIONS)}


def codec_table(encoding):
    return [ord(c) if c != "�" else -1 for c in bytes(range(256)).decode(encoding, errors="replace")]


def tables():
    codecs = {enc: [ord(c) for c in bytes(range(256)).decode(enc)] for enc in chardata.CHARMAP_ENCODINGS}
    codecs["windows-1252"] = codec_table("windows-1252")
    return {
        "html_entities": chardata.HTML_ENTITIES,
        "codecs": codecs,
        "mojibake_categories": badness.MOJIBAKE_CATEGORIES,
        "utf8_clues": chardata.UTF8_CLUES,
    }


def main():
    commit = subprocess.run(["git", "-C", BAZARR, "rev-parse", "HEAD"], capture_output=True, text=True,
                            check=True).stdout.strip()
    cases = [entry(n, t) for n, t in CASES]
    for name, text in SURROGATE_CASES:
        cases.append({
            "name": name,
            "input_wtf8_hex": text.encode("utf-8", "surrogatepass").hex(),
            "output": ftfy.fix_text(text, **OPTIONS),
        })
    for name, unit, prefix, count in LONG_CASES:
        out = ftfy.fix_text(prefix + unit * count, **OPTIONS)
        cases.append({
            "name": name,
            "repeat": {"prefix": prefix, "unit": unit, "count": count},
            "output_sha256": hashlib.sha256(out.encode("utf-8")).hexdigest(),
        })
    out = {
        "$comment": "Generated by golden.py from ftfy as Bazarr vendors it. Do not edit.",
        "source": {"repository": "https://github.com/morpheus65535/bazarr", "commit": commit,
                   "path": "libs/ftfy", "version": ftfy.__version__, "license": "Apache-2.0"},
        "python": sys.version.split()[0],
        "options": OPTIONS,
        "cases": cases,
        "tables": tables(),
    }
    json.dump(out, sys.stdout, ensure_ascii=False, indent=1)
    sys.stdout.write("\n")


main()
