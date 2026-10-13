"""Writes the subtitle pairs of testdata/cases and the alignments ffsubsync
finds for them (cases.json), which the Go tests compare against.

The alignments come from ffsubsync 0.5.0 (revision
d69d3f98aa0dfbca5d1f7932143c220c6aac278c, MIT,
https://github.com/smacke/ffsubsync), as vendored by Bazarr in libs/, run
through its command line entry point with Bazarr's arguments and a subtitle
file as the reference.

    python golden.py <directory holding the ffsubsync package>

needs numpy and ffsubsync's dependencies (srt, pysubs2, charset_normalizer,
tqdm, ffmpeg-python, rich).
"""

import json
import logging
import os
import random
import sys
import tempfile
from datetime import timedelta

sys.path.insert(0, sys.argv[1])

import srt  # noqa: E402
from ffsubsync import aligners, ffsubsync  # noqa: E402

logging.disable(logging.CRITICAL)
HERE = os.path.dirname(os.path.abspath(__file__))
CASES = os.path.join(HERE, "cases")

# Bazarr's defaults: framerate fixing off, at most 60 s of offset; the
# quality gate applies when framerate fixing is on.
BAZARR = ["--max-offset-seconds", "60", "--output-encoding", "same"]
NO_FIX = ["--no-fix-framerate", "--gss"]
FIX = ["--skip-sync-on-low-quality", "--min-score", "0.0",
       "--quality-max-offset-seconds", "30", "--max-framerate-deviation", "0.1"]


def truth(seed, count=320):
    """Dialogue of about twenty minutes: cues of 0.8 to 5 s, gaps of 0.2 to
    9 s."""
    rnd = random.Random(seed)
    cues, t = [], 5.0 + rnd.uniform(0, 20)
    for i in range(count):
        d = rnd.uniform(0.8, 5.0)
        cues.append((t, t + d, "Line %d of the dialogue." % (i + 1)))
        t += d + (rnd.uniform(0.2, 1.5) if rnd.random() < 0.7 else rnd.uniform(1.5, 9.0))
    return cues


def distort(cues, scale=1.0, shift=0.0, jitter=0.0, drop=0.0, seed=0):
    rnd = random.Random(seed)
    out = []
    for s, e, text in cues:
        if rnd.random() < drop:
            continue
        j = rnd.uniform(-jitter, jitter) if jitter else 0.0
        s2, e2 = s * scale + shift + j, e * scale + shift + j
        if s2 < 0:
            continue
        out.append((s2, e2, text))
    return out


def write(name, cues):
    subs = [srt.Subtitle(i + 1, timedelta(seconds=round(s, 3)), timedelta(seconds=round(e, 3)), text)
            for i, (s, e, text) in enumerate(cues)]
    with open(os.path.join(CASES, name), "w", encoding="utf-8") as f:
        f.write(srt.compose(subs, reindex=False))


METADATA = [
    (1.0, 4.0, "English subtitles by The Team - www.example.org"),
    (6.0, 9.0, "[MUSIC PLAYING]"),
    (9.5, 11.0, "♪ ♪"),
]


def cases():
    base = truth(1)
    yield "shift_forward", base, distort(base, shift=-3.21), NO_FIX
    yield "shift_backward", base, distort(base, shift=7.5), NO_FIX
    yield "jitter", base, distort(base, shift=12.34, jitter=0.25, drop=0.1, seed=2), NO_FIX
    yield "metadata", base, METADATA + distort(base, shift=4.0) + [(1300.0, 1303.0, "Subtitles by someone - English")], NO_FIX
    yield "beyond_max_offset", base, distort(base, shift=-90.0), NO_FIX
    yield "framerate_pal", base, distort(base, scale=23.976 / 25.0, shift=1.0), FIX
    yield "framerate_film", base, distort(base, scale=24.0 / 23.976, shift=-2.0), FIX
    other = truth(3, 200)
    yield "gss", other, distort(other, scale=1.013, shift=-2.0), FIX + ["--gss"]


def main():
    os.makedirs(CASES, exist_ok=True)
    scores = []
    original = aligners.MaxScoreAligner.transform

    def capture(self, *a):
        out = original(self, *a)
        scores.append(float(out[0][0]))
        return out

    aligners.MaxScoreAligner.transform = capture
    golden = []
    with tempfile.TemporaryDirectory() as tmp:
        for name, ref, sub, extra in cases():
            write(name + ".ref.srt", ref)
            write(name + ".srt", sub)
            scores.clear()
            argv = [os.path.join(CASES, name + ".ref.srt"), "-i", os.path.join(CASES, name + ".srt"),
                    "-o", os.path.join(tmp, name + ".out.srt")] + BAZARR + extra
            result = ffsubsync.run(ffsubsync.make_parser().parse_args(argv))
            golden.append({
                "name": name,
                "args": BAZARR + extra,
                "success": bool(result.get("sync_was_successful")),
                "offset": result["offset_seconds"],
                "scale": result["framerate_scale_factor"],
                "score": scores[0] if scores else None,
            })
    with open(os.path.join(HERE, "cases.json"), "w") as f:
        json.dump(golden, f, indent=1)
        f.write("\n")


if __name__ == "__main__":
    main()
