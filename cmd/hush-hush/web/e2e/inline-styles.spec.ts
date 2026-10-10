import { expect, test } from '@playwright/test';

// alrayyes/hush-hush#762: the stylesheet is inlined into the page, so the
// browser has no CSS request to wait for before it paints. Lighthouse's
// render-blocking and network-dependency-tree insights flagged the linked
// file on every page. bun run lighthouse reads the same thing in the real
// reports; this keeps it from coming back unnoticed between runs.
test('the page ships its styles inline, not as a linked stylesheet', async ({
	request,
}) => {
	const res = await request.get('/');
	const html = await res.text();

	// SvelteKit keeps a disabled copy of the link for client navigation; a
	// disabled stylesheet blocks nothing, so only an enabled one counts.
	expect(html).not.toMatch(
		/<link(?![^>]*\bdisabled\b)[^>]*rel=["']stylesheet["']/,
	);
	expect(html).toContain('<style>');
});
