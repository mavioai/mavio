// Gathers the references of every client's snapshot tests into
// dist/snapshots/gallery: an index.html that draws each screen at each size
// side by side across the clients (docs/clients.md §6.8), and the images
// it draws. Directories given as arguments hold the latest run's images of
// a client instead, as <client>=<directory>.
import { copyFileSync, existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { basename, join } from "node:path";

const root = join(import.meta.dirname, "../../..");
const screens = JSON.parse(readFileSync(join(import.meta.dirname, "screens.json"), "utf8"));

/** Where each client keeps its references. */
const clients = {
  web: "apps/web/snapshots/references",
  ios: "libs/apple/snapshots",
  android: "libs/android/ui/snapshots",
  desktop: "apps/desktop/snapshots",
};
for (const arg of process.argv.slice(2)) {
  const [client, dir] = arg.split("=");
  if (!(client in clients) || !dir) throw new Error(`want <client>=<directory>, got ${arg}`);
  clients[client] = dir;
}

const out = join(root, "dist/snapshots/gallery");
mkdirSync(out, { recursive: true });

// The images of every client, by name.
const images = {};
for (const [client, dir] of Object.entries(clients)) {
  const path = join(root, dir);
  if (!existsSync(path)) continue;
  mkdirSync(join(out, client), { recursive: true });
  for (const file of readdirSync(path).filter((f) => f.endsWith(".png"))) {
    copyFileSync(join(path, file), join(out, client, file));
    (images[basename(file, ".png")] ??= {})[client] = `${client}/${file}`;
  }
}

const escape = (s) => s.replace(/[&<>"]/g, (c) => `&#${c.charCodeAt(0)};`);
const names = [
  ...Object.keys(screens.screens).flatMap((s) => Object.keys(screens.sizes).map((z) => `${s}-${z}`)),
  ...Object.keys(images).filter((n) => !Object.keys(screens.screens).some((s) => n.startsWith(`${s}-`))),
].filter((n) => images[n]);
const rows = names.map((name) => {
  const cells = Object.keys(clients).map((c) =>
    images[name][c]
      ? `<figure><a href="${images[name][c]}"><img src="${images[name][c]}" loading="lazy" alt=""></a><figcaption>${c}</figcaption></figure>`
      : `<figure class="none"><figcaption>${c}: none</figcaption></figure>`,
  );
  return `<section><h2>${escape(name)}</h2><div class="row">${cells.join("")}</div></section>`;
});

writeFileSync(
  join(out, "index.html"),
  `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Mavio snapshots</title>
<style>
:root { color-scheme: dark; font-family: system-ui, sans-serif; background: #0e0f12; color: #e6e7ea; }
body { margin: 0 16px 48px; }
h2 { font-size: 16px; margin: 32px 0 12px; }
.row { display: flex; gap: 16px; align-items: flex-start; overflow-x: auto; }
figure { margin: 0; flex: none; }
img { display: block; max-height: 60vh; max-width: 80vw; border: 1px solid #2a2c33; }
figcaption { font-size: 12px; color: #9a9ca3; margin-top: 4px; }
.none { width: 120px; }
</style></head><body>
<h1>Snapshots</h1>
${rows.join("\n")}
</body></html>
`,
);
console.log(`${join(out, "index.html")}: ${names.length} snapshots`);
