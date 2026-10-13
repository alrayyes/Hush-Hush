import { expect, test } from '@playwright/test';

// alrayyes/Hush-Hush#885: <meta name="theme-color"> tints the browser's window
// chrome and an installed app's title bar. It has to follow the theme the page
// shows, including a choice made with the toggle against the OS scheme.
// themeColor's values are unit-tested in src/lib/theme.spec.ts; this checks
// that the page applies them, before first paint and on a click.

const LIGHT = '#ffffff';
const DARK = '#121212';

const themeColor = (page: import('@playwright/test').Page) =>
	page.locator('head meta[name="theme-color"]');

test('with no stored choice the color follows the OS scheme', async ({
	page,
}) => {
	await page.emulateMedia({ colorScheme: 'dark' });
	await page.goto('/login');

	await expect(themeColor(page)).toHaveAttribute('content', DARK);
});

test('a stored choice wins over the OS scheme', async ({ page }) => {
	await page.emulateMedia({ colorScheme: 'light' });
	await page.addInitScript(() => localStorage.setItem('theme', 'dark'));
	await page.goto('/login');

	await expect(themeColor(page)).toHaveAttribute('content', DARK);
});

test('the toggle changes the color without a reload', async ({ page }) => {
	await page.emulateMedia({ colorScheme: 'light' });
	await page.goto('/login');
	await expect(themeColor(page)).toHaveAttribute('content', LIGHT);

	await page.getByRole('button', { name: /switch to dark mode/i }).click();

	await expect(themeColor(page)).toHaveAttribute('content', DARK);
});
