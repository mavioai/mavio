// Builds the plugin into a plugin folder the server can start, dist/plugin
// unless another folder is given, for this platform or the os/arch given.
// Usage: bun scripts/build.ts [os/arch] [folder]

import { join, resolve } from "node:path";
import { compile, root, targets } from "./compile.ts";

const platform = process.argv[2] || undefined;
const target = platform && targets[platform];
if (platform && !target) throw new Error(`no Bun target for ${platform}`);
const dir = process.argv[3] ? resolve(process.argv[3]) : join(root, "dist/plugin");
console.log(compile(dir, platform?.split("/")[0], target || undefined));
