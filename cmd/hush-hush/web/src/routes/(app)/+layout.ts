import { redirect } from '@sveltejs/kit';
import { checkSession } from '$lib/api';
import type { LayoutLoad } from './$types';

// web-ui/spec.md's "Unauthenticated access is blocked" requirement: every
// page under this route group needs a valid session, and a visitor
// without one is redirected to /login rather than shown anything. /login
// itself lives outside this group (a sibling of it, not a child), so it
// never runs this check and can't redirect-loop against itself.
//
// Throwing redirect() here - rather than in each page's own load, or in
// the page component after it mounts - is what stops a child page's load
// (and its own fetch calls) from ever running at all: SvelteKit aborts
// the whole navigation the moment an ancestor load throws one. That's
// the mechanism "no secret or token data is fetched" actually relies on.
export const load: LayoutLoad = async () => {
	const authenticated = await checkSession();

	if (!authenticated) {
		redirect(303, '/login');
	}
};
