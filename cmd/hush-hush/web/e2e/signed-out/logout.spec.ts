import { expect, test } from '@playwright/test';

// Last of the specs that share the journey's session, since signing out ends
// it for everything before.
test('Log out leaves for the login page, and a public page then has no nav', async ({
	page,
}) => {
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/');

	// Log out lives in .topbar-actions, not <nav> - it's an account
	// action, not a navigation link (#294).
	await page.getByRole('button', { name: 'Log out' }).click();
	await page.waitForURL('/login');

	await page.goto('/changelog');
	await expect(page.locator('nav')).toHaveCount(0);
});
