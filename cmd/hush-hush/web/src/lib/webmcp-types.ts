// Minimal ambient declaration for the WebMCP API this project calls -
// document.modelContext isn't in TypeScript's lib.dom.d.ts yet, since the
// spec is still a Draft Community Group Report
// (docs/adr/0020-webmcp-tools.md). Covers only the registerTool shape
// webmcp.ts actually uses, not the full spec surface (annotations,
// client.requestUserInteraction, the declarative form-attribute API,
// etc.) - add to this only when a real caller needs more of it.
//
// A plain .ts file named webmcp-types.ts, not webmcp.d.ts: TypeScript
// treats a .d.ts file as the declaration file *for* a same-named .ts/.js
// module in the same directory and silently excludes it from the
// program otherwise - a real webmcp.d.ts sitting next to webmcp.ts never
// entered the compilation at all (confirmed via `tsc --listFiles`), so
// its declare global block had no effect. A distinct basename avoids the
// collision.
declare global {
	interface ModelContextTool<In, Out> {
		name: string;
		description: string;
		inputSchema?: Record<string, unknown>;
		execute: (input: In) => Promise<Out> | Out;
	}

	interface ModelContext {
		registerTool<In, Out>(tool: ModelContextTool<In, Out>): Promise<undefined>;
	}

	interface Document {
		// Optional: absent in every browser without the origin-trial flag,
		// which is every browser today short of a manual one
		// (docs/adr/0020-webmcp-tools.md's "Build it now, feature-detected"
		// decision).
		modelContext?: ModelContext;
	}
}

export {};
