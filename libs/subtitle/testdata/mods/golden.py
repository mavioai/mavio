# Generates golden.json: the results of Bazarr's own Subzero mods, language
# guessing and hearing-impaired detection on the subtitles of this directory,
# for the Go tests of package subtitle to compare against. Run it with the
# Bazarr checkout recorded in golden.json:
#
#   python3 -I golden.py /path/to/bazarr > golden.json
#
# It needs only the Python standard library besides Bazarr's vendored libs.
import ast
import json
import os
import re
import subprocess
import sys

BAZARR = os.path.abspath(sys.argv[1])
HERE = os.path.dirname(os.path.abspath(__file__))
sys.path[:0] = [os.path.join(BAZARR, "custom_libs"), os.path.join(BAZARR, "libs")]

from subzero.language import Language  # noqa: E402
from subzero.modification import SubtitleModifications  # noqa: E402
from guess_language import guess_language  # noqa: E402

# name, file, Bazarr language (alpha3), BCP 47 tag for Mavio, mods
SCENARIOS = [
    ("test_apply_mods_remove_hi", "subs_for_mods.srt", "eng", "en", ["remove_HI", "OCR_fixes"]),
    ("parse", "edge_en.srt", "eng", "en", []),
    ("remove_HI", "edge_en.srt", "eng", "en", ["remove_HI"]),
    ("OCR_fixes", "edge_en.srt", "eng", "en", ["OCR_fixes"]),
    ("common", "edge_en.srt", "eng", "en", ["common"]),
    ("common_unknown_language", "edge_en.srt", "fra", "fr", ["common"]),
    ("all_text_mods", "edge_en.srt", "eng", "en", ["common", "remove_HI", "OCR_fixes", "remove_tags"]),
    ("remove_tags", "edge_en.srt", "eng", "en", ["remove_tags"]),
    ("emoji", "edge_en.srt", "eng", "en", ["emoji"]),
    ("color", "edge_en.srt", "eng", "en", ["color(name=dark-red)", "common"]),
    ("color_unknown", "edge_en.srt", "eng", "en", ["color(name=pink)"]),
    ("shift_offset", "edge_en.srt", "eng", "en", ["shift_offset(h=0,m=0,s=1,ms=500)", "shift_offset(h=0,m=0,s=-3,ms=20)"]),
    ("shift_offset_zero", "edge_en.srt", "eng", "en", ["shift_offset(h=0,m=0,s=1,ms=0)", "shift_offset(h=0,m=0,s=-1,ms=0)", "common"]),
    ("change_FPS", "edge_en.srt", "eng", "en", ["change_FPS(from=23.976,to=25)"]),
    ("two_point_fit", "edge_en.srt", "eng", "en",
     ["two_point_fit(rh=0,rm=0,rs=1,rms=0,oh=0,om=0,os=2,oms=500,from=23.976,to=24)"]),
    ("fix_uppercase_not_upper", "edge_en.srt", "eng", "en", ["fix_uppercase"]),
    ("fix_uppercase", "edge_upper.srt", "eng", "en", ["fix_uppercase", "remove_HI"]),
    ("remove_HI_upper", "edge_upper.srt", "eng", "en", ["remove_HI"]),
    ("reverse_rtl", "edge_he.srt", "heb", "he", ["reverse_rtl", "common"]),
    ("reverse_rtl_wrong_language", "edge_en.srt", "eng", "en", ["reverse_rtl"]),
    ("common_spanish", "edge_es.srt", "spa", "es", ["common", "OCR_fixes"]),
    ("remove_HI_japanese", "edge_ja.srt", "jpn", "ja", ["remove_HI", "common"]),
    ("OCR_fixes_german", "edge_de.srt", "deu", "de", ["OCR_fixes", "remove_HI"]),
    ("ass", "subs.ass", "eng", "en", ["common", "remove_HI", "OCR_fixes"]),
    ("ass_remove_tags", "subs.ass", "eng", "en", ["remove_tags"]),
]

# Short texts for language guessing.
TEXTS = {
    "en": "The quick brown fox jumps over the lazy dog while the children watch from the window.",
    "de": "Der schnelle braune Fuchs springt über den faulen Hund, während die Kinder aus dem Fenster schauen.",
    "fr": "Le renard brun rapide saute par-dessus le chien paresseux pendant que les enfants regardent.",
    "es": "El rápido zorro marrón salta sobre el perro perezoso mientras los niños miran por la ventana.",
    "it": "La volpe marrone veloce salta sopra il cane pigro mentre i bambini guardano dalla finestra.",
    "pt": "A rápida raposa marrom pula sobre o cão preguiçoso enquanto as crianças olham pela janela.",
    "nl": "De snelle bruine vos springt over de luie hond terwijl de kinderen uit het raam kijken.",
    "sv": "Den snabba bruna räven hoppar över den lata hunden medan barnen tittar ut genom fönstret.",
    "pl": "Szybki brązowy lis przeskakuje nad leniwym psem, a dzieci patrzą przez okno.",
    "ru": "Быстрая коричневая лиса прыгает через ленивую собаку, пока дети смотрят в окно.",
    "uk": "Швидка руда лисиця стрибає через ледачого собаку, поки діти дивляться у вікно.",
    "el": "Η γρήγορη καφέ αλεπού πηδάει πάνω από το τεμπέλικο σκυλί ενώ τα παιδιά κοιτάζουν.",
    "ar": "الثعلب البني السريع يقفز فوق الكلب الكسول بينما ينظر الأطفال من النافذة.",
    "he": "השועל החום המהיר קופץ מעל הכלב העצלן בזמן שהילדים מסתכלים מהחלון.",
    "ja": "素早い茶色の狐が怠け者の犬を飛び越えるのを、子供たちが窓から見ています。",
    "ko": "빠른 갈색 여우가 게으른 개를 뛰어넘는 것을 아이들이 창문으로 보고 있습니다.",
    "zh-Hans": "敏捷的棕色狐狸跳过了懒惰的狗，孩子们正在从窗户里看着这一切。",
    "zh-Hant": "敏捷的棕色狐狸跳過了懶惰的狗，孩子們正在從窗戶裡看著這一切。",
    "th": "สุนัขจิ้งจอกสีน้ำตาลกระโดดข้ามสุนัขขี้เกียจขณะที่เด็กๆ มองจากหน้าต่าง",
    "hi": "तेज़ भूरी लोमड़ी आलसी कुत्ते के ऊपर कूदती है जबकि बच्चे खिड़की से देखते हैं।",
    "tr": "Hızlı kahverengi tilki tembel köpeğin üzerinden atlarken çocuklar pencereden bakıyor.",
    "fi": "Nopea ruskea kettu hyppää laiskan koiran yli, kun lapset katsovat ikkunasta.",
    "short": "Hi there",
    "numbers": "1234 5678 -- !!",
}

