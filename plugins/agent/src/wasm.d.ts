// Files Bun embeds in the executable, imported with { type: "file" } as
// their path.
declare module "*.wasm" {
	const path: string;
	export default path;
}
