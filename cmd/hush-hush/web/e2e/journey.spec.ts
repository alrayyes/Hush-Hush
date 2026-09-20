import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

// alrayyes/hush-hush#272: /audit-log is both a SvelteKit page route and a
// real backend API endpoint, and Go's mux used to route a hard
// navigation there straight to the JSON handler instead of the SPA's
// static fallback. A real page.goto (not a client-side nav click, which
// never hits the server for this exact path either way) is the only way
// to exercise that server-side routing decision at all.
test('a hard navigation to /audit-log renders the app, not the raw API JSON', async ({
	page,
}) => {
	const response = await page.goto('/audit-log');

	expect(response?.headers()['content-type']).toContain('text/html');

	// No session yet, so the SPA's own client-side auth check redirects
	// to /login - unaffected by this fix, and proof the SPA actually
	// booted rather than the browser just displaying a JSON array as text
	// (which would never run any client JS to redirect anywhere).
	await page.waitForURL('/login');
	await expect(page).toHaveTitle('Log in - hush-hush');
	await expect(page.locator('nav')).toHaveCount(0);
});

// web-ui-design-system/tasks.md #2.1/#2.2: the shared chrome's nav follows
// a session onto every page, not just the ones under (app)/, and never
// shows for an unauthenticated visitor - plus the a11y.md-mandated
// axe-core scan, folded into this same login-and-navigate journey rather
// than a parallel suite re-driving the same pages.
test('an authenticated visitor keeps the nav across pages, an anonymous one never sees it, and the app has no a11y violations', async ({
	page,
	context,
	browserName,
}) => {
	test.skip(
		browserName !== 'chromium',
		'CDP virtual authenticator is Chromium-only',
	);

	const client = await context.newCDPSession(page);
	await client.send('WebAuthn.enable');
	await client.send('WebAuthn.addVirtualAuthenticator', {
		options: {
			protocol: 'ctap2',
			transport: 'internal',
			hasResidentKey: true,
			hasUserVerification: true,
			isUserVerified: true,
		},
	});

	await page.goto('/login');
	await page.getByRole('button', { name: 'Register passkey' }).click();
	await page.waitForURL('/');

	const nav = page.locator('nav');
	await expect(nav.getByRole('link', { name: 'Secrets' })).toBeVisible();

	await page.goto('/changelog');
	await expect(nav.getByRole('link', { name: 'Secrets' })).toBeVisible();

	// Scanned back on the secrets overview - the one authenticated page
	// with real interactive structure (a table, dialogs), not the mostly
	// static changelog.
	await page.goto('/');
	await expect(page.locator('main')).toBeVisible();
	const results = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(results.violations).toEqual([]);

	// A real secret with real width pressure - #271's own gap: the
	// public-pages-only viewport test never caught the authenticated
	// pages' tables scrolling sideways in their own box at 320px. The
	// same dialog also exercises the consumer combobox's listbox, open
	// and populated - the hand-written ARIA APG combobox
	// (alrayyes/hush-hush#251) is the newest, most complex interactive
	// widget added since the last scan.
	await page.getByRole('button', { name: 'New secret' }).click();
	// bits-ui's Dialog autofocuses the first field (Id) on open, racing any
	// interaction with a later field started right away - wait for that
	// autofocus to settle before touching a later field, or its own focus
	// gets stolen back mid-fill.
	await expect(page.getByLabel('Id')).toBeFocused();
	await page.locator('#create-id').fill('mattermost_deploy_webhook');
	await page.locator('#create-value').fill(btoa('placeholder'));
	await page
		.locator('#create-description')
		.fill('prod deploy webhook for homelab/vps-docker');
	await page.locator('#create-used-by').fill('homelab');
	await page.getByRole('listbox').waitFor();
	const comboboxResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(comboboxResults.violations).toEqual([]);
	await page.getByRole('option', { name: 'Add "homelab"' }).click();
	await page.getByRole('button', { name: 'Create' }).click();
	await page.getByRole('button', { name: 'New secret' }).waitFor();

	await page.setViewportSize({ width: 320, height: 720 });
	// Client-side nav clicks, not page.goto() - a hard navigation to
	// /audit-log is its own dedicated test below (#272), and this loop is
	// about the mobile layout, not routing.
	for (const linkName of ['Secrets', 'Audit log', 'Settings']) {
		await nav.getByRole('link', { name: linkName }).click();
		await expect(page.locator('main')).toBeVisible();

		// Checks every scrollable element on the page, not just the
		// document - a table with overflow-x: auto never overflows the
		// document (it scrolls sideways within its own box instead),
		// which is exactly what let this regression ship unnoticed the
		// first time. Scoped to overflow-x: scroll/auto specifically -
		// overflow: hidden (the visually-hidden <thead> pattern) also
		// reports a scrollWidth/clientWidth mismatch by design, with no
		// visible scrollbar to go with it.
		const widest = await page.evaluate(() =>
			Math.max(
				0,
				...Array.from(document.querySelectorAll('*'))
					.filter((el) =>
						['scroll', 'auto'].includes(getComputedStyle(el).overflowX),
					)
					.map((el) => el.scrollWidth - el.clientWidth),
			),
		);
		expect(
			widest,
			`${linkName} has a horizontally scrollable element at 320px`,
		).toBe(0);
	}
	await page.setViewportSize({ width: 1280, height: 800 });

	await nav.getByRole('button', { name: 'Log out' }).click();
	await page.waitForURL('/login');

	await page.goto('/changelog');
	await expect(page.locator('nav')).toHaveCount(0);
});
