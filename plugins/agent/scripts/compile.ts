// Compiles the plugin with Bun into one executable in a plugin folder next
// to its manifest.

import { copyFileSync, mkdirSync, rmSync } from "node:fs";
import { join } from "node:path";

/** The Bun targets of the platforms the server is built for, by os/arch. */
export const targets: Record<string, string> = {
	"linux/amd64": "bun-linux-x64",
	"linux/arm64": "bun-linux-arm64",
	"darwin/amd64": "bun-darwin-x64",
	"darwin/arm64": "bun-darwin-arm64",
	"windows/amd64": "bun-windows-x64",
	"windows/arm64": "bun-windows-arm64",
};

/** The project's folder. */
export const root = join(import.meta.dir, "..");

/**
 * compile builds the plugin folder dir for os, with the Bun target given
 * or this platform's, and returns the executable's path.
 */
export function compile(dir: string, os: string = process.platform, target?: string): string {
	const exe = join(dir, os === "windows" || os === "win32" ? "plugin.exe" : "plugin");
	rmSync(dir, { recursive: true, force: true });
	mkdirSync(dir, { recursive: true });
	const args = ["build", "--compile", "--minify", ...(target ? [`--target=${target}`] : []), join(root, "src/main.ts"), "--outfile", exe];
	const run = Bun.spawnSync([process.execPath, ...args], { cwd: root, stdout: "inherit", stderr: "inherit" });
	if (run.exitCode !== 0) throw new Error(`bun build exited with ${run.exitCode}`);
	copyFileSync(join(root, "manifest.json"), join(dir, "manifest.json"));
	return exe;
}
