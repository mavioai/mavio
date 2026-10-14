// Packages the plugin for every platform the server is built for into
// dist/, as the catalog offers it: a zip per platform holding manifest.json
// and the executable, and dist/catalog.json listing them.
// Usage: bun scripts/dist.ts [os/arch ...]

import { createHash } from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import manifest from "../manifest.json" with { type: "json" };
import { compile, root, targets } from "./compile.ts";

const wanted = process.argv.slice(2);
const now = new Date().toISOString().replace(/\.\d+Z$/, "Z");
const versions = [];
for (const platform of wanted.length > 0 ? wanted : Object.keys(targets)) {
	const target = targets[platform];
	if (!target) throw new Error(`no Bun target for ${platform}`);
	const [os, arch] = platform.split("/") as [string, string];
	const dir = join(root, "dist", `${os}-${arch}`);
	const exe = compile(dir, os, target);
	const zip = `agent-${manifest.version}-${os}-${arch}.zip`;
	const run = Bun.spawnSync(["zip", "-q", "-X", join("..", zip), "manifest.json", exe.slice(dir.length + 1)], { cwd: dir, stderr: "inherit" });
	if (run.exitCode !== 0) throw new Error(`zip exited with ${run.exitCode}`);
	const sha256 = createHash("sha256").update(readFileSync(join(root, "dist", zip))).digest("hex");
	versions.push({ version: manifest.version, api_version: manifest.apiVersion, runtime: "RUNTIME_PROCESS", os, arch, url: zip, sha256, release_time: now });
	console.log(`dist/${zip}`);
}
const catalog = {
	plugins: [
		{
			id: manifest.id,
			name: manifest.name,
			description: "An AI gateway to the providers pi-ai supports, with their API keys and OAuth logins, and agent sessions.",
			author: manifest.author,
			homepage: manifest.homepage,
			versions,
		},
	],
};
writeFileSync(join(root, "dist/catalog.json"), JSON.stringify(catalog, null, 2) + "\n");
