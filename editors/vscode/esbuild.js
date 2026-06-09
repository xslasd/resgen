const esbuild = require("esbuild");

const watch = process.argv.includes("--watch");

async function main() {
	await esbuild.build({
		entryPoints: ["src/extension.ts"],
		bundle: true,
		format: "cjs",
		minify: !watch,
		sourcemap: watch,
		sourcesContent: false,
		platform: "node",
		outfile: "dist/extension.js",
		external: ["vscode"],
		logLevel: "info",
		watch: watch,
	});
}

main().catch((e) => {
	console.error(e);
	process.exit(1);
});
