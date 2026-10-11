import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

const WCAG = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'];

// alrayyes/hush-hush#790. The status code is the server's half (#789);
// the page is this app's own.
test('a path no route matches is a 404 with a plain page in the app shell', async ({
	page,
}) => {
	const response = await page.goto('/this/path/cannot/exist');

	expect(response?.status()).toBe(404);
	await expect(page).toHaveTitle(/Page not found/);
	await expect(
		page.getByRole('heading', { level: 1, name: 'Page not found' }),
	).toBeVisible();
	await expect(page.getByText('404', { exact: true })).toBeVisible();
	// Signed out: the login-page shell, no signed-in navigation.
	await expect(page.getByRole('navigation', { name: 'Primary' })).toHaveCount(
		0,
	);
	await expect(page.getByRole('contentinfo')).toBeVisible();
	await expect(
		page.getByRole('link', { name: /sign in/i }).first(),
	).toHaveAttribute('href', '/');

	const results = await new AxeBuilder({ page }).withTags(WCAG).analyze();
	expect(results.violations).toEqual([]);

	const html = await page.content();
	expect(html).not.toMatch(/\bat \S+\.(js|ts|svelte)/);
	expect(html).not.toMatch(/\/home\/|node_modules/);
});

test('the 404 page has no horizontal scroll at 320px', async ({ page }) => {
	await page.setViewportSize({ width: 320, height: 720 });
	await page.goto('/nope');
	await expect(
		page.getByRole('heading', { level: 1, name: 'Page not found' }),
	).toBeVisible();

	const { scrollWidth, clientWidth } = await page.evaluate(() => ({
		scrollWidth: document.documentElement.scrollWidth,
		clientWidth: document.documentElement.clientWidth,
	}));
	expect(scrollWidth).toBeLessThanOrEqual(clientWidth);
});