# Texts for hearing-impaired detection, with the alpha3 language passed.
HI_TEXTS = [
    ("srt_brackets", "1\n00:00:01,000 --> 00:00:02,000\n[DOOR SLAMS]\n", "eng"),
    ("srt_parentheses", "1\n00:00:01,000 --> 00:00:02,000\n(laughing) Hi.\n", "eng"),
    ("srt_parentheses_arabic", "1\n00:00:01,000 --> 00:00:02,000\n(ملاحظة) مرحبا\n", "ara"),
    ("srt_brackets_arabic", "1\n00:00:01,000 --> 00:00:02,000\n[موسيقى] مرحبا\n", "ara"),
    ("srt_music", "1\n00:00:01,000 --> 00:00:02,000\n♪ La la la ♪\n", "eng"),
    ("srt_plain", "1\n00:00:01,000 --> 00:00:02,000\nHello there.\n", "eng"),
    ("srt_short_brackets", "1\n00:00:01,000 --> 00:00:02,000\n[a] b\n", "eng"),
    ("srt_an8", "1\n00:00:01,000 --> 00:00:02,000\n{\\an8}Top line\n", "eng"),
    ("srt_an8_then_brackets", "1\n00:00:01,000 --> 00:00:02,000\n{\\an8}[NOISE] here\n", "eng"),
    ("srt_multiline", "1\n00:00:01,000 --> 00:00:02,000\n[DOOR\nSLAMS]\n", "eng"),
    ("no_language", "1\n00:00:01,000 --> 00:00:02,000\n(laughing)\n", None),
]


def load_parse_for_hi_regex():
    """Loads parse_for_hi_regex and its expressions from subliminal_patch/core.py
    without importing it, which would import every provider."""
    path = os.path.join(BAZARR, "custom_libs", "subliminal_patch", "core.py")
    with open(path, encoding="utf-8") as f:
        tree = ast.parse(f.read())
    wanted = {"HI_REGEX_WITHOUT_PARENTHESIS", "HI_REGEX_WITH_PARENTHESIS", "HI_REGEX_PARENTHESIS_EXCLUDED_LANGUAGES",
              "parse_for_hi_regex"}
    body = [n for n in tree.body
            if (isinstance(n, ast.Assign) and any(getattr(t, "id", None) in wanted for t in n.targets))
            or (isinstance(n, ast.FunctionDef) and n.name in wanted)]
    ns = {"re": re}
    exec(compile(ast.Module(body=body, type_ignores=[]), path, "exec"), ns)
    return ns["parse_for_hi_regex"]


parse_for_hi_regex = load_parse_for_hi_regex()


def read(name):
    with open(os.path.join(HERE, name), encoding="utf-8") as f:
        return f.read()


def run(name, file, alpha3, tag, mods):
    m = SubtitleModifications()
    m.load(content=read(file), language=Language(alpha3), mods=mods)
    m.modify(*mods)
    cues = [[e.start, e.end, e.text.replace("\\N", "\n")] for e in m.f.events if e.type == "Dialogue"]
    return {"name": name, "file": file, "language": tag, "mods": mods, "used": m.mods_used, "cues": cues}


def main():
    commit = subprocess.run(["git", "-C", BAZARR, "rev-parse", "HEAD"], capture_output=True, text=True,
                            check=True).stdout.strip()
    out = {
        "$comment": "Generated by golden.py from Bazarr's own code. Do not edit.",
        "source": {"repository": "https://github.com/morpheus65535/bazarr", "commit": commit},
        "mods": [run(*s) for s in SCENARIOS],
        "languages": [{"name": k, "text": v, "guess": str(guess_language(v) or "")} for k, v in TEXTS.items()],
        "files": [{"file": f, "guess": str(guess_language(read(f)) or "")}
                  for f in ("edge_en.srt", "edge_de.srt", "edge_es.srt", "edge_he.srt", "edge_ja.srt",
                            "subs_for_mods.srt")],
        "hearing_impaired": [{"name": n, "text": t, "language": l or "", "hi": parse_for_hi_regex(t, l)}
                             for n, t, l in HI_TEXTS],
    }
    json.dump(out, sys.stdout, ensure_ascii=False, indent=1)
    sys.stdout.write("\n")


main()
