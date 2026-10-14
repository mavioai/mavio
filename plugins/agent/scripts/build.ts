// Builds the plugin into dist/plugin, a plugin folder the server can start,
// for this platform or the os/arch given.
// Usage: bun scripts/build.ts [os/arch]

import { join } from "node:path";
import { compile, root, targets } from "./compile.ts";

const platform = process.argv[2];
const target = platform && targets[platform];
if (platform && !target) throw new Error(`no Bun target for ${platform}`);
console.log(compile(join(root, "dist/plugin"), platform?.split("/")[0], target || undefined));
