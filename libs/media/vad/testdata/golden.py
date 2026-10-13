"""Records the speech probabilities onnxruntime gives with Silero VAD's
16 kHz model, which the Go inference is checked against.

    python golden.py silero_vad_16k_op15.onnx test.wav > probabilities.json

silero_vad_16k_op15.onnx and test.wav (tests/data/test.wav) come from
github.com/snakers4/silero-vad (MIT) at commit
1e261b036686cd0017d500ee96acd1c4ba572a9d. Windows are run as Silero's
OnnxWrapper runs them: 512 samples preceded by the 64 before them, the state
kept across windows. "noise" is the input noise() of vad_test.go generates.
"""

import json
import sys

import numpy as np
import onnxruntime as ort


def noise(n):
    s, out = 1, []
    for i in range(n):
        s = (s * 1664525 + 1013904223) & 0xFFFFFFFF
        v = (s >> 16) - 32768
        shift = 1 + (i // 8000) % 4
        out.append(v >> shift)
    return np.array(out, dtype=np.float32) / 32768


def run(session, x):
    state = np.zeros((2, 1, 128), np.float32)
    context = np.zeros((1, 64), np.float32)
    out = []
    pad = (-len(x)) % 512
    x = np.concatenate([x, np.zeros(pad, np.float32)])
    for i in range(0, len(x), 512):
        inp = np.concatenate([context, x[None, i:i + 512]], 1)
        o, state = session.run(None, {"input": inp, "state": state, "sr": np.array(16000, dtype=np.int64)})
        out.append(float(o[0, 0]))
        context = inp[:, -64:]
    return out


session = ort.InferenceSession(sys.argv[1], providers=["CPUExecutionProvider"])
wav = np.fromfile(sys.argv[2], dtype="<i2")[22:].astype(np.float32) / 32768
json.dump({
    "noise": [round(p, 7) for p in run(session, noise(40000))],
    "speech": [round(p, 7) for p in run(session, wav)],
}, sys.stdout)
