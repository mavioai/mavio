// The microphone's AudioWorklet: averages the context's samples down to
// 16 kHz and posts 20 ms of 16-bit PCM at a time.
class Capture extends AudioWorkletProcessor {
  constructor() {
    super();
    this.ratio = sampleRate / 16000;
    this.chunk = new Int16Array(320);
    this.filled = 0;
    this.sum = 0;
    this.count = 0;
    this.position = 0;
  }

  process(inputs) {
    const input = inputs[0][0];
    if (!input) return true;
    for (let i = 0; i < input.length; i++) {
      this.sum += input[i];
      this.count++;
      this.position++;
      if (this.position >= this.ratio) {
        this.position -= this.ratio;
        const s = Math.max(-1, Math.min(1, this.sum / this.count));
        this.chunk[this.filled++] = s < 0 ? s * 0x8000 : s * 0x7fff;
        this.sum = 0;
        this.count = 0;
        if (this.filled === this.chunk.length) {
          this.port.postMessage(this.chunk.buffer.slice(0));
          this.filled = 0;
        }
      }
    }
    return true;
  }
}

registerProcessor("capture", Capture);
