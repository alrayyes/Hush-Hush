import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

// web-ui-design-system/tasks.md #1.2: every page's base styles target a
// phone-width viewport, with wider layouts applied through min-width media
// queries - no page requires horizontal scrolling at 320px. Scoped to the
// pages reachable with no session; the authenticated (app) pages get the
// same coverage from the login-and-navigate journey test
// (web-ui-design-system/tasks.md #2.2), once that exists.
const PUBLIC_PAGES = ['/login', '/changelog', '/disclaimer', '/privacy'];

for (const path of PUBLIC_PAGES) {
	test(`${path} has no horizontal scroll at 320px`, async ({ page }) => {
		await page.setViewportSize({ width: 320, height: 720 });
		await page.goto(path);
		await expect(page.locator('main')).toBeVisible();

		const { scrollWidth, clientWidth } = await page.evaluate(() => ({
			scrollWidth: document.documentElement.scrollWidth,
			clientWidth: document.documentElement.clientWidth,
		}));

		expect(scrollWidth).toBeLessThanOrEqual(clientWidth);
	});

	test(`${path} renders at 1280px`, async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto(path);

		await expect(page.locator('main')).toBeVisible();
	});

	// rules/a11y.md: every page a journey test covers gets its own scan.
	// /changelog, /disclaimer, and /privacy have no session-gated flow of
	// their own to fold this into, unlike the authenticated pages the big
	// journey test already scans - a plain navigate-and-scan here is the
	// whole test.
	test(`${path} has no a11y violations`, async ({ page }) => {
		await page.goto(path);
		await expect(page.locator('main')).toBeVisible();

		const results = await new AxeBuilder({ page })
			.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
			.analyze();
		expect(results.violations).toEqual([]);
	});
}
