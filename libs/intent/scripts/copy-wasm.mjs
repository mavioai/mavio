// Copies the WASM module built by `cargo build --target wasm32-unknown-unknown`
// to where it is used: dist/intent.wasm, which the web UI imports through
// @mavio/intent, and the directory the server embeds it from.
import { copyFileSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const built = join(root, 'target/wasm32-unknown-unknown/release/mavio_intent.wasm');
for (const to of [join(root, 'dist/intent.wasm'), join(root, '../../apps/server/internal/assistant/intent/dist/intent.wasm')]) {
  mkdirSync(dirname(to), { recursive: true });
  copyFileSync(built, to);
}
