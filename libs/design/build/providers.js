// Reads providers/providers.json, the AI providers' logos by provider ID,
// and their SVGs in providers/svg, cleaned of what only a web page of
// lobehub/icons needs.
import { readFileSync } from 'node:fs';

// clean keeps an SVG's viewBox and drawing, without its title, size or style.
function clean(svg, file) {
  const out = svg
    .trim()
    .replace(/<title>[^<]*<\/title>/g, '')
    .replace(/^<svg([^>]*)>/, (_, attrs) => `<svg${attrs.replace(/\s(height|width|style)="[^"]*"/g, '')}>`);
  if (!/^<svg[^>]* viewBox="[^"]+"/.test(out)) throw new Error(`${file}: no viewBox`);
  if (/<(script|image|use|foreignObject)\b/.test(out)) throw new Error(`${file}: only shapes and gradients are supported`);
  return out;
}

// readProviders returns { logos: { name: { svg, monochrome } }, providers: { id: name } }.
export function readProviders() {
  const manifest = JSON.parse(readFileSync('providers/providers.json', 'utf8'));
  delete manifest.$comment;
  const logos = {};
  for (const name of new Set(Object.values(manifest))) {
    if (!/^[a-z0-9-]+$/.test(name)) throw new Error(`provider logo ${name}: names are lowercase with dashes`);
    const file = `providers/svg/${name}.svg`;
    // Logos without -color draw in currentColor, the text's.
    logos[name] = { svg: clean(readFileSync(file, 'utf8'), file), monochrome: !name.endsWith('-color') };
  }
  return { logos, providers: Object.fromEntries(Object.entries(manifest).sort(([a], [b]) => a.localeCompare(b))) };
}
