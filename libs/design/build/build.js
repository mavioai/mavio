// Generates gen/{ts,swift,kotlin,go} from tokens/, icons/ and strings/;
// run by pnpm nx run design:generate from libs/design.
import { execFileSync } from 'node:child_process';
import { rmSync } from 'node:fs';
import { readTokens } from './tokens.js';
import { readIcons } from './icons.js';
import { readStrings } from './strings.js';
import { emitTs } from './emit-ts.js';
import { emitSwift } from './emit-swift.js';
import { emitKotlin } from './emit-kotlin.js';
import { emitGo } from './emit-go.js';

const tokens = await readTokens();
const icons = readIcons();
const strings = readStrings();

rmSync('gen', { recursive: true, force: true });
emitTs(tokens, icons, strings);
emitSwift(tokens, icons, strings);
emitKotlin(tokens, icons, strings);
emitGo(tokens, icons, strings);
execFileSync('gofmt', ['-w', 'gen/go']);
