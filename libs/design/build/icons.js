// Copies the Phosphor icons icons/icons.json names into icons/svg and reads
// their path data, which the emitters write for each platform.
import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

export const WEIGHTS = ['regular', 'fill'];
export const VIEWBOX = 256;

// The package exports no file paths; pnpm links it into node_modules.
const PHOSPHOR = 'node_modules/@phosphor-icons/core/assets';

function paths(svg, file) {
  if (!svg.includes(`viewBox="0 0 ${VIEWBOX} ${VIEWBOX}"`)) throw new Error(`${file}: viewBox is not 0 0 ${VIEWBOX} ${VIEWBOX}`);
  const body = svg.replace(/^<svg[^>]*>|<\/svg>\s*$/g, '');
  const ds = [...body.matchAll(/<path d="([^"]+)"\/>/g)].map((m) => m[1]);
  if (ds.length === 0 || body.replace(/<path d="[^"]+"\/>/g, '') !== '') {
    throw new Error(`${file}: only <path d="…"/> elements are supported`);
  }
  return ds;
}

// readIcons returns { name: { regular: [d…], fill: [d…] } }.
export function readIcons() {
  const manifest = JSON.parse(readFileSync('icons/icons.json', 'utf8'));
  delete manifest.$comment;
  rmSync('icons/svg', { recursive: true, force: true });
  mkdirSync('icons/svg');
  const icons = {};
  for (const [name, phosphor] of Object.entries(manifest)) {
    if (!/^[a-z][a-zA-Z]*$/.test(name)) throw new Error(`icon ${name}: names are camelCase`);
    icons[name] = {};
    for (const weight of WEIGHTS) {
      const file = weight === 'regular' ? `${phosphor}.svg` : `${phosphor}-${weight}.svg`;
      const svg = readFileSync(join(PHOSPHOR, weight, file), 'utf8').trim();
      writeFileSync(`icons/svg/${name}${weight === 'regular' ? '' : '-' + weight}.svg`, svg + '\n');
      icons[name][weight] = paths(svg, file);
    }
  }
  return icons;
}
