import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

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

	await nav.getByRole('button', { name: 'Log out' }).click();
	await page.waitForURL('/login');

	await page.goto('/changelog');
	await expect(page.locator('nav')).toHaveCount(0);
});
