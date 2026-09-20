import { checkSession, getHealth } from '$lib/api';
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
//
// authenticated is what the shared chrome uses to decide whether to
// show the authenticated nav (web-ui-design-system/tasks.md #2.1) - the
// same checkSession() call (app)/+layout.ts already makes for its own
// redirect guard, now also made at the root so a page outside that
// group (changelog, disclaimer, privacy) still gets the nav when a
// session exists.
//
// depends('app:auth') is what lets login/logout force this to
// actually re-run: checkSession() reaches the API through api.ts's own
// plain fetch, not this load's tracked one, so without an explicit
// dependency SvelteKit has no signal to invalidate the first result on
// a later navigation - the nav would otherwise stay stuck showing
// whatever session state was true when the root layout first loaded.
export const load: LayoutLoad = async ({ depends }) => {
	depends('app:auth');

	const [health, authenticated] = await Promise.all([
		getHealth(),
		checkSession(),
	]);

	return { version: health.version, authenticated };
};
