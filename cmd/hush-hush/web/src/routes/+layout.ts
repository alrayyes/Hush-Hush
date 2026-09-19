import { getHealth } from '$lib/api';
import type { LayoutLoad } from './$types';

// "own backend, static frontend, no SSR" - design.md's "Frontend:
// SvelteKit + @sveltejs/adapter-static" decision. Every page's data
// comes from a fetch() call against the Go API at runtime; there's
// nothing here for a server render to do.
export const ssr = false;

// The footer's own version - fetched here, at the true root, rather
// than in the (app) group's layout, since the footer is on every page
// including login (web-ui/spec.md's "Footer content is present on
// every page" requirement) and /healthz needs no session.
export const load: LayoutLoad = async () => {
	const health = await getHealth();

	return { version: health.version };
};
