// "own backend, static frontend, no SSR" - design.md's "Frontend:
// SvelteKit + @sveltejs/adapter-static" decision. Every page's data
// comes from a fetch() call against the Go API at runtime; there's
// nothing here for a server render to do.
export const ssr = false;
